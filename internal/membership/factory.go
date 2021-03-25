package membership

import (
	"context"

	"github.com/shaj13/raftkit/api"
)

type factory struct {
	ctx          context.Context
	cancel       context.CancelFunc
	cfg          config
	r            reporter
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

	mem, err := c(f.ctx, f.r, f.cfg, id, addr)
	return mem, true, err
}
