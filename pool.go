package raft

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/shaj13/raftkit/api"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type pool struct {
	factory *factory
	cfg     *config
	mu      sync.Mutex // protects membs
	membs   map[uint64]Member
}

func (p *pool) nextID() uint64 {
	var id uint64
	for {
		id = uint64(rand.Int63()) + 1
		if _, ok := p.get(id); !ok {
			break
		}
	}
	return id
}

func (p *pool) members() []Member {
	membs := []Member{}
	p.mu.Lock()
	for _, m := range p.membs {
		membs = append(membs, m)
	}
	p.mu.Unlock()
	return membs
}

func (p *pool) get(id uint64) (Member, bool) {
	p.mu.Lock()
	m, ok := p.membs[id]
	p.mu.Unlock()
	return m, ok
}

func (p *pool) add(m api.Member) error {
	mem, ok := p.get(m.ID)
	if ok && mem.Address() == m.Address {
		// member already exist
		return nil
	}

	if ok {
		return p.update(m)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	mem, ok, err := p.factory.from(m)
	if !ok || err != nil {
		return err
	}

	p.membs[m.ID] = mem
	return nil
}

func (p *pool) update(m api.Member) error {
	mem, ok := p.get(m.ID)
	if ok && mem.Address() == m.Address {
		// member already exist
		return nil
	}

	if !ok {
		return p.add(m)
	}

	return mem.update(m.Address)
}

func (p *pool) remove(m api.Member) error {
	mem, ok := p.get(m.ID)
	if !ok {
		return fmt.Errorf("raft: member %x not found", m.ID)
	}

	if mem.kind() == m.Type {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	mem.close()

	mem, ok, err := p.factory.cast(mem, m.Type)
	if !ok || err != nil {
		return err
	}

	p.membs[m.ID] = mem
	return nil
}

func (p *pool) snapshot() []api.Member {
	p.mu.Lock()
	defer p.mu.Unlock()
	membs := []api.Member{}
	for _, mem := range p.membs {
		m := p.factory.to(mem)
		membs = append(membs, m)
	}
	return membs
}

func (p *pool) recover(membs []api.Member) {
	for _, m := range membs {
		if err := p.add(m); err != nil {
			p.cfg.logger.Errorf("raft: Failed to add member %x, Err: %s", m.ID, err)
		}
	}
}
