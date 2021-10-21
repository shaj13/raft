package membership

import (
	"context"
	"testing"

	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPoolNextID(t *testing.T) {
	p := New(context.Background(), testConfig).(*pool)
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
	m := &raftpb.Member{ID: 1, Type: raftpb.LocalMember}
	p := New(context.Background(), testConfig)
	p.Add(*m)
	m.Address = "5050"
	p.Update(*m)
	mem, _ := p.Get(m.ID)
	require.Equal(t, m.Address, mem.Address())
}

func TestPoolRemoveErr(t *testing.T) {
	p := New(context.Background(), testConfig)
	err := p.Remove(raftpb.Member{})
	require.Contains(t, err.Error(), "not found")
}

func TestPoolRestore(t *testing.T) {
	p := New(context.Background(), testConfig)
	ids := make(map[uint64]struct{})
	pool := &raftpb.Pool{
		Members: []raftpb.Member{},
	}

	for i := 0; i < 5; i++ {
		m := raftpb.Member{ID: uint64(i), Type: raftpb.LocalMember}
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
	p := New(context.Background(), testConfig)
	p.Add(raftpb.Member{ID: p.NextID(), Type: raftpb.LocalMember})
	membs := p.Snapshot()
	require.Equal(t, len(p.Members()), len(membs))
}

func TestPoolRemove(t *testing.T) {
	m := &raftpb.Member{ID: 1, Type: raftpb.LocalMember}

	r := &mockReporter{mock.Mock{}}
	r.On("ReportShutdown", m.ID).Return()

	p := New(context.Background(), mockConfig{r: r})
	p.Add(*m)

	// run twice to check removing
	// a removed member does not return error
	for i := 0; i < 2; i++ {
		m.Type = raftpb.RemovedMember
		err := p.Remove(*m)
		mem, _ := p.Get(m.ID)
		require.NoError(t, err)
		require.Equal(t, raftpb.RemovedMember, mem.Type())
	}

}
