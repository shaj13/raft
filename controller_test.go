package raft

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	daemonmock "github.com/shaj13/raftkit/internal/mocks/daemon"
	membershipmock "github.com/shaj13/raftkit/internal/mocks/membership"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

func TestControllerPush(t *testing.T) {
	ctrl := gomock.NewController(t)
	daemon := daemonmock.NewMockDaemon(ctrl)
	daemon.EXPECT().Push(gomock.Any()).Return(nil)
	c := new(controller)
	c.daemon = daemon
	err := c.Push(context.TODO(), etcdraftpb.Message{})
	require.NoError(t, err)
}

func TestControllerPromoteMember(t *testing.T) {
	ctrl := gomock.NewController(t)
	daemon := daemonmock.NewMockDaemon(ctrl)
	daemon.EXPECT().Status().Return(raft.Status{}, errNotLeader).AnyTimes()
	n := new(Node)
	n.daemon = daemon
	n.exec = testPreCond
	c := new(controller)
	c.node = n
	err := c.PromoteMember(context.TODO(), RawMember{})
	require.Equal(t, errNotLeader, err)
}

func TestControllerJoin(t *testing.T) {
	table := []struct {
		expect func(c *controller)
		raw    *RawMember
		err    error
		id     uint64
	}{
		{
			expect: func(c *controller) {
				ctrl := gomock.NewController(t)
				pool := membershipmock.NewMockPool(ctrl)
				pool.EXPECT().Get(gomock.Any()).Return(nil, false)
				daemon := daemonmock.NewMockDaemon(ctrl)
				daemon.EXPECT().Status().Return(raft.Status{}, nil)
				daemon.EXPECT().ProposeConfChange(gomock.Any(), gomock.Any(), gomock.Eq(etcdraftpb.ConfChangeAddNode)).Return(errNotLeader)
				n := new(Node)
				n.exec = testPreCond
				n.daemon = daemon
				n.pool = pool
				c.node = n
			},
			raw: &RawMember{ID: 10},
			err: errNotLeader,
		},
		{
			expect: func(c *controller) {
				ctrl := gomock.NewController(t)
				pool := membershipmock.NewMockPool(ctrl)
				daemon := daemonmock.NewMockDaemon(ctrl)
				mem := membershipmock.NewMockMember(ctrl)
				mem.EXPECT().Type().Return(VoterMember)
				pool.EXPECT().Get(gomock.Any()).Return(mem, true).MaxTimes(2)
				pool.EXPECT().Snapshot().Return(nil)
				daemon.EXPECT().Status().Return(raft.Status{}, nil)
				daemon.EXPECT().ProposeConfChange(gomock.Any(), gomock.Any(), gomock.Eq(etcdraftpb.ConfChangeUpdateNode)).Return(nil)
				n := new(Node)
				n.exec = testPreCond
				n.daemon = daemon
				n.pool = pool
				c.node = n
				c.pool = pool
			},
			raw: &RawMember{ID: 123},
			id:  123,
		},
	}

	for _, tt := range table {
		c := new(controller)
		tt.expect(c)
		resp, err := c.Join(context.TODO(), tt.raw)
		require.Equal(t, tt.err, err)
		if tt.err == nil {
			require.NotNil(t, resp)
			require.Equal(t, tt.id, resp.ID)
		}
	}

}
