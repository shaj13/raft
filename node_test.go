package raft

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/shaj13/raft/internal/membership"
	membershipmock "github.com/shaj13/raft/internal/mocks/membership"
	raftenginemock "github.com/shaj13/raft/internal/mocks/raftengine"
	storagemock "github.com/shaj13/raft/internal/mocks/storage"
	transportmock "github.com/shaj13/raft/internal/mocks/transport"
	"github.com/shaj13/raft/internal/raftengine"
	"github.com/shaj13/raft/internal/raftpb"
	"github.com/shaj13/raft/internal/transport"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/raft/v3/tracker"
)

func TestNodePreConditions(t *testing.T) {
	// the tests aims to verify that all node method calls the selected pre conditions and in order.
	ctrl := gomock.NewController(t)
	eng := raftenginemock.NewMockEngine(ctrl)
	eng.EXPECT().Status().Return(raft.Status{}, nil).AnyTimes()
	ctx := context.TODO()

	table := []struct {
		call     func(n *Node) error
		expected []func(c *Node) error
	}{
		{
			call: func(n *Node) error { return n.LinearizableRead(ctx) },
			expected: []func(c *Node) error{
				joined(),
				noLeader(),
				available(),
			},
		},
		{
			call: func(n *Node) error {
				_, err := n.Snapshot()
				return err
			},
			expected: []func(c *Node) error{
				joined(),
			},
		},
		{
			call: func(n *Node) error { return n.TransferLeadership(ctx, 0) },
			expected: []func(c *Node) error{
				joined(),
				notMember(0),
				memberRemoved(0),
				noLeader(),
				notType(0, 0),
				disableForwarding(),
				available(),
			},
		},
		{
			call: func(n *Node) error { return n.Stepdown(ctx) },
			expected: []func(c *Node) error{
				joined(),
				notLeader(),
				available(),
			},
		},
		{
			call: func(n *Node) error { return n.Replicate(ctx, nil) },
			expected: []func(c *Node) error{
				joined(),
				noLeader(),
				notType(0, 0),
				disableForwarding(),
				available(),
			},
		},
		{
			call: func(n *Node) error { return n.UpdateMember(ctx, new(RawMember)) },
			expected: []func(c *Node) error{
				joined(),
				notMember(0),
				memberRemoved(0),
				noLeader(),
				notType(0, 0),
				addressInUse(0, ""),
				disableForwarding(),
				available(),
			},
		},
		{
			call: func(n *Node) error { return n.RemoveMember(ctx, 0) },
			expected: []func(c *Node) error{
				joined(),
				notMember(0),
				memberRemoved(0),
				leader(0),
				noLeader(),
				notType(0, 0),
				disableForwarding(),
				available(),
			},
		},
		{
			call: func(n *Node) error { return n.AddMember(ctx, new(RawMember)) },
			expected: []func(c *Node) error{
				joined(),
				idInUse(0),
				addressInUse(0, ""),
				noLeader(),
				notType(0, 0),
				disableForwarding(),
				available(),
			},
		},
		{
			call: func(n *Node) error { return n.PromoteMember(ctx, 0) },
			expected: []func(c *Node) error{
				joined(),
				notMember(0),
				noLeader(),
				notType(0, 0),
				notType(0, 0),
				disableForwarding(),
				available(),
			},
		},
		{
			call: func(n *Node) error { return n.DemoteMember(ctx, 0) },
			expected: []func(c *Node) error{
				joined(),
				notMember(0),
				memberRemoved(0),
				noLeader(),
				leader(0),
				notType(0, 0),
				notType(0, 0),
				disableForwarding(),
				available(),
			},
		},
	}

	for _, tt := range table {
		terr := fmt.Errorf("TestNodePreConditions")
		got := []func(c *Node) error{}
		node := new(Node)
		node.engine = eng
		node.exec = func(fns ...func(c *Node) error) error {
			got = fns
			return terr
		}

		err := tt.call(node)
		require.Equal(t, terr, err)
		require.Equal(t, len(tt.expected), len(got))

		for i, fn := range got {
			require.Equal(t, fmt.Sprintf("%p", tt.expected[i]), fmt.Sprintf("%p", fn))
		}
	}
}

func TestNodePreCond(t *testing.T) {
	table := []struct {
		fn  func(n *Node) error
		err error
	}{
		{
			fn:  func(n *Node) error { return ErrNotLeader },
			err: ErrNotLeader,
		},
		{
			fn:  func(n *Node) error { return nil },
			err: nil,
		},
	}

	for _, tt := range table {
		n := new(Node)
		err := n.preCond(tt.fn)
		require.Equal(t, tt.err, err)
	}
}

func TestNodeHandler(t *testing.T) {
	h := transport.Handler("TestHandler")
	n := new(Node)
	n.handler = h
	require.Equal(t, h, n.Handler())
}

func TestShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	eng := raftenginemock.NewMockEngine(ctrl)
	eng.EXPECT().Shutdown(gomock.Any())
	n := new(Node)
	n.engine = eng
	err := n.Shutdown(context.TODO())
	require.NoError(t, err)
}

func TestNodeLinearizableRead(t *testing.T) {
	ctrl := gomock.NewController(t)
	eng := raftenginemock.NewMockEngine(ctrl)
	eng.EXPECT().LinearizableRead(gomock.Any()).Return(nil)
	eng.EXPECT().Status().Return(raft.Status{}, nil)
	n := new(Node)
	n.engine = eng
	n.exec = testPreCond
	err := n.LinearizableRead(context.TODO())
	require.NoError(t, err)
}

func TestNodeSnapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	eng := raftenginemock.NewMockEngine(ctrl)
	stg := storagemock.NewMockStorage(ctrl)
	shotter := storagemock.NewMockSnapshotter(ctrl)

	eng.EXPECT().CreateSnapshot().Return(etcdraftpb.Snapshot{}, nil)
	stg.EXPECT().Snapshotter().Return(shotter)
	shotter.EXPECT().Reader(gomock.Any(), gomock.Any()).Return(nil, nil)

	n := new(Node)
	n.engine = eng
	n.exec = testPreCond
	n.storage = stg
	_, err := n.Snapshot()
	require.NoError(t, err)
}

func TestNodeTransferLeadership(t *testing.T) {
	id := uint64(10)
	ctrl := gomock.NewController(t)
	eng := raftenginemock.NewMockEngine(ctrl)
	eng.EXPECT().TransferLeadership(gomock.Any(), gomock.Eq(id)).Return(nil)
	eng.EXPECT().Status().Return(raft.Status{}, nil)
	n := new(Node)
	n.engine = eng
	n.exec = testPreCond
	err := n.TransferLeadership(context.TODO(), id)
	require.NoError(t, err)
}

func TestNodeStepDown(t *testing.T) {
	ctrl := gomock.NewController(t)
	eng := raftenginemock.NewMockEngine(ctrl)
	pool := membershipmock.NewMockPool(ctrl)
	m1 := membershipmock.NewMockMember(ctrl)
	m2 := membershipmock.NewMockMember(ctrl)
	m3 := membershipmock.NewMockMember(ctrl)

	for i, m := range []*membershipmock.MockMember{m1, m2, m3} {
		m.EXPECT().ID().Return(uint64(i)).AnyTimes()
		m.EXPECT().Type().Return(VoterMember)
		m.EXPECT().IsActive().Return(true)
		m.EXPECT().ActiveSince().Return(time.Now().Add(time.Second * time.Duration(i)))
	}

	pool.EXPECT().Members().Return([]membership.Member{m1, m2})
	eng.EXPECT().Status().Return(raft.Status{}, nil).AnyTimes()
	eng.EXPECT().TransferLeadership(gomock.Any(), gomock.Eq(uint64(1)))

	n := new(Node)
	n.exec = testPreCond
	n.engine = eng
	n.pool = pool

	err := n.Stepdown(context.TODO())
	require.NoError(t, err)

	pool.EXPECT().Members().Return(nil)
	err = n.Stepdown(context.TODO())
	require.Error(t, err)
	require.Contains(t, err.Error(), "longest active member")
}

func TestNodeUpdateMember(t *testing.T) {
	ctrl := gomock.NewController(t)
	pool := membershipmock.NewMockPool(ctrl)
	m1 := membershipmock.NewMockMember(ctrl)
	eng := raftenginemock.NewMockEngine(ctrl)
	eng.EXPECT().ProposeConfChange(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	eng.EXPECT().Status().Return(raft.Status{}, nil)
	m1.EXPECT().Type().Return(LearnerMember)
	pool.EXPECT().Get(gomock.Any()).Return(m1, true)

	raw := &RawMember{}
	n := new(Node)
	n.engine = eng
	n.pool = pool
	n.exec = testPreCond
	err := n.UpdateMember(context.TODO(), raw)
	require.NoError(t, err)
	require.Equal(t, LearnerMember, raw.Type)
}

func TestNodeReplicate(t *testing.T) {
	ctrl := gomock.NewController(t)
	eng := raftenginemock.NewMockEngine(ctrl)
	eng.EXPECT().ProposeReplicate(gomock.Any(), gomock.Any()).Return(nil)
	eng.EXPECT().Status().Return(raft.Status{}, nil)

	n := new(Node)
	n.engine = eng
	n.exec = testPreCond
	err := n.Replicate(context.TODO(), nil)
	require.NoError(t, err)
}

func TestNodeRemoveMember(t *testing.T) {
	var raw *raftpb.Member
	ctrl := gomock.NewController(t)
	pool := membershipmock.NewMockPool(ctrl)
	m1 := membershipmock.NewMockMember(ctrl)
	eng := raftenginemock.NewMockEngine(ctrl)
	eng.
		EXPECT().
		ProposeConfChange(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, m *raftpb.Member, t etcdraftpb.ConfChangeType) error {
			raw = m
			return nil
		})
	eng.EXPECT().Status().Return(raft.Status{}, nil)
	m1.EXPECT().Raw().Return(RawMember{})
	pool.EXPECT().Get(gomock.Any()).Return(m1, true)

	n := new(Node)
	n.engine = eng
	n.pool = pool
	n.exec = testPreCond
	err := n.RemoveMember(context.TODO(), 0)
	require.NoError(t, err)
	require.Equal(t, RemovedMember, raw.Type)
}

func TestNodeAddMember(t *testing.T) {
	id := uint64(10)
	raw := &raftpb.Member{Type: LearnerMember}
	ctrl := gomock.NewController(t)
	pool := membershipmock.NewMockPool(ctrl)
	eng := raftenginemock.NewMockEngine(ctrl)
	eng.
		EXPECT().
		ProposeConfChange(gomock.Any(), gomock.Any(), gomock.Eq(etcdraftpb.ConfChangeAddLearnerNode)).
		Return(nil)
	eng.EXPECT().Status().Return(raft.Status{}, nil)
	pool.EXPECT().NextID().Return(id)

	n := new(Node)
	n.engine = eng
	n.pool = pool
	n.exec = testPreCond
	err := n.AddMember(context.TODO(), raw)
	require.NoError(t, err)
	require.Equal(t, id, raw.ID)
}

func TestNodeDemoteMember(t *testing.T) {
	var raw *raftpb.Member
	ctrl := gomock.NewController(t)
	pool := membershipmock.NewMockPool(ctrl)
	m1 := membershipmock.NewMockMember(ctrl)
	eng := raftenginemock.NewMockEngine(ctrl)
	eng.
		EXPECT().
		ProposeConfChange(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, m *raftpb.Member, t etcdraftpb.ConfChangeType) error {
			raw = m
			return nil
		})
	eng.EXPECT().Status().Return(raft.Status{}, nil)
	m1.EXPECT().Raw().Return(RawMember{})
	pool.EXPECT().Get(gomock.Any()).Return(m1, true)

	n := new(Node)
	n.engine = eng
	n.pool = pool
	n.exec = testPreCond
	err := n.DemoteMember(context.TODO(), 0)
	require.NoError(t, err)
	require.Equal(t, LearnerMember, raw.Type)
}

func TestNodeMembers(t *testing.T) {
	ctrl := gomock.NewController(t)
	pool := membershipmock.NewMockPool(ctrl)
	m1 := membershipmock.NewMockMember(ctrl)
	m2 := membershipmock.NewMockMember(ctrl)
	pool.EXPECT().Members().Return([]membership.Member{m1, m2})
	n := new(Node)
	n.pool = pool
	membs := n.Members()
	require.Equal(t, 2, len(membs))
}

func TestNNodeLeader(t *testing.T) {
	st := raft.Status{
		BasicStatus: raft.BasicStatus{
			SoftState: raft.SoftState{Lead: 10},
		},
	}
	ctrl := gomock.NewController(t)
	eng := raftenginemock.NewMockEngine(ctrl)
	eng.EXPECT().Status().Return(st, nil)
	n := new(Node)
	n.engine = eng
	require.Equal(t, st.Lead, n.Leader())
}

func TestNodeStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	eng := raftenginemock.NewMockEngine(ctrl)
	eng.EXPECT().Start(gomock.Any(), gomock.Any()).Return(nil)
	n := new(Node)
	n.engine = eng
	err := n.Start()
	require.NoError(t, err)
}

func TestNodePromoteMember(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	eng := raftenginemock.NewMockEngine(ctrl)
	pool := membershipmock.NewMockPool(ctrl)
	mem := membershipmock.NewMockMember(ctrl)
	client := transportmock.NewMockClient(ctrl)
	n := new(Node)
	n.exec = testPreCond
	n.engine = eng
	n.pool = pool
	n.cfg = newConfig()

	eng.EXPECT().Status().Return(raft.Status{}, nil).AnyTimes()
	pool.EXPECT().Get(gomock.Any()).Return(mem, true).AnyTimes()
	mem.EXPECT().Raw().Return(RawMember{}).AnyTimes()
	mem.EXPECT().Address().Return("").AnyTimes()
	mem.EXPECT().ID().Return(uint64(1)).AnyTimes()
	client.EXPECT().PromoteMember(gomock.Any(), gomock.Any()).Return(nil)

	// round #1 it return error when promote forwarded
	// and leader lost.
	err := n.promoteMember(ctx, 1, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no elected cluster leader")

	// round #2 it return error when dial return error
	n.dial = func(c context.Context, s string) (transport.Client, error) {
		return nil, ErrNotLeader
	}
	err = n.promoteMember(ctx, 1, false)
	require.Equal(t, ErrNotLeader, err)

	// round #3 it forward promote request.
	n.dial = func(c context.Context, s string) (transport.Client, error) {
		return client, nil
	}
	err = n.promoteMember(ctx, 1, false)
	require.NoError(t, err)

	// round #4 it return error when member not ready yet.
	st := raft.Status{
		BasicStatus: raft.BasicStatus{
			SoftState: raft.SoftState{Lead: 10},
		},
		Progress: map[uint64]tracker.Progress{
			10: {Match: 100},
		},
	}
	eng = raftenginemock.NewMockEngine(ctrl)
	eng.EXPECT().Status().Return(st, nil).AnyTimes()
	n.engine = eng
	err = n.promoteMember(ctx, 1, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not synced with the leader yet")

	// round #5 it return promote member.
	st = raft.Status{
		BasicStatus: raft.BasicStatus{
			SoftState: raft.SoftState{Lead: 10},
		},
		Progress: map[uint64]tracker.Progress{
			10: {Match: 100},
			1:  {Match: 90},
		},
	}
	eng = raftenginemock.NewMockEngine(ctrl)
	eng.EXPECT().Status().Return(st, nil).AnyTimes()
	eng.EXPECT().ProposeConfChange(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	n.engine = eng
	err = n.promoteMember(ctx, 1, false)
	require.NoError(t, err)
}

func TestPreConditions(t *testing.T) {
	nilErr := fmt.Errorf("<nil>")
	table := []struct {
		fn       func(c *Node) error
		contains string
		expect   func(c *Node)
	}{
		{
			fn:       notType(1, VoterMember),
			contains: nilErr.Error(),
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				pool := membershipmock.NewMockPool(ctrl)
				mem := membershipmock.NewMockMember(ctrl)
				pool.EXPECT().Get(gomock.Eq(uint64(1))).Return(mem, true)
				mem.EXPECT().Type().Return(VoterMember)
				n.pool = pool
			},
		},
		{
			fn:       notType(1, VoterMember),
			contains: "not a voter",
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				pool := membershipmock.NewMockPool(ctrl)
				mem := membershipmock.NewMockMember(ctrl)
				pool.EXPECT().Get(gomock.Eq(uint64(1))).Return(mem, true)
				mem.EXPECT().Type().Return(LearnerMember)
				n.pool = pool
			},
		},
		{
			fn:       disableForwarding(),
			contains: nilErr.Error(),
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				eng := raftenginemock.NewMockEngine(ctrl)
				eng.EXPECT().Status().Return(raft.Status{}, nil).MaxTimes(2)
				n.engine = eng
				n.cfg = newConfig()
			},
		},
		{
			fn:       disableForwarding(),
			contains: ErrNotLeader.Error(),
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				eng := raftenginemock.NewMockEngine(ctrl)
				eng.EXPECT().Status().Return(raft.Status{
					BasicStatus: raft.BasicStatus{
						ID: 12,
					},
				}, nil).MaxTimes(2)
				n.engine = eng
				n.cfg = newConfig(WithDisableProposalForwarding())
			},
		},
		{
			fn:       noLeader(),
			contains: "no elected cluster leader",
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				eng := raftenginemock.NewMockEngine(ctrl)
				eng.EXPECT().Status().Return(raft.Status{}, nil)
				n.engine = eng
			},
		},
		{
			fn:       noLeader(),
			contains: nilErr.Error(),
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				eng := raftenginemock.NewMockEngine(ctrl)
				eng.EXPECT().Status().Return(raft.Status{
					BasicStatus: raft.BasicStatus{
						SoftState: raft.SoftState{Lead: 10},
					},
				}, nil)
				n.engine = eng
			},
		},
		{
			fn:       idInUse(1),
			contains: "id used by member",
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				pool := membershipmock.NewMockPool(ctrl)
				pool.EXPECT().Get(gomock.Any()).Return(nil, true)
				n.pool = pool
			},
		},
		{
			fn:       idInUse(1),
			contains: nilErr.Error(),
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				pool := membershipmock.NewMockPool(ctrl)
				pool.EXPECT().Get(gomock.Any()).Return(nil, false)
				n.pool = pool
			},
		},
		{
			fn:       leader(1),
			contains: nilErr.Error(),
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				eng := raftenginemock.NewMockEngine(ctrl)
				eng.EXPECT().Status().Return(raft.Status{}, nil)
				n.engine = eng
			},
		},
		{
			fn:       leader(0),
			contains: "is the leader",
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				eng := raftenginemock.NewMockEngine(ctrl)
				eng.EXPECT().Status().Return(raft.Status{}, nil)
				n.engine = eng
			},
		},
		{
			fn:       notLeader(),
			contains: ErrNotLeader.Error(),
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				eng := raftenginemock.NewMockEngine(ctrl)
				eng.EXPECT().Status().Return(raft.Status{
					BasicStatus: raft.BasicStatus{
						ID: 15,
					},
				}, nil).MaxTimes(2)
				n.engine = eng
			},
		},
		{
			fn:       notLeader(),
			contains: nilErr.Error(),
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				eng := raftenginemock.NewMockEngine(ctrl)
				eng.EXPECT().Status().Return(raft.Status{}, nil).MaxTimes(2)
				n.engine = eng
			},
		},
		{
			fn:       addressInUse(1, "addr"),
			contains: "address used by member",
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				pool := membershipmock.NewMockPool(ctrl)
				mem := membershipmock.NewMockMember(ctrl)
				pool.EXPECT().Members().Return([]membership.Member{mem})
				mem.EXPECT().ID().Return(uint64(2)).AnyTimes()
				mem.EXPECT().Address().Return("addr")
				n.pool = pool
			},
		},
		{
			fn:       addressInUse(0, ""),
			contains: nilErr.Error(),
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				pool := membershipmock.NewMockPool(ctrl)
				pool.EXPECT().Members().Return([]membership.Member{})
				n.pool = pool
			},
		},
		{
			fn:       memberRemoved(0),
			contains: "removed",
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				pool := membershipmock.NewMockPool(ctrl)
				mem := membershipmock.NewMockMember(ctrl)
				pool.EXPECT().Get(gomock.Any()).Return(mem, true)
				mem.EXPECT().Type().Return(RemovedMember)
				n.pool = pool
			},
		},
		{
			fn:       memberRemoved(0),
			contains: nilErr.Error(),
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				pool := membershipmock.NewMockPool(ctrl)
				pool.EXPECT().Get(gomock.Any()).Return(nil, false)
				n.pool = pool
			},
		},
		{
			fn:       notMember(0),
			contains: nilErr.Error(),
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				pool := membershipmock.NewMockPool(ctrl)
				pool.EXPECT().Get(gomock.Any()).Return(nil, true)
				n.pool = pool
			},
		},
		{
			fn:       notMember(0),
			contains: "unknown member",
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				pool := membershipmock.NewMockPool(ctrl)
				pool.EXPECT().Get(gomock.Any()).Return(nil, false)
				n.pool = pool
			},
		},
		{
			fn:       available(),
			contains: " quorum lost",
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				pool := membershipmock.NewMockPool(ctrl)
				m1 := membershipmock.NewMockMember(ctrl)
				m2 := membershipmock.NewMockMember(ctrl)
				m1.EXPECT().Type().Return(VoterMember).MaxTimes(2)
				m2.EXPECT().Type().Return(VoterMember).MaxTimes(2)
				m1.EXPECT().IsActive().Return(false)
				m2.EXPECT().IsActive().Return(true)
				pool.EXPECT().Members().Return([]membership.Member{m1, m2}).MaxTimes(2)
				n.pool = pool
			},
		},
		{
			fn:       available(),
			contains: nilErr.Error(),
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				pool := membershipmock.NewMockPool(ctrl)
				m1 := membershipmock.NewMockMember(ctrl)
				m1.EXPECT().Type().Return(VoterMember).MaxTimes(2)
				m1.EXPECT().IsActive().Return(true)
				pool.EXPECT().Members().Return([]membership.Member{m1}).MaxTimes(2)
				n.pool = pool
			},
		},
		{
			fn:       joined(),
			contains: "not yet part of a raft",
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				eng := raftenginemock.NewMockEngine(ctrl)
				eng.EXPECT().Status().Return(raft.Status{}, nil)
				n.engine = eng
			},
		},
		{
			fn:       joined(),
			contains: nilErr.Error(),
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				eng := raftenginemock.NewMockEngine(ctrl)
				eng.EXPECT().Status().Return(raft.Status{
					BasicStatus: raft.BasicStatus{
						ID: 1,
					},
				}, nil)
				n.engine = eng
			},
		},
	}

	for _, tt := range table {
		n := new(Node)
		tt.expect(n)
		err := tt.fn(n)
		if err == nil {
			err = nilErr
		}
		require.Contains(t, err.Error(), tt.contains)
	}
}

func TestNodeGroupHandler(t *testing.T) {
	expected := "handler"
	ng := &NodeGroup{
		handler: expected,
	}
	require.Equal(t, expected, ng.handler)
}

func TestNodeGroupAdd(t *testing.T) {
	ctrl := gomock.NewController(t)
	eng := raftenginemock.NewMockEngine(ctrl)
	eng.EXPECT().Status().Return(raft.Status{}, nil)

	ng := &NodeGroup{
		router: &router{
			ctrls: make(map[uint64]transport.Controller),
		},
		mux:     raftengine.NewMux(),
		handler: "ng-handler",
	}

	node := &Node{
		engine: eng,
	}

	// it refuse to add node when engine started.
	ok := ng.Add(1, node)
	require.False(t, ok)

	eng = raftenginemock.NewMockEngine(ctrl)
	eng.EXPECT().Status().Return(raft.Status{}, ErrNodeStopped)

	node.engine = eng
	node.cfg = newConfig()

	// it add node and change some of config.
	ok = ng.Add(1, node)
	require.True(t, ok)
	require.Equal(t, uint64(1), node.cfg.groupID)
	require.Equal(t, ng.mux, node.cfg.mux)
	require.Equal(t, ng.router, node.cfg.controller)
	require.Equal(t, ng.handler, node.handler)
}

func testPreCond(fns ...func(c *Node) error) error {
	return nil
}
