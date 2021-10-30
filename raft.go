package raft

import (
	"context"

	"github.com/shaj13/raftkit/internal/daemon"
	"github.com/shaj13/raftkit/internal/membership"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/storage/disk"
	itransport "github.com/shaj13/raftkit/internal/transport"
	"github.com/shaj13/raftkit/transport"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

func New(proto transport.Proto, opts ...Option) *Node {
	cfg := newConfig(opts...)
	ctx := cfg.ctx
	newHandler, dialer := itransport.Proto(proto).Get()
	cfg.controller = new(controller)
	cfg.storage = disk.New(ctx, cfg)
	cfg.dial = dialer(ctx, cfg)
	cfg.pool = membership.New(ctx, cfg)
	cfg.daemon = daemon.New(ctx, cfg)

	node := new(Node)
	node.pool = cfg.pool
	node.daemon = cfg.daemon
	node.storage = cfg.storage
	node.dial = cfg.dial
	node.disableForwarding = cfg.rcfg.DisableProposalForwarding
	node.handler = newHandler(ctx, cfg)

	cfg.controller.(*controller).node = node
	cfg.controller.(*controller).daemon = cfg.daemon
	cfg.controller.(*controller).pool = cfg.pool

	return node
}

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
