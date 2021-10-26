package membership

import (
	"context"
	"fmt"
	"time"

	"github.com/shaj13/raftkit/internal/raftpb"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

type factory struct {
	ctx context.Context
	cfg Config
}

func (f *factory) Create(m raftpb.Member) (Member, error) {
	switch m.Type {
	case raftpb.LocalMember:
		return newLocal(f.ctx, f.cfg, m)
	case raftpb.RemoteMember:
		return newRemote(f.ctx, f.cfg, m)
	case raftpb.RemovedMember:
		return newRemoved(f.ctx, f.cfg, m)
	default:
		return nil, fmt.Errorf("raft/membership: unknown member type %s", m.Type)
	}
}

func newFactory(ctx context.Context, cfg Config) *factory {
	f := new(factory)
	f.ctx = ctx
	f.cfg = cfg
	return f
}

func newLocal(_ context.Context, cfg Config, m raftpb.Member) (Member, error) {
	return &local{
		r:      cfg.Reporter(),
		active: time.Now(),
		raw:    &m,
	}, nil
}

func newRemoved(_ context.Context, _ Config, m raftpb.Member) (Member, error) {
	return removed{
		raw: m,
	}, nil
}

func newRemote(ctx context.Context, cfg Config, m raftpb.Member) (Member, error) {
	rpc, err := cfg.Dial()(ctx, m.Address)
	if err != nil {
		return nil, err
	}

	mem := new(remote)
	mem.ctx, mem.cancel = context.WithCancel(ctx)
	mem.rc = rpc
	mem.raw = &m
	mem.cfg = cfg
	mem.r = cfg.Reporter()
	mem.dial = cfg.Dial()
	mem.msgc = make(chan etcdraftpb.Message, 4096)
	mem.done = make(chan struct{})
	// assuming member is active.
	mem.active = true
	mem.activeSince = time.Now()
	go mem.run()

	return mem, nil
}
