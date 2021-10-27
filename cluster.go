package raft

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/shaj13/raftkit/internal/daemon"
	"github.com/shaj13/raftkit/internal/log"
	"github.com/shaj13/raftkit/internal/membership"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/storage"
	"github.com/shaj13/raftkit/internal/transport"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

// TODO: do we need to expose ?
var errNotLeader = errors.New("raft: operation not permitted, node is not the leader")

type Cluster interface {
	Start(opts ...StartOption) error
	Leave(ctx context.Context) error
	StepDown(ctx context.Context) error
	UpdateMember(ctx context.Context, raw *RawMember) error
	CreateSnapshot() (io.ReadCloser, error)
	TransferLeadership(ctx context.Context, id uint64) error
	RemoveMember(ctx context.Context, id uint64) error
	AddMember(ctx context.Context, raw *RawMember) error
	PromoteMember(ctx context.Context, id uint64) error
	GetMemebr(id uint64) (Member, bool)
	Members() []Member
	// TODO: Remove this api
	RemovedMembers() []Member
	IsAvailable() bool
	Whoami() uint64
	Leader() uint64
}

type cluster struct {
	dial              transport.Dial
	pool              membership.Pool
	storage           storage.Storage
	daemon            daemon.Daemon
	disableForwarding bool
}

func (c *cluster) CreateSnapshot() (io.ReadCloser, error) {
	err := c.precondition(
		joined(),
	)

	if err != nil {
		return nil, err
	}

	snap, err := c.daemon.CreateSnapshot()
	if err != nil {
		return nil, err
	}

	_, r, err := c.storage.Snapshotter().Reader(snap)
	return r, err
}

func (c *cluster) TransferLeadership(ctx context.Context, id uint64) error {
	err := c.precondition(
		joined(),
		notMember(id),
		noLeader(),
		disableForwarding(),
		available(),
	)

	if err != nil {
		return err
	}

	return c.daemon.TransferLeadership(ctx, id)
}

func (c *cluster) StepDown(ctx context.Context) error {
	err := c.precondition(
		joined(),
		notLeader(),
		available(),
	)

	if err != nil {
		return err
	}

	// we can do the following, because member's active since is given from the current node clock.
	longest := time.Now().Add(math.MaxInt64)
	cond := func(m Member) bool {
		since := m.ActiveSince()
		ok := m.IsActive() && m.Type() == RemoteMember && since.Before(longest)
		if ok {
			longest = since
			return true
		}

		return false
	}

	// get longest active member then transfer leadership to it.
	// TODO: can we get it progress ?
	membs := c.members(cond)
	if len(membs) == 0 {
		return errors.New("raft: failed to find longest active member")
	}

	return c.daemon.TransferLeadership(ctx, membs[0].ID())
}

func (c *cluster) Start(opts ...StartOption) error {
	cfg := new(startConfig)
	cfg.apply(opts...)
	return c.daemon.Start(cfg.addr, cfg.operators...)
}

func (c *cluster) Leave(ctx context.Context) error {
	return c.RemoveMember(
		ctx,
		c.Whoami(),
	)
}

func (c *cluster) UpdateMember(ctx context.Context, raw *RawMember) error {
	err := c.precondition(
		joined(),
		notMember(raw.ID),
		addressInUse(raw.ID, raw.Address),
		noLeader(),
		disableForwarding(),
		available(),
	)

	if err != nil {
		return err
	}

	mem, _ := c.GetMemebr(raw.ID)
	raw.Type = mem.Type()

	return c.daemon.ProposeConfChange(ctx, raw, etcdraftpb.ConfChangeUpdateNode)
}

func (c *cluster) RemoveMember(ctx context.Context, id uint64) error {
	err := c.precondition(
		joined(),
		notMember(id),
		memberRemoved(id),
		rmLeader(id),
		noLeader(),
		disableForwarding(),
		available(),
	)

	if err != nil {
		return err
	}

	mem, _ := c.GetMemebr(id)
	raw := mem.Raw()
	raw.Type = raftpb.RemovedMember

	return c.daemon.ProposeConfChange(ctx, &raw, etcdraftpb.ConfChangeRemoveNode)
}

func (c *cluster) AddMember(ctx context.Context, raw *RawMember) error {
	err := c.precondition(
		joined(),
		addressInUse(raw.ID, raw.Address),
		idInUse(raw.ID),
		noLeader(),
		disableForwarding(),
		available(),
	)

	if err != nil {
		return err
	}

	if raw.ID == None {
		raw.ID = c.pool.NextID()
	}

	cct := etcdraftpb.ConfChangeAddNode
	if raw.Type == raftpb.LearnerMember || raw.Type == raftpb.LocalLearnerMember {
		cct = etcdraftpb.ConfChangeAddLearnerNode
	}

	return c.daemon.ProposeConfChange(ctx, raw, cct)
}

func (c *cluster) PromoteMember(ctx context.Context, id uint64) error {
	return c.promoteMember(ctx, id, false)
}

func (c *cluster) GetMemebr(id uint64) (Member, bool) {
	return c.pool.Get(id)
}

func (c *cluster) members(cond func(m Member) bool) []Member {
	mems := []Member{}
	for _, m := range c.pool.Members() {
		if cond(m) {
			mems = append(mems, m)
		}
	}
	return mems
}

func (c *cluster) Members() []Member {
	cond := func(m Member) bool {
		return m.Type() != raftpb.RemovedMember
	}
	return c.members(cond)
}

func (c *cluster) RemovedMembers() []Member {
	cond := func(m Member) bool {
		return m.Type() == raftpb.RemovedMember
	}
	return c.members(cond)
}

func (c *cluster) IsAvailable() bool {
	cond := func(m Member) bool {
		return m.IsActive()
	}

	q := (len(c.Members()))/2 + 1
	n := len(c.members(cond))

	return n >= q
}

func (c *cluster) Whoami() uint64 {
	s, _ := c.daemon.Status()
	return s.ID
}

func (c *cluster) Leader() uint64 {
	s, _ := c.daemon.Status()
	return s.Lead
}

func (c *cluster) precondition(fns ...func(c *cluster) error) error {
	for _, fn := range fns {
		if err := fn(c); err != nil {
			return err
		}
	}
	return nil
}

func (c *cluster) promoteMember(ctx context.Context, id uint64, forwarded bool) error {
	err := c.precondition(
		joined(),
		notMember(id),
		noLeader(),
		disableForwarding(),
		available(),
	)

	if err != nil {
		return err
	}

	// TODO: move to precond
	mem, _ := c.GetMemebr(id)
	if !(mem.Type() == LocalLearnerMember || mem.Type() == LearnerMember) {
		return fmt.Errorf("raft: memebr %x  is not a learner", id)
	}

	rs, err := c.daemon.Status()
	if err != nil {
		return err
	}

	// leader may lost during forwarding,
	// if there is no progress and promotion have been forwarded to this node.
	if rs.Progress == nil && forwarded {
		return daemon.ErrNoLeader
	}

	if rs.Progress == nil {
		lmem, ok := c.GetMemebr(rs.Lead)
		// leader lost, because rs.Lead = None.
		if !ok {
			return daemon.ErrNoLeader
		}

		client, err := c.dial(ctx, lmem.Address())
		if err != nil {
			return err
		}

		log.Debugf("raft.node: forwarding member %x promotion to %x", id, lmem.ID())
		return client.PromoteMember(ctx, mem.Raw())
	}

	leader := rs.Progress[rs.ID].Match
	learner := rs.Progress[id].Match
	// the learner's Match not caught up with the leader yet.
	if float64(learner) < float64(leader)*0.9 {
		return fmt.Errorf("raft: promotion failed, memebr %x not synced with the leader yet", id)
	}

	raw := mem.Raw()
	(&raw).Type = RemoteMember

	return c.daemon.ProposeConfChange(ctx, &raw, etcdraftpb.ConfChangeAddNode)
}

func joined() func(c *cluster) error {
	return func(c *cluster) error {
		if c.Whoami() == 0 {
			return fmt.Errorf("raft: node is not yet part of a raft cluster")
		}
		return nil
	}
}

func available() func(c *cluster) error {
	return func(c *cluster) error {
		if !c.IsAvailable() {
			return fmt.Errorf("raft: quorum lost and the cluster unavailable, no new logs can be committed")
		}
		return nil
	}
}

func notMember(id uint64) func(c *cluster) error {
	return func(c *cluster) error {
		if _, ok := c.GetMemebr(id); !ok {
			return fmt.Errorf("raft: unknown member %x", id)
		}
		return nil
	}
}

func memberRemoved(id uint64) func(c *cluster) error {
	return func(c *cluster) error {
		m, ok := c.GetMemebr(id)
		if ok && m.Type() == RemovedMember {
			return fmt.Errorf("raft: member %x already removed", id)
		}
		return nil
	}
}

func addressInUse(mid uint64, addr string) func(c *cluster) error {
	return func(c *cluster) error {
		membs := c.members(func(m Member) bool {
			return m.Address() == addr && m.ID() != mid
		})

		if len(membs) > 0 {
			return fmt.Errorf("raft: address used by member %x", membs[0].ID())
		}
		return nil
	}
}

func notLeader() func(c *cluster) error {
	return func(c *cluster) error {
		if c.Whoami() != c.Leader() {
			return errNotLeader
		}
		return nil
	}
}

func rmLeader(id uint64) func(c *cluster) error {
	return func(c *cluster) error {
		if id == c.Leader() {
			return fmt.Errorf("raft: member %x is the leader and cannot be removed, transfer leadership first", id)
		}
		return nil
	}
}

func idInUse(id uint64) func(c *cluster) error {
	return func(c *cluster) error {
		if _, ok := c.GetMemebr(id); ok {
			return fmt.Errorf("raft: id used by member %x", id)
		}
		return nil
	}
}

func noLeader() func(c *cluster) error {
	return func(c *cluster) error {
		if c.Leader() == None {
			return daemon.ErrNoLeader
		}
		return nil
	}
}

func disableForwarding() func(c *cluster) error {
	return func(c *cluster) error {
		if c.Leader() != c.Whoami() && c.disableForwarding {
			return errNotLeader
		}
		return nil
	}
}
