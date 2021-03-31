package raft

import (
	"context"

	"github.com/shaj13/raftkit/internal/daemon"
	"github.com/shaj13/raftkit/internal/membership"
	raftrpc "github.com/shaj13/raftkit/internal/net/grpc"
	"github.com/shaj13/raftkit/internal/storage/disk"
)

func NewNew(ctx context.Context) (daemon.Daemon, interface{}) {
	cfg := defaultConfig()
	cfg.storage = disk.New(ctx, cfg)
	cfg.dial = raftrpc.Dialer(ctx, cfg)
	r := &reporter{}
	cfg.reporter = r
	cfg.pool = membership.New(ctx, r, cfg, cfg.dial)
	d := daemon.New(ctx, cfg)
	r.daemon = d
	cluster := &cluster{
		pool:   cfg.pool,
		daemon: d,
	}
	cfg.controller = &capi{
		c:    cluster,
		pool: cfg.pool,
		p:    d,
	}
	srv, _ := raftrpc.NewServer(ctx, cfg)
	return d, srv
}
