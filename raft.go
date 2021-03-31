package raft

import (
	"context"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/daemon"
	"github.com/shaj13/raftkit/internal/membership"
	raftrpc "github.com/shaj13/raftkit/internal/net/grpc"
	"github.com/shaj13/raftkit/internal/storage/disk"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func New(ctx context.Context) (Cluster, interface{}) {
	cfg := defaultConfig()
	cfg.controller = new(controller)
	cfg.reporter = new(reporter)
	cfg.storage = disk.New(ctx, cfg)
	cfg.dial = raftrpc.Dialer(ctx, cfg)
	cfg.pool = membership.New(ctx, cfg.reporter, cfg, cfg.dial)

	daemon := daemon.New(ctx, cfg)

	cluster := new(cluster)
	cluster.pool = cfg.pool
	cluster.daemon = daemon

	cfg.controller.(*controller).cluster = cluster
	cfg.controller.(*controller).daemon = daemon
	cfg.controller.(*controller).pool = cfg.pool

	cfg.reporter.(*reporter).daemon = daemon

	srv, _ := raftrpc.NewServer(ctx, cfg)

	return cluster, srv
}

type controller struct {
	cluster *cluster
	daemon  daemon.Daemon
	pool    *membership.Pool
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

type reporter struct {
	daemon daemon.Daemon
}

func (r *reporter) ReportUnreachable(id uint64) {
	r.daemon.Notify(daemon.Unreachable, id)
}

func (r *reporter) ReportShutdown(id uint64) {
	r.daemon.Notify(daemon.Shutdown, id)
}

func (r *reporter) ReportSnapshot(id uint64, status raft.SnapshotStatus) {
	if status == raft.SnapshotFailure {
		r.daemon.Notify(daemon.SnapshotFailure, id)
		return
	}

	r.daemon.Notify(daemon.SnapshotOK, id)
}
