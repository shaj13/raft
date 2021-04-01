package membership

import (
	"context"
	"time"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/net"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

type factory struct {
	ctx          context.Context
	cfg          Config
	rep          Reporter
	dial         net.Dial
	constructors map[api.MemberType]constructor
}

func (f *factory) From(m api.Member) (Member, bool, error) {
	return f.create(m.ID, m.Address, m.Type)
}

func (f *factory) To(m Member) api.Member {
	return api.Member{
		ID:      m.ID(),
		Address: m.Address(),
		Type:    m.Type(),
	}
}

func (f *factory) Cast(m Member, t api.MemberType) (Member, bool, error) {
	return f.create(m.ID(), m.Address(), t)
}

func (f *factory) create(id uint64, addr string, t api.MemberType) (Member, bool, error) {
	c, ok := f.constructors[t]
	if !ok {
		return nil, false, nil
	}

	mem, err := c(f.ctx, f.rep, f.cfg, f.dial, id, addr)
	return mem, true, err
}

func newFactory(ctx context.Context, cfg Config) *factory {
	f := new(factory)
	f.ctx = ctx
	f.cfg = cfg
	f.dial = cfg.Dial()
	f.rep = cfg.Reporter()
	f.constructors = map[api.MemberType]constructor{
		api.RemoteMember:  newRemote,
		api.RemovedMember: newRemoved,
		api.LocalMember:   newLocal,
	}
	return f
}

func newLocal(_ context.Context, r Reporter, _ Config, _ net.Dial, id uint64, addr string) (Member, error) {
	return &local{
		id:     id,
		r:      r,
		addr:   addr,
		active: time.Now(),
	}, nil
}

func newRemoved(_ context.Context, r Reporter, _ Config, _ net.Dial, id uint64, addr string) (Member, error) {
	return removed{
		id:   id,
		addr: addr,
	}, nil
}

func newRemote(ctx context.Context, r Reporter, cfg Config, dial net.Dial, id uint64, addr string) (Member, error) {
	rpc, err := dial(ctx, addr)
	if err != nil {
		return nil, err
	}

	mem := new(remote)
	mem.ctx, mem.cancel = context.WithCancel(ctx)
	mem.rpc = rpc
	mem.id = id
	mem.addr = addr
	mem.cfg = cfg
	mem.r = r
	mem.dial = dial
	mem.msgc = make(chan raftpb.Message, 4096)
	mem.done = make(chan struct{})
	// assuming member is active.
	mem.active = true
	mem.activeSince = time.Now()
	go mem.run()

	return mem, nil
}
