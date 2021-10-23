package raft

import (
	"context"

	"github.com/shaj13/raftkit/internal/daemon"
	"github.com/shaj13/raftkit/internal/membership"
	"github.com/shaj13/raftkit/internal/raftpb"
	intrpc "github.com/shaj13/raftkit/internal/rpc"
	"github.com/shaj13/raftkit/internal/storage/disk"
	"github.com/shaj13/raftkit/rpc"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

func New(proto rpc.Proto, opts ...Option) (Cluster, interface{}) {
	cfg := newConfig(opts...)
	ctx := cfg.ctx
	newServer, dialer := intrpc.Proto(proto).Get()
	cfg.controller = new(controller)
	cfg.storage = disk.New(ctx, cfg)
	cfg.dial = dialer(ctx, cfg)
	cfg.pool = membership.New(ctx, cfg)
	cfg.daemon = daemon.New(ctx, cfg)

	cluster := new(cluster)
	cluster.pool = cfg.pool
	cluster.daemon = cfg.daemon

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
	var (
		memb Member
		err  error
	)

	if mm, _ := c.cluster.GetMemebr(m.ID); mm == nil {
		memb, err = c.cluster.AddMember(context.Background(), m.Address)
	} else {
		err = c.cluster.UpdateMember(context.Background(), m.ID, m.Address)
		memb, _ = c.cluster.GetMemebr(m.ID)
	}

	if err != nil {
		return 0, []raftpb.Member{}, err
	}

	pool := c.pool.Snapshot()

	for i, m := range pool {
		if m.Type == raftpb.LocalMember {
			(&m).Type = raftpb.RemoteMember
			pool[i] = m
			continue
		}

		if m.ID == memb.ID() {
			(&m).Type = raftpb.LocalMember
			pool[i] = m
			continue
		}
	}

	return memb.ID(), pool, nil
}

func (c *controller) Push(ctx context.Context, m etcdraftpb.Message) error {
	return c.daemon.Push(m)
}
