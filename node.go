package raft

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/shaj13/raft/internal/membership"
	"github.com/shaj13/raft/internal/raftengine"
	"github.com/shaj13/raft/internal/raftpb"
	"github.com/shaj13/raft/internal/storage"
	"github.com/shaj13/raft/internal/transport"
	etransport "github.com/shaj13/raft/transport"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

// TODO: do we need to expose ?
var (
	// ErrNodeStopped is returned by the Node methods after a call to
	// Shutdown or when it has not started.
	ErrNodeStopped = raftengine.ErrStopped
	// ErrNotLeader is returned when an operation can't be completed on a
	// follower or candidate node
	ErrNotLeader = errors.New("raft: node is not the leader")
)

func NewNodeGroup(proto etransport.Proto) *NodeGroup {
	cfg := newConfig()
	nh, _ := transport.Proto(proto).Get()
	mux := raftengine.NewMux()

	router := &router{
		ctrls: make(map[uint64]transport.Controller),
	}
	cfg.controller = router

	return &NodeGroup{
		mux:     mux,
		handler: nh(cfg),
		router:  router,
	}
}

type NodeGroup struct {
	mux     raftengine.Mux
	handler transport.Handler
	router  *router
}

func (ng *NodeGroup) Handler() etransport.Handler {
	return ng.handler
}

func (ng *NodeGroup) Start() {
	ng.mux.Start()
}

func (ng *NodeGroup) Add(id uint64, n *Node) bool {
	if _, err := n.engine.Status(); err == nil {
		return false
	}
	n.cfg.groupID = id
	n.cfg.mux = ng.mux
	n.handler = ng.handler
	ng.router.add(id, n.cfg.controller)
	return true
}

func (ng *NodeGroup) Remove(id uint64) {
	ng.router.remove(id)
}

func (ng *NodeGroup) Stop() {
	ng.mux.Stop()
}

type Node struct {
	handler transport.Handler
	dial    transport.Dial
	pool    membership.Pool
	storage storage.Storage
	engine  raftengine.Engine
	cfg     *config
	// exec pre conditions, its used by tests.
	exec func(fns ...func(c *Node) error) error
}

func (n *Node) Shutdown(ctx context.Context) error {
	return n.engine.Shutdown(ctx)
}

func (n *Node) Handler() etransport.Handler {
	return n.handler
}

func (n *Node) LinearizableRead(ctx context.Context) error {
	err := n.preCond(
		joined(),
		noLeader(),
		available(),
	)

	if err != nil {
		return err
	}

	return n.engine.LinearizableRead(ctx)
}

func (n *Node) Snapshot() (io.ReadCloser, error) {
	err := n.preCond(
		joined(),
	)

	if err != nil {
		return nil, err
	}

	snap, err := n.engine.CreateSnapshot()
	if err != nil {
		return nil, err
	}

	meta := snap.Metadata
	return n.storage.Snapshotter().Reader(meta.Term, meta.Index)
}

func (n *Node) TransferLeadership(ctx context.Context, id uint64) error {
	err := n.preCond(
		joined(),
		notMember(id),
		memberRemoved(id),
		noLeader(),
		notType(n.Whoami(), VoterMember),
		disableForwarding(), // TODO: verify this.
		available(),
	)

	if err != nil {
		return err
	}

	return n.engine.TransferLeadership(ctx, id)
}

func (n *Node) StepDown(ctx context.Context) error {
	err := n.preCond(
		joined(),
		notLeader(),
		available(),
	)

	if err != nil {
		return err
	}

	// we can do the following, because member's active since is given from the current node clock.
	longest := time.Now().Add(math.MaxInt64)
	id := n.Whoami()
	cond := func(m Member) bool {
		since := m.ActiveSince()
		ok := m.IsActive() && m.Type() == VoterMember && since.Before(longest) && id != m.ID()
		if ok {
			longest = since
			return true
		}

		return false
	}

	// get longest active member then transfer leadership to it.
	membs := n.members(cond)
	if len(membs) == 0 {
		return errors.New("raft: failed to find longest active member")
	}

	return n.engine.TransferLeadership(ctx, membs[0].ID())
}

func (n *Node) Start(opts ...StartOption) error {
	cfg := new(startConfig)
	cfg.apply(opts...)
	return n.engine.Start(cfg.addr, cfg.operators...)
}

func (n *Node) Leave(ctx context.Context) error {
	return n.RemoveMember(
		ctx,
		n.Whoami(),
	)
}

func (n *Node) Replicate(ctx context.Context, data []byte) error {
	err := n.preCond(
		joined(),
		noLeader(),
		notType(n.Whoami(), VoterMember),
		disableForwarding(),
		available(),
	)

	if err != nil {
		return err
	}

	return n.engine.ProposeReplicate(ctx, data)
}

func (n *Node) UpdateMember(ctx context.Context, raw *RawMember) error {
	err := n.preCond(
		joined(),
		notMember(raw.ID),
		memberRemoved(raw.ID),
		noLeader(),
		notType(n.Whoami(), VoterMember),
		addressInUse(raw.ID, raw.Address),
		disableForwarding(),
		available(),
	)

	if err != nil {
		return err
	}

	mem, _ := n.GetMemebr(raw.ID)
	raw.Type = mem.Type()

	return n.engine.ProposeConfChange(ctx, raw, etcdraftpb.ConfChangeUpdateNode)
}

func (n *Node) RemoveMember(ctx context.Context, id uint64) error {
	err := n.preCond(
		joined(),
		notMember(id),
		memberRemoved(id),
		leader(id),
		noLeader(),
		notType(n.Whoami(), VoterMember),
		disableForwarding(),
		available(),
	)

	if err != nil {
		return err
	}

	mem, _ := n.GetMemebr(id)
	raw := mem.Raw()
	raw.Type = raftpb.RemovedMember

	return n.engine.ProposeConfChange(ctx, &raw, etcdraftpb.ConfChangeRemoveNode)
}

func (n *Node) AddMember(ctx context.Context, raw *RawMember) error {
	err := n.preCond(
		joined(),
		idInUse(raw.ID),
		addressInUse(raw.ID, raw.Address),
		noLeader(),
		notType(n.Whoami(), VoterMember),
		disableForwarding(),
		available(),
	)

	if err != nil {
		return err
	}

	if raw.ID == None {
		raw.ID = n.pool.NextID()
	}

	cct := etcdraftpb.ConfChangeAddNode
	if raw.Type != VoterMember {
		cct = etcdraftpb.ConfChangeAddLearnerNode
	}

	return n.engine.ProposeConfChange(ctx, raw, cct)
}

func (n *Node) PromoteMember(ctx context.Context, id uint64) error {
	return n.promoteMember(ctx, id, false)
}

func (n *Node) DemoteMember(ctx context.Context, id uint64) error {
	err := n.preCond(
		joined(),
		notMember(id),
		memberRemoved(id),
		noLeader(),
		leader(id),
		notType(n.Whoami(), VoterMember),
		notType(id, VoterMember),
		disableForwarding(),
		available(),
	)

	if err != nil {
		return err
	}

	mem, _ := n.GetMemebr(id)
	raw := mem.Raw()
	(&raw).Type = LearnerMember

	return n.engine.ProposeConfChange(ctx, &raw, etcdraftpb.ConfChangeAddLearnerNode)
}

func (n *Node) GetMemebr(id uint64) (Member, bool) {
	return n.pool.Get(id)
}

func (n *Node) members(cond func(m Member) bool) []Member {
	mems := []Member{}
	for _, m := range n.pool.Members() {
		if cond(m) {
			mems = append(mems, m)
		}
	}
	return mems
}

func (n *Node) Members() []Member {
	return n.members(func(m Member) bool { return true })
}

func (n *Node) Whoami() uint64 {
	s, _ := n.engine.Status()
	return s.ID
}

func (n *Node) Leader() uint64 {
	s, _ := n.engine.Status()
	return s.Lead
}

func (n *Node) preCond(fns ...func(c *Node) error) error {
	if n.exec != nil {
		return n.exec(fns...)
	}

	for _, fn := range fns {
		if err := fn(n); err != nil {
			return err
		}
	}
	return nil
}

func (n *Node) promoteMember(ctx context.Context, id uint64, forwarded bool) error {
	err := n.preCond(
		joined(),
		notMember(id),
		noLeader(),
		notType(n.Whoami(), VoterMember),
		notType(id, LearnerMember),
		disableForwarding(),
		available(),
	)

	if err != nil {
		return err
	}

	rs, err := n.engine.Status()
	if err != nil {
		return err
	}

	// leader may lost during forwarding,
	// if there is no progress and promotion have been forwarded to this node.
	if rs.Progress == nil && forwarded {
		return raftengine.ErrNoLeader
	}

	mem, _ := n.GetMemebr(id)
	raw := mem.Raw()

	if rs.Progress == nil {
		lmem, ok := n.GetMemebr(rs.Lead)
		// leader lost, because rs.Lead = None.
		if !ok {
			return raftengine.ErrNoLeader
		}

		client, err := n.dial(ctx, lmem.Address())
		if err != nil {
			return err
		}

		n.cfg.logger.V(3).Infof("raft.node: forwarding member %x promotion to %x", id, lmem.ID())
		return client.PromoteMember(ctx, raw)
	}

	leader := rs.Progress[rs.Lead].Match
	learner := rs.Progress[id].Match
	// the learner's Match not caught up with the leader yet.
	if float64(learner) < float64(leader)*0.9 {
		return fmt.Errorf("raft: promotion failed, memebr %x not synced with the leader yet", id)
	}

	(&raw).Type = VoterMember

	return n.engine.ProposeConfChange(ctx, &raw, etcdraftpb.ConfChangeAddNode)
}

func joined() func(c *Node) error {
	return func(c *Node) error {
		if c.Whoami() == None {
			return fmt.Errorf("raft: node is not yet part of a raft cluster")
		}
		return nil
	}
}

func available() func(c *Node) error {
	return func(c *Node) error {
		reachables := c.members(func(m Member) bool {
			return m.IsActive() && m.Type() == VoterMember
		})

		voters := c.members(func(m Member) bool {
			return m.Type() == VoterMember
		})

		if len(reachables) < len(voters)/2+1 {
			return fmt.Errorf("raft: quorum lost and the cluster unavailable, no new logs can be committed")
		}
		return nil
	}
}

func notMember(id uint64) func(c *Node) error {
	return func(c *Node) error {
		if _, ok := c.GetMemebr(id); !ok {
			return fmt.Errorf("raft: unknown member %x", id)
		}
		return nil
	}
}

func memberRemoved(id uint64) func(c *Node) error {
	return func(c *Node) error {
		m, ok := c.GetMemebr(id)
		if ok && m.Type() == RemovedMember {
			return fmt.Errorf("raft: member %x removed", id)
		}
		return nil
	}
}

func addressInUse(mid uint64, addr string) func(c *Node) error {
	return func(c *Node) error {
		membs := c.members(func(m Member) bool {
			return m.Address() == addr && m.ID() != mid
		})

		if len(membs) > 0 {
			return fmt.Errorf("raft: address used by member %x", membs[0].ID())
		}
		return nil
	}
}

func notLeader() func(c *Node) error {
	return func(c *Node) error {
		if c.Whoami() != c.Leader() {
			return ErrNotLeader
		}
		return nil
	}
}

func leader(id uint64) func(c *Node) error {
	return func(c *Node) error {
		if id == c.Leader() {
			return fmt.Errorf("raft: operation not permitted, member %x is the leader, transfer leadership first", id)
		}
		return nil
	}
}

func idInUse(id uint64) func(c *Node) error {
	return func(c *Node) error {
		if _, ok := c.GetMemebr(id); ok {
			return fmt.Errorf("raft: id used by member %x", id)
		}
		return nil
	}
}

func noLeader() func(c *Node) error {
	return func(c *Node) error {
		if c.Leader() == None {
			return raftengine.ErrNoLeader
		}
		return nil
	}
}

func disableForwarding() func(c *Node) error {
	return func(c *Node) error {
		disable := c.cfg.rcfg.DisableProposalForwarding
		if c.Leader() != c.Whoami() && disable {
			return ErrNotLeader
		}
		return nil
	}
}

func notType(id uint64, t MemberType) func(c *Node) error {
	return func(c *Node) error {
		mem, _ := c.GetMemebr(id)
		if mt := mem.Type(); mt != t {
			return fmt.Errorf("raft: memebr (%x) is a %s not a %s", id, mt, t)
		}
		return nil
	}
}
