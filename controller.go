package raft

import (
	"context"

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

func (c *controller) Join(ctx context.Context, m *raftpb.Member) (uint64, []raftpb.Member, error) {
	var err error

	if mm, _ := c.node.GetMemebr(m.ID); mm == nil {
		err = c.node.AddMember(ctx, m)
	} else {
		err = c.node.UpdateMember(ctx, m)
	}

	if err != nil {
		return 0, []raftpb.Member{}, err
	}

	memb, _ := c.node.GetMemebr(m.ID)
	pool := c.pool.Snapshot()
	return memb.ID(), pool, nil
}

func (c *controller) Push(ctx context.Context, m etcdraftpb.Message) error {
	return c.daemon.Push(m)
}

func (c *controller) PromoteMember(ctx context.Context, m raftpb.Member) error {
	return c.node.promoteMember(ctx, m.ID, true)
}
