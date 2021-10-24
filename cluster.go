package raft

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/shaj13/raftkit/internal/daemon"
	"github.com/shaj13/raftkit/internal/membership"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/storage"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

type Cluster interface {
	Start(opts ...StartOption) error
	Leave(ctx context.Context) error
	StepDown(ctx context.Context) error
	UpdateMember(ctx context.Context, id uint64, addr string) error
	CreateSnapshot() (io.ReadCloser, error)
	TransferLeadership(ctx context.Context, id uint64) error
	RemoveMember(ctx context.Context, id uint64) error
	AddMember(ctx context.Context, addr string) (Member, error)
	GetMemebr(id uint64) (Member, bool)
	Members() []Member
	RemovedMembers() []Member
	AddressInUse(addr string) uint64
	LongestActive() (Member, error)
	IsAvailable() bool
	IsMemberRemoved(id uint64) bool
	IsMember(id uint64) bool
	Whoami() uint64
	Leader() uint64
}

type cluster struct {
	pool    membership.Pool
	storage storage.Storage
	daemon  daemon.Daemon
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

	m, err := c.LongestActive()
	if err != nil {
		return err
	}

	return c.daemon.TransferLeadership(ctx, m.ID())
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

func (c *cluster) UpdateMember(ctx context.Context, id uint64, addr string) error {
	err := c.precondition(
		joined(),
		available(),
		notMember(id),
		addressInUse(addr),
	)

	if err != nil {
		return err
	}

	m := &raftpb.Member{
		ID:   id,
		Type: raftpb.RemovedMember,
	}

	return c.daemon.ProposeConfChange(ctx, m, etcdraftpb.ConfChangeUpdateNode)
}

func (c *cluster) RemoveMember(ctx context.Context, id uint64) error {
	err := c.precondition(
		joined(),
		available(),
		notMember(id),
		memberRemoved(id),
		rmLeader(id),
	)

	if err != nil {
		return err
	}

	m := &raftpb.Member{
		ID:   id,
		Type: raftpb.RemovedMember,
	}

	return c.daemon.ProposeConfChange(ctx, m, etcdraftpb.ConfChangeRemoveNode)
}

func (c *cluster) AddMember(ctx context.Context, addr string) (Member, error) {
	err := c.precondition(
		joined(),
		available(),
		addressInUse(addr),
	)

	if err != nil {
		return nil, err
	}

	m := &raftpb.Member{
		ID:      c.pool.NextID(),
		Address: addr,
		Type:    raftpb.RemoteMember,
	}

	err = c.daemon.ProposeConfChange(ctx, m, etcdraftpb.ConfChangeAddNode)
	if err != nil {
		return nil, err
	}

	memb, _ := c.pool.Get(m.ID)
	return memb, nil
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

func (c *cluster) AddressInUse(addr string) uint64 {
	for _, m := range c.Members() {
		if m.Address() == addr {
			return m.ID()
		}
	}
	return 0
}

func (c *cluster) LongestActive() (Member, error) {
	var (
		longest     Member
		longestTime time.Time
	)

	for _, m := range c.Members() {
		since := m.ActiveSince()
		if since.IsZero() || m.Type() == raftpb.LocalMember {
			continue
		}

		if longest == nil {
			longest = m
			continue
		}
		if since.Before(longestTime) {
			longest = m
			longestTime = since
		}
	}

	if longest == nil {
		return nil, errors.New("raft: failed to find longest active member")
	}

	return longest, nil
}

func (c *cluster) IsAvailable() bool {
	cond := func(m Member) bool {
		return m.IsActive()
	}

	q := (len(c.Members()))/2 + 1
	n := len(c.members(cond))

	return n >= q
}

func (c *cluster) IsMemberRemoved(id uint64) bool {
	m, _ := c.GetMemebr(id)
	return m.Type() == raftpb.RemovedMember
}

func (c *cluster) IsMember(id uint64) bool {
	_, ok := c.pool.Get(id)
	return ok
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
		if !c.IsMember(id) {
			return fmt.Errorf("raft: unknown member %x", id)
		}
		return nil
	}
}

func memberRemoved(id uint64) func(c *cluster) error {
	return func(c *cluster) error {
		if c.IsMemberRemoved(id) {
			return fmt.Errorf("raft: member %x already removed", id)
		}
		return nil
	}
}

func addressInUse(addr string) func(c *cluster) error {
	return func(c *cluster) error {
		if id := c.AddressInUse(addr); id > 0 {
			return fmt.Errorf("raft: address used by member %x", id)
		}
		return nil
	}
}

func notLeader() func(c *cluster) error {
	return func(c *cluster) error {
		if c.Whoami() != c.Leader() {
			return daemon.ErrNotLeader
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
