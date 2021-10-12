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
	return f.create(m.ID, m.Address, m.Type)
}

func (f *factory) To(m Member) raftpb.Member {
	return raftpb.Member{
		ID:      m.ID(),
		Address: m.Address(),
		Type:    m.Type(),
	}
}

func (f *factory) Cast(m Member, t raftpb.MemberType) (Member, bool, error) {
	return f.create(m.ID(), m.Address(), t)
}

func (f *factory) create(id uint64, addr string, t raftpb.MemberType) (Member, bool, error) {
	c, ok := f.constructors[t]
	if !ok {
		return nil, false, nil
	}

	mem, err := c(f.ctx, f.cfg, id, addr)
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

func newLocal(_ context.Context, cfg Config, id uint64, addr string) (Member, error) {
	return &local{
		id:     id,
		r:      cfg.Reporter(),
		addr:   addr,
		active: time.Now(),
	}, nil
}

func newRemoved(_ context.Context, _ Config, id uint64, addr string) (Member, error) {
	return removed{
		id:   id,
		addr: addr,
	}, nil
}

func newRemote(ctx context.Context, cfg Config, id uint64, addr string) (Member, error) {
	rpc, err := cfg.Dial()(ctx, addr)
	if err != nil {
		return nil, err
	}

	mem := new(remote)
	mem.ctx, mem.cancel = context.WithCancel(ctx)
	mem.rpc = rpc
	mem.id = id
	mem.addr = addr
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
