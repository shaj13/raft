package raft

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	membershipmock "github.com/shaj13/raft/internal/mocks/membership"
	raftenginemock "github.com/shaj13/raft/internal/mocks/raftengine"
	"github.com/shaj13/raft/internal/raftpb"
	"github.com/shaj13/raft/internal/transport"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

func TestControllerPush(t *testing.T) {
	ctrl := gomock.NewController(t)
	eng := raftenginemock.NewMockEngine(ctrl)
	eng.EXPECT().Push(gomock.Any()).Return(nil)
	c := new(controller)
	c.engine = eng
	err := c.Push(context.TODO(), 0, etcdraftpb.Message{})
	require.NoError(t, err)
}

func TestControllerPromoteMember(t *testing.T) {
	ctrl := gomock.NewController(t)
	eng := raftenginemock.NewMockEngine(ctrl)
	eng.EXPECT().Status().Return(raft.Status{}, ErrNotLeader).AnyTimes()
	n := new(Node)
	n.engine = eng
	n.exec = testPreCond
	c := new(controller)
	c.node = n
	err := c.PromoteMember(context.TODO(), 0, RawMember{})
	require.Equal(t, ErrNotLeader, err)
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
				eng := raftenginemock.NewMockEngine(ctrl)
				eng.EXPECT().Status().Return(raft.Status{}, nil)
				eng.
					EXPECT().
					ProposeConfChange(gomock.Any(), gomock.Any(), gomock.Eq(etcdraftpb.ConfChangeAddNode)).
					Return(ErrNotLeader)
				n := new(Node)
				n.exec = testPreCond
				n.engine = eng
				n.pool = pool
				c.node = n
			},
			raw: &RawMember{ID: 10},
			err: ErrNotLeader,
		},
		{
			expect: func(c *controller) {
				ctrl := gomock.NewController(t)
				pool := membershipmock.NewMockPool(ctrl)
				eng := raftenginemock.NewMockEngine(ctrl)
				mem := membershipmock.NewMockMember(ctrl)
				mem.EXPECT().Type().Return(VoterMember)
				pool.EXPECT().Get(gomock.Any()).Return(mem, true).MaxTimes(2)
				pool.EXPECT().Snapshot().Return(nil)
				eng.EXPECT().Status().Return(raft.Status{}, nil)
				eng.EXPECT().ProposeConfChange(gomock.Any(), gomock.Any(), gomock.Eq(etcdraftpb.ConfChangeUpdateNode)).Return(nil)
				n := new(Node)
				n.exec = testPreCond
				n.engine = eng
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
		resp, err := c.Join(context.TODO(), 0, tt.raw)
		require.Equal(t, tt.err, err)
		if tt.err == nil {
			require.NotNil(t, resp)
			require.Equal(t, tt.id, resp.ID)
		}
	}

}

func TestRouterMethodsErr(t *testing.T) {
	ctx := context.TODO()
	noGroup := uint64(0)

	table := []func(r *router) error{
		func(r *router) error {
			return r.PromoteMember(ctx, noGroup, raftpb.Member{})
		},
		func(r *router) error {
			_, err := r.Join(ctx, noGroup, nil)
			return err
		},
		func(r *router) error {
			return r.Push(ctx, noGroup, etcdraftpb.Message{})
		},
	}

	for _, tt := range table {
		r := new(router)
		err := tt(r)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown group")
	}
}

func TestRouterAddRemove(t *testing.T) {
	gid := uint64(100)
	ctrl := new(controller)
	r := new(router)
	r.ctrls = map[uint64]transport.Controller{}
	r.add(gid, ctrl)
	got, _ := r.get(gid)
	require.Equal(t, ctrl, got)

	r.remove(gid)
	_, err := r.get(gid)
	require.Error(t, err)
}
