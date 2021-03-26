package membership

import (
	"context"
	"time"

	"github.com/shaj13/raftkit/api"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

type factory struct {
	ctx          context.Context
	cfg          config
	rep          reporter
	dial         Dial
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

func newFactory(ctx context.Context, rep reporter, cfg config, dial Dial) *factory {
	f := new(factory)
	f.ctx = ctx
	f.cfg = cfg
	f.dial = dial
	f.rep = rep
	f.constructors = map[api.MemberType]constructor{
		api.RemoteMember:  newRemote,
		api.RemovedMember: newRemoved,
		api.LocalMember:   newLocal,
	}
	return f
}

func newLocal(_ context.Context, r reporter, _ config, _ Dial, id uint64, addr string) (Member, error) {
	return &local{
		id:     id,
		r:      r,
		addr:   addr,
		active: time.Now(),
	}, nil
}

func newRemoved(_ context.Context, r reporter, _ config, _ Dial, id uint64, addr string) (Member, error) {
	return removed{
		id:   id,
		addr: addr,
	}, nil
}

func newRemote(ctx context.Context, r reporter, cfg config, dial Dial, id uint64, addr string) (Member, error) {
	tr, err := dial(ctx, addr)
	if err != nil {
		return nil, err
	}

	mem := new(remote)
	mem.ctx, mem.cancel = context.WithCancel(ctx)
	mem.tr = tr
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
