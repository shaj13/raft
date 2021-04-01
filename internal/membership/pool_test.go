package membership

import (
	"context"
	"testing"

	"github.com/shaj13/raftkit/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestPoolNextID(t *testing.T) {
	p := newTestPool()
	rec := make(map[uint64]struct{})

	for i := 0; i < 10; i++ {
		id := p.NextID()
		_, ok := rec[id]
		if ok {
			t.Error("pool generate same id more than once")
		}
		p.membs[id] = nil
		rec[id] = struct{}{}
	}
}

func TestPoolUpdate(t *testing.T) {
	m := &api.Member{ID: 1, Type: api.LocalMember}
	p := newTestPool()
	p.Add(*m)
	m.Address = "5050"
	p.Update(*m)
	mem, _ := p.Get(m.ID)
	assert.Equal(t, m.Address, mem.Address())
}

func TestPoolRemoveErr(t *testing.T) {
	p := newTestPool()
	err := p.Remove(api.Member{})
	assert.Contains(t, err.Error(), "not found")
}

func TestPoolRestore(t *testing.T) {
	p := newTestPool()
	ids := make(map[uint64]struct{})
	pool := &api.Pool{
		Members: []api.Member{},
	}

	for i := 0; i < 5; i++ {
		m := api.Member{ID: uint64(i), Type: api.LocalMember}
		ids[uint64(i)] = struct{}{}
		pool.Members = append(pool.Members, m)
	}

	p.Restore(*pool)

	for id := range ids {
		_, ok := p.Get(id)
		if !ok {
			t.Errorf("pool have'nt add member %x during the restore", id)
		}
	}
}

func TestPoolSnapshot(t *testing.T) {
	p := newTestPool()
	p.Add(api.Member{ID: p.NextID(), Type: api.LocalMember})
	membs := p.Snapshot()
	assert.Equal(t, len(p.Members()), len(membs))
}

func TestPoolRemove(t *testing.T) {
	m := &api.Member{ID: 1, Type: api.LocalMember}

	r := &mockReporter{mock.Mock{}}
	r.On("ReportShutdown", m.ID).Return()

	p := newTestPool()
	p.factory.rep = r
	p.Add(*m)

	// run twice to check removing
	// a removed member does not return error
	for i := 0; i < 2; i++ {
		m.Type = api.RemovedMember
		err := p.Remove(*m)
		mem, _ := p.Get(m.ID)
		assert.NoError(t, err)
		assert.Equal(t, api.RemovedMember, mem.Type())
	}

}

func newTestPool() *Pool {
	p := New(context.Background(), testConfig)
	return p
}
