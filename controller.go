package raft

import (
	"context"
	"fmt"
	"sync"

	"github.com/shaj13/raftkit/internal/daemon"
	"github.com/shaj13/raftkit/internal/membership"
	"github.com/shaj13/raftkit/internal/raftpb"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

type controller struct {
	node   *Node
	daemon daemon.Daemon
	pool   membership.Pool
}

func (c *controller) Join(ctx context.Context, gid uint64, m *raftpb.Member) (*raftpb.JoinResponse, error) {
	var err error

	if _, ok := c.node.GetMemebr(m.ID); !ok {
		err = c.node.AddMember(ctx, m)
	} else {
		err = c.node.UpdateMember(ctx, m)
	}

	if err != nil {
		return nil, err
	}

	resp := &raftpb.JoinResponse{
		ID:      m.ID,
		Members: c.pool.Snapshot(),
	}

	return resp, nil
}

func (c *controller) Push(ctx context.Context, gid uint64, m etcdraftpb.Message) error {
	return c.daemon.Push(m)
}

func (c *controller) PromoteMember(ctx context.Context, gid uint64, m raftpb.Member) error {
	return c.node.promoteMember(ctx, m.ID, true)
}

type router struct {
	mu    sync.Mutex
	ctrls map[uint64]*controller
}

func (r *router) register(gid uint64, ctrl *controller) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ctrls[gid] = ctrl
}

func (r *router) get(gid uint64) (*controller, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ctrl, ok := r.ctrls[gid]
	if !ok {
		return nil, fmt.Errorf("raft: unknown group id %x", gid)
	}
	return ctrl, nil
}

func (r *router) Join(ctx context.Context, gid uint64, m *raftpb.Member) (*raftpb.JoinResponse, error) {
	ctrl, err := r.get(gid)
	if err != nil {
		return nil, err
	}
	return ctrl.Join(ctx, gid, m)
}

func (r *router) Push(ctx context.Context, gid uint64, m etcdraftpb.Message) error {
	ctrl, err := r.get(gid)
	if err != nil {
		return err
	}
	return ctrl.Push(ctx, gid, m)
}

func (r *router) PromoteMember(ctx context.Context, gid uint64, m raftpb.Member) error {
	ctrl, err := r.get(gid)
	if err != nil {
		return err
	}
	return ctrl.PromoteMember(ctx, gid, m)
}
