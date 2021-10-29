package membership

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/shaj13/raftkit/internal/log"
	"github.com/shaj13/raftkit/internal/raftpb"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func New(ctx context.Context, cfg Config) Pool {
	return &pool{
		ctx:   ctx,
		cfg:   cfg,
		membs: make(map[uint64]Member),
		matcher: func(m raftpb.Member) raftpb.MemberType {
			return m.Type
		},
	}
}

type pool struct {
	ctx     context.Context
	cfg     Config
	matcher func(m raftpb.Member) raftpb.MemberType
	mu      sync.Mutex // protects the membs
	membs   map[uint64]Member
}

func (p *pool) RegisterTypeMatcher(fn func(m raftpb.Member) raftpb.MemberType) {
	p.matcher = fn
}

func (p *pool) NextID() uint64 {
	var id uint64
	for {
		id = uint64(rand.Int63()) + 1
		if _, ok := p.Get(id); !ok {
			break
		}
	}
	return id
}

func (p *pool) Members() []Member {
	membs := []Member{}
	p.mu.Lock()
	for _, m := range p.membs {
		membs = append(membs, m)
	}
	p.mu.Unlock()
	return membs
}

func (p *pool) Get(id uint64) (Member, bool) {
	p.mu.Lock()
	m, ok := p.membs[id]
	p.mu.Unlock()
	return m, ok
}

func (p *pool) Add(m raftpb.Member) error {
	_, ok := p.Get(m.ID)
	if ok {
		return p.Update(m)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	mem, err := p.newMember(m)
	if err != nil {
		return err
	}

	p.membs[m.ID] = mem
	return nil
}

func (p *pool) Update(m raftpb.Member) error {
	mem, ok := p.Get(m.ID)
	if !ok {
		return p.Add(m)
	}

	return mem.Update(m)
}

func (p *pool) Remove(m raftpb.Member) error {
	mem, ok := p.Get(m.ID)
	if !ok {
		return fmt.Errorf("raft/membership: member %x not found", m.ID)
	}

	if mem.Type() == m.Type {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if err := mem.Close(); err != nil {
		log.Warnf("raft.membership: closing member %x: %v", m.ID, err)
	}

	mem, err := p.newMember(m)
	if err != nil {
		return err
	}

	p.membs[m.ID] = mem
	return nil
}

func (p *pool) Snapshot() []raftpb.Member {
	p.mu.Lock()
	defer p.mu.Unlock()
	membs := []raftpb.Member{}
	for _, mem := range p.membs {
		membs = append(membs, mem.Raw())
	}
	return membs
}

func (p *pool) Restore(pool raftpb.Pool) {
	for _, m := range pool.Members {
		if err := p.Add(m); err != nil {
			log.Errorf("raft.membership: adding member %x: %v", m.ID, err)
		}
	}
}

func (p *pool) newMember(m raftpb.Member) (Member, error) {
	switch p.matcher(m) {
	case raftpb.VoterMember, raftpb.LearnerMember, raftpb.StagingMember:
		return newRemote(p.ctx, p.cfg, m)
	case raftpb.RemovedMember:
		return newRemoved(p.ctx, p.cfg, m)
	case raftpb.LocalMember:
		return newLocal(p.ctx, p.cfg, m)
	default:
		return nil, fmt.Errorf("raft/membership: unknown member type %s", m.Type)
	}
}
