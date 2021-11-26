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

var (
	// ErrNodeStopped is returned by the Node methods after a call to
	// Shutdown or when it has not started.
	ErrNodeStopped = raftengine.ErrStopped
	// ErrNotLeader is returned when an operation can't be completed on a
	// follower or candidate node
	ErrNotLeader = errors.New("raft: node is not the leader")
)

// NewNodeGroup returns a new NodeGroup.
// the provided transportation protocol must be the same for all sub-nodes.
//
//	nodeA := raft.New(gRPC, ...)
//	nodeB := raft.New(gRPC, ...)
//	ng := raft.NewNodeGroup(gRPC)
//
// the returned node group will lazily initialize,
// from the first node registered within it, So it's recommended to apply
// the same  HeartbeatTick, ElectionTick, and TickInterval configuration to all sub-nodes.
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

// NodeGroup manage multi Raft nodes from many different Raft groups known as Raft clusters.
// NodeGroup is more efficient than a collection of nodes.
//
// Scales raft into multiple raft groups requires data sharding,
// each raft group responsible for managing data in the range [start, end].
// as the system grows to include more ranges, so does the amount of traffic
// required to handle heartbeats. The number of ranges is much larger than the
// number of physical nodes so many ranges will have overlapping membership
// this is where NodeGroup comes in: instead of allowing each range to run Raft independently,
// we manage an entire node’s worth of ranges as a group. Each pair of physical nodes
// only needs to exchange heartbeats once per tick (coalesced heartbeats),
// no matter how many ranges they have in common.
//
// Add, and Remove can run while node group stopped.
// starting an added node is required a started node group,
// Otherwise, it will hang until the node group started.
type NodeGroup struct {
	mux     raftengine.Mux
	handler transport.Handler
	router  *router
}

// Handler return NodeGroup transportation handler,
// that delegated to respond to RPC requests over the wire.
// the returned handler must be registered with the transportation server.
func (ng *NodeGroup) Handler() etransport.Handler {
	return ng.handler
}

// Start starts the NodeGroup. It can be called after Stop to restart the
// NodeGroup.
// Start returns when Stop called.
func (ng *NodeGroup) Start() {
	ng.mux.Start()
}

// Add add the given node that associated to the given group id,
// and reports whether the node were successfully added.
//
// The node and the group are correlated so each group id must have
// its own node object and each node object must have its own group id.
//
// All registered nodes within the node group must have the same id,
// that is how multiple nodes object representing one single physical node
// that participate in multiple raft groups. Starting a node with a
// different id from the previous one will cause a panic.
//
// 	nodeA = id(1)
// 	nodeB = id(2)
// 	nodeGroup.Add(1, nodeA)
//	nodeGroup.Add(2, nodeB)
// 	nodeB.Start(...) // panic
//
// The provided node must be freshly created and has not been started.
//
// 	nodeA := raft.New(...)
//  nodeB := raft.New(...)
//  nodeGroup.Add(1, nodeA)
//	nodeGroup.Add(2, nodeB)
func (ng *NodeGroup) Add(groupID uint64, n *Node) bool {
	if _, err := n.engine.Status(); err == nil || n.cfg.groupID != None {
		return false
	}
	ng.router.add(groupID, n.cfg.controller)
	n.cfg.groupID = groupID
	n.cfg.mux = ng.mux
	n.cfg.controller = ng.router
	n.handler = ng.handler
	return true
}

// Remove remove node related to the given group id.
// after the removal, the actual node will become idle,
// it must coordinate with node shutdown explicitly.
//
// 	nodeGroup.Remove(12)
// 	node.Shutdown(ctx)
//
func (ng *NodeGroup) Remove(groupID uint64) {
	ng.router.remove(groupID)
}

// Stop performs any necessary termination of the NodeGroup.
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

// Handler return node transportation handler,
// that delegated to respond to RPC requests over the wire.
// the returned handler must be registered with the transportation server,
// unless the node is registered with a node group.
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
