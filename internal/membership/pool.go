package membership

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/log"
	"github.com/shaj13/raftkit/internal/net"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func New(ctx context.Context, rep Reporter, cfg Config, dial net.Dial) *Pool {
	return &Pool{
		factory: newFactory(ctx, rep, cfg, dial),
		membs:   make(map[uint64]Member),
	}
}

type Pool struct {
	factory *factory
	mu      sync.Mutex // protects membs
	membs   map[uint64]Member
}

func (p *Pool) NextID() uint64 {
	var id uint64
	for {
		id = uint64(rand.Int63()) + 1
		if _, ok := p.Get(id); !ok {
			break
		}
	}
	return id
}

func (p *Pool) Members() []Member {
	membs := []Member{}
	p.mu.Lock()
	for _, m := range p.membs {
		membs = append(membs, m)
	}
	p.mu.Unlock()
	return membs
}

func (p *Pool) Get(id uint64) (Member, bool) {
	p.mu.Lock()
	m, ok := p.membs[id]
	p.mu.Unlock()
	return m, ok
}

func (p *Pool) Add(m api.Member) error {
	mem, ok := p.Get(m.ID)
	if ok && mem.Address() == m.Address {
		// member already exist
		return nil
	}

	if ok {
		return p.Update(m)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	mem, ok, err := p.factory.From(m)
	if !ok || err != nil {
		return err
	}

	p.membs[m.ID] = mem
	return nil
}

func (p *Pool) Update(m api.Member) error {
	mem, ok := p.Get(m.ID)
	if ok && mem.Address() == m.Address {
		// member already exist
		return nil
	}

	if !ok {
		return p.Add(m)
	}

	return mem.Update(m.Address)
}

func (p *Pool) Remove(m api.Member) error {
	mem, ok := p.Get(m.ID)
	if !ok {
		return fmt.Errorf("raft/membership: member %x not found", m.ID)
	}

	if mem.Type() == m.Type {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	mem.Close()

	mem, ok, err := p.factory.Cast(mem, m.Type)
	if !ok || err != nil {
		return err
	}

	p.membs[m.ID] = mem
	return nil
}

func (p *Pool) Snapshot() []api.Member {
	p.mu.Lock()
	defer p.mu.Unlock()
	membs := []api.Member{}
	for _, mem := range p.membs {
		m := p.factory.To(mem)
		membs = append(membs, m)
	}
	return membs
}

func (p *Pool) Restore(pool api.Pool) {
	for _, m := range pool.Members {
		if err := p.Add(m); err != nil {
			log.Errorf("raft/membership: Failed to add member %x, Err: %s", m.ID, err)
		}
	}
}
