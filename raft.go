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

func New(proto transport.Proto, opts ...Option) (Cluster, interface{}) {
	cfg := newConfig(opts...)
	ctx := cfg.ctx
	newServer, dialer := itransport.Proto(proto).Get()
	cfg.controller = new(controller)
	cfg.storage = disk.New(ctx, cfg)
	cfg.dial = dialer(ctx, cfg)
	cfg.pool = membership.New(ctx, cfg)
	cfg.daemon = daemon.New(ctx, cfg)

	cluster := new(cluster)
	cluster.pool = cfg.pool
	cluster.daemon = cfg.daemon
	cluster.storage = cfg.storage
	cluster.dial = cfg.dial
	cluster.disableForwarding = cfg.rcfg.DisableProposalForwarding

	cfg.controller.(*controller).cluster = cluster
	cfg.controller.(*controller).daemon = cfg.daemon
	cfg.controller.(*controller).pool = cfg.pool

	srv, _ := newServer(ctx, cfg)

	return cluster, srv
}

type controller struct {
	cluster *cluster
	daemon  daemon.Daemon
	pool    membership.Pool
}

func (c *controller) Join(ctx context.Context, m *raftpb.Member) (uint64, []raftpb.Member, error) {
	var err error

	if mm, _ := c.cluster.GetMemebr(m.ID); mm == nil {
		err = c.cluster.AddMember(ctx, m)
	} else {
		err = c.cluster.UpdateMember(ctx, m)
	}

	if err != nil {
		return 0, []raftpb.Member{}, err
	}

	memb, _ := c.cluster.GetMemebr(m.ID)
	pool := c.pool.Snapshot()
	return memb.ID(), pool, nil
}

func (c *controller) Push(ctx context.Context, m etcdraftpb.Message) error {
	return c.daemon.Push(m)
}

func (c *controller) PromoteMember(ctx context.Context, m raftpb.Member) error {
	return c.cluster.promoteMember(ctx, m.ID, true)
}
