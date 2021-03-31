package raft

import (
	"context"
	"fmt"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/daemon"
	"github.com/shaj13/raftkit/internal/membership"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

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

type capi struct {
	c    *cluster
	p    daemon.Daemon
	pool *membership.Pool
}

func (a *capi) Join(ctx context.Context, m *api.Member) (uint64, []api.Member, error) {
	var (
		memb Member
		err  error
	)

	if mm, _ := a.c.GetMemebr(m.ID); mm == nil {
		memb, err = a.c.AddMember(context.Background(), m.Address)
	} else {
		err = a.c.UpdateMember(context.Background(), m.ID, m.Address)
		memb, _ = a.c.GetMemebr(m.ID)
	}

	if err != nil {
		return 0, []api.Member{}, err
	}

	pool := a.pool.Snapshot()

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

	fmt.Printf("member added id, %x\n", memb.ID())

	return memb.ID(), pool, nil
}

func (a *capi) Push(ctx context.Context, m raftpb.Message) error {
	return a.p.Push(m)
}
