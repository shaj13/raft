package raft

import (
	"context"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/daemon"
	"github.com/shaj13/raftkit/internal/membership"
	"github.com/shaj13/raftkit/internal/rpc"
	"github.com/shaj13/raftkit/internal/storage/disk"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func New(ctx context.Context, opts ...Option) (Cluster, interface{}) {
	cfg := newConfig(opts...)
	newServer, dialer := rpc.GRPC.Get()
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

func (c *controller) Join(ctx context.Context, m *api.Member) (uint64, []api.Member, error) {
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
		return 0, []api.Member{}, err
	}

	pool := c.pool.Snapshot()

	for i, m := range pool {
		if m.Type == api.LocalMember {
			(&m).Type = api.RemoteMember
			pool[i] = m
			continue
		}

		if m.ID == memb.ID() {
			(&m).Type = api.LocalMember
			pool[i] = m
			continue
		}
	}

	return memb.ID(), pool, nil
}

func (c *controller) Push(ctx context.Context, m raftpb.Message) error {
	return c.daemon.Push(m)
}
