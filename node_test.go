package raft

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/shaj13/raftkit/internal/membership"
	daemonmock "github.com/shaj13/raftkit/internal/mocks/daemon"
	membershipmock "github.com/shaj13/raftkit/internal/mocks/membership"
	storagemock "github.com/shaj13/raftkit/internal/mocks/storage"
	transportmock "github.com/shaj13/raftkit/internal/mocks/transport"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/transport"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/raft/v3/tracker"
)

func TestNodePreConditions(t *testing.T) {
	// the tests aims to verify that all node method calls the selected pre conditions and in order.
	ctrl := gomock.NewController(t)
	daemon := daemonmock.NewMockDaemon(ctrl)
	daemon.EXPECT().Status().Return(raft.Status{}, nil).AnyTimes()
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
				notType(0, 0),
				available(),
			},
		},
		{
			call: func(n *Node) error {
				_, _, err := n.Snapshot()
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
			call: func(n *Node) error { return n.StepDown(ctx) },
			expected: []func(c *Node) error{
				joined(),
				notLeader(),
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
		node.daemon = daemon
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
			fn:  func(n *Node) error { return errNotLeader },
			err: errNotLeader,
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
	daemon := daemonmock.NewMockDaemon(ctrl)
	daemon.EXPECT().Shutdown(gomock.Any())
	n := new(Node)
	n.daemon = daemon
	err := n.Shutdown(context.TODO())
	require.NoError(t, err)
}

func TestNodeLinearizableRead(t *testing.T) {
	ctrl := gomock.NewController(t)
	daemon := daemonmock.NewMockDaemon(ctrl)
	daemon.EXPECT().LinearizableRead(gomock.Any()).Return(nil)
	daemon.EXPECT().Status().Return(raft.Status{}, nil)
	n := new(Node)
	n.daemon = daemon
	n.exec = testPreCond
	err := n.LinearizableRead(context.TODO())
	require.NoError(t, err)
}

func TestNodeSnapshot(t *testing.T) {
	expected := "snap-path"
	ctrl := gomock.NewController(t)
	daemon := daemonmock.NewMockDaemon(ctrl)
	stg := storagemock.NewMockStorage(ctrl)
	shotter := storagemock.NewMockSnapshotter(ctrl)

	daemon.EXPECT().CreateSnapshot().Return(etcdraftpb.Snapshot{}, nil)
	stg.EXPECT().Snapshotter().Return(shotter)
	shotter.EXPECT().Reader(gomock.Any()).Return(expected, nil, nil)

	n := new(Node)
	n.daemon = daemon
	n.exec = testPreCond
	n.storage = stg
	got, _, err := n.Snapshot()

	require.NoError(t, err)
	require.Equal(t, expected, got)
}

func TestNodeTransferLeadership(t *testing.T) {
	id := uint64(10)
	ctrl := gomock.NewController(t)
	daemon := daemonmock.NewMockDaemon(ctrl)
	daemon.EXPECT().TransferLeadership(gomock.Any(), gomock.Eq(id)).Return(nil)
	daemon.EXPECT().Status().Return(raft.Status{}, nil)
	n := new(Node)
	n.daemon = daemon
	n.exec = testPreCond
	err := n.TransferLeadership(context.TODO(), id)
	require.NoError(t, err)
}

func TestNodeStepDown(t *testing.T) {
	ctrl := gomock.NewController(t)
	daemon := daemonmock.NewMockDaemon(ctrl)
	pool := membershipmock.NewMockPool(ctrl)
	m1 := membershipmock.NewMockMember(ctrl)
	m2 := membershipmock.NewMockMember(ctrl)

	for i, m := range []*membershipmock.MockMember{m1, m2} {
		m.EXPECT().ID().Return(uint64(i))
		m.EXPECT().Type().Return(VoterMember)
		m.EXPECT().IsActive().Return(true)
		m.EXPECT().ActiveSince().Return(time.Now().Add(time.Second * time.Duration(i)))
	}

	pool.EXPECT().Members().Return([]membership.Member{m1, m2})
	daemon.EXPECT().TransferLeadership(gomock.Any(), gomock.Eq(uint64(0)))

	n := new(Node)
	n.exec = testPreCond
	n.daemon = daemon
	n.pool = pool

	err := n.StepDown(context.TODO())
	require.NoError(t, err)

	pool.EXPECT().Members().Return(nil)
	err = n.StepDown(context.TODO())
	require.Error(t, err)
	require.Contains(t, err.Error(), "longest active member")
}

func TestNodeUpdateMember(t *testing.T) {
	ctrl := gomock.NewController(t)
	pool := membershipmock.NewMockPool(ctrl)
	m1 := membershipmock.NewMockMember(ctrl)
	daemon := daemonmock.NewMockDaemon(ctrl)
	daemon.EXPECT().ProposeConfChange(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	daemon.EXPECT().Status().Return(raft.Status{}, nil)
	m1.EXPECT().Type().Return(LearnerMember)
	pool.EXPECT().Get(gomock.Any()).Return(m1, true)

	raw := &RawMember{}
	n := new(Node)
	n.daemon = daemon
	n.pool = pool
	n.exec = testPreCond
	err := n.UpdateMember(context.TODO(), raw)
	require.NoError(t, err)
	require.Equal(t, LearnerMember, raw.Type)
}

func TestNodeRemoveMember(t *testing.T) {
	var raw *raftpb.Member
	ctrl := gomock.NewController(t)
	pool := membershipmock.NewMockPool(ctrl)
	m1 := membershipmock.NewMockMember(ctrl)
	daemon := daemonmock.NewMockDaemon(ctrl)
	daemon.
		EXPECT().
		ProposeConfChange(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, m *raftpb.Member, t etcdraftpb.ConfChangeType) error {
			raw = m
			return nil
		})
	daemon.EXPECT().Status().Return(raft.Status{}, nil)
	m1.EXPECT().Raw().Return(RawMember{})
	pool.EXPECT().Get(gomock.Any()).Return(m1, true)

	n := new(Node)
	n.daemon = daemon
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
	daemon := daemonmock.NewMockDaemon(ctrl)
	daemon.
		EXPECT().
		ProposeConfChange(gomock.Any(), gomock.Any(), gomock.Eq(etcdraftpb.ConfChangeAddLearnerNode)).
		Return(nil)
	daemon.EXPECT().Status().Return(raft.Status{}, nil)
	pool.EXPECT().NextID().Return(id)

	n := new(Node)
	n.daemon = daemon
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
	daemon := daemonmock.NewMockDaemon(ctrl)
	daemon.
		EXPECT().
		ProposeConfChange(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, m *raftpb.Member, t etcdraftpb.ConfChangeType) error {
			raw = m
			return nil
		})
	daemon.EXPECT().Status().Return(raft.Status{}, nil)
	m1.EXPECT().Raw().Return(RawMember{})
	pool.EXPECT().Get(gomock.Any()).Return(m1, true)

	n := new(Node)
	n.daemon = daemon
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
	daemon := daemonmock.NewMockDaemon(ctrl)
	daemon.EXPECT().Status().Return(st, nil)
	n := new(Node)
	n.daemon = daemon
	require.Equal(t, st.Lead, n.Leader())
}

func TestNodeStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	daemon := daemonmock.NewMockDaemon(ctrl)
	daemon.EXPECT().Start(gomock.Any(), gomock.Any()).Return(nil)
	n := new(Node)
	n.daemon = daemon
	err := n.Start()
	require.NoError(t, err)
}

func TestNodePromoteMember(t *testing.T) {
	ctx := context.TODO()
	ctrl := gomock.NewController(t)
	daemon := daemonmock.NewMockDaemon(ctrl)
	pool := membershipmock.NewMockPool(ctrl)
	mem := membershipmock.NewMockMember(ctrl)
	client := transportmock.NewMockClient(ctrl)
	n := new(Node)
	n.exec = testPreCond
	n.daemon = daemon
	n.pool = pool

	daemon.EXPECT().Status().Return(raft.Status{}, nil).AnyTimes()
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
		return nil, errNotLeader
	}
	err = n.promoteMember(ctx, 1, false)
	require.Equal(t, errNotLeader, err)

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
	daemon = daemonmock.NewMockDaemon(ctrl)
	daemon.EXPECT().Status().Return(st, nil).AnyTimes()
	n.daemon = daemon
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
	daemon = daemonmock.NewMockDaemon(ctrl)
	daemon.EXPECT().Status().Return(st, nil).AnyTimes()
	daemon.EXPECT().ProposeConfChange(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	n.daemon = daemon
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
				daemon := daemonmock.NewMockDaemon(ctrl)
				daemon.EXPECT().Status().Return(raft.Status{}, nil).MaxTimes(2)
				n.daemon = daemon
			},
		},
		{
			fn:       disableForwarding(),
			contains: errNotLeader.Error(),
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				daemon := daemonmock.NewMockDaemon(ctrl)
				daemon.EXPECT().Status().Return(raft.Status{
					BasicStatus: raft.BasicStatus{
						ID: 12,
					},
				}, nil).MaxTimes(2)
				n.daemon = daemon
				n.disableForwarding = true
			},
		},
		{
			fn:       noLeader(),
			contains: "no elected cluster leader",
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				daemon := daemonmock.NewMockDaemon(ctrl)
				daemon.EXPECT().Status().Return(raft.Status{}, nil)
				n.daemon = daemon
			},
		},
		{
			fn:       noLeader(),
			contains: nilErr.Error(),
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				daemon := daemonmock.NewMockDaemon(ctrl)
				daemon.EXPECT().Status().Return(raft.Status{
					BasicStatus: raft.BasicStatus{
						SoftState: raft.SoftState{Lead: 10},
					},
				}, nil)
				n.daemon = daemon
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
				daemon := daemonmock.NewMockDaemon(ctrl)
				daemon.EXPECT().Status().Return(raft.Status{}, nil)
				n.daemon = daemon
			},
		},
		{
			fn:       leader(0),
			contains: "is the leader",
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				daemon := daemonmock.NewMockDaemon(ctrl)
				daemon.EXPECT().Status().Return(raft.Status{}, nil)
				n.daemon = daemon
			},
		},
		{
			fn:       notLeader(),
			contains: errNotLeader.Error(),
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				daemon := daemonmock.NewMockDaemon(ctrl)
				daemon.EXPECT().Status().Return(raft.Status{
					BasicStatus: raft.BasicStatus{
						ID: 15,
					},
				}, nil).MaxTimes(2)
				n.daemon = daemon
			},
		},
		{
			fn:       notLeader(),
			contains: nilErr.Error(),
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				daemon := daemonmock.NewMockDaemon(ctrl)
				daemon.EXPECT().Status().Return(raft.Status{}, nil).MaxTimes(2)
				n.daemon = daemon
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
				daemon := daemonmock.NewMockDaemon(ctrl)
				daemon.EXPECT().Status().Return(raft.Status{}, nil)
				n.daemon = daemon
			},
		},
		{
			fn:       joined(),
			contains: nilErr.Error(),
			expect: func(n *Node) {
				ctrl := gomock.NewController(t)
				daemon := daemonmock.NewMockDaemon(ctrl)
				daemon.EXPECT().Status().Return(raft.Status{
					BasicStatus: raft.BasicStatus{
						ID: 1,
					},
				}, nil)
				n.daemon = daemon
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

func testPreCond(fns ...func(c *Node) error) error {
	return nil
}
