package membership

import (
	"context"
	"time"

	"github.com/shaj13/raftkit/internal/raftpb"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

type factory struct {
	ctx          context.Context
	cfg          Config
	constructors map[raftpb.MemberType]constructor
}

func (f *factory) From(m raftpb.Member) (Member, bool, error) {
	return f.create(m)
}

func (f *factory) To(m Member) raftpb.Member {
	return raftpb.Member{
		ID:      m.ID(),
		Address: m.Address(),
		Type:    m.Type(),
	}
}

func (f *factory) Cast(m Member, t raftpb.MemberType) (Member, bool, error) {
	temp := raftpb.Member{ // TODO: remove me when we have support raw.
		ID:      m.ID(),
		Address: m.Address(),
		Type:    t,
	}
	return f.create(temp)
}

func (f *factory) create(m raftpb.Member) (Member, bool, error) {
	c, ok := f.constructors[m.Type]
	if !ok {
		return nil, false, nil
	}

	mem, err := c(f.ctx, f.cfg, m)
	return mem, true, err
}

func newFactory(ctx context.Context, cfg Config) *factory {
	f := new(factory)
	f.ctx = ctx
	f.cfg = cfg
	f.constructors = map[raftpb.MemberType]constructor{
		raftpb.RemoteMember:  newRemote,
		raftpb.RemovedMember: newRemoved,
		raftpb.LocalMember:   newLocal,
	}
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
