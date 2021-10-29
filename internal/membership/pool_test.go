package membership

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/transport"
	"github.com/stretchr/testify/require"
)

func TestPoolNewMember(t *testing.T) {
	m := raftpb.Member{
		Address: ":5052",
		ID:      123,
		Local:   true,
	}

	p := &pool{
		ctx: context.TODO(),
		cfg: testConfig(t),
	}

	mem, err := p.newMember(m)
	require.NoError(t, err)
	require.Equal(t, m.Address, mem.Address())
	require.Equal(t, m.ID, mem.ID())
	require.Equal(t, m.Type, mem.Type())
}

func TestPoolNextID(t *testing.T) {
	p := New(context.TODO(), testConfig(t)).(*pool)
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
	m := &raftpb.Member{ID: 1, Local: true}
	p := New(context.TODO(), testConfig(t))
	p.Add(*m)
	m.Address = "5050"
	p.Update(*m)
	mem, _ := p.Get(m.ID)
	require.Equal(t, m.Address, mem.Address())
}

func TestPoolRemoveErr(t *testing.T) {
	p := New(context.TODO(), testConfig(t))
	err := p.Remove(raftpb.Member{})
	require.Contains(t, err.Error(), "not found")
}

func TestPoolRestore(t *testing.T) {
	p := New(context.TODO(), testConfig(t))
	ids := make(map[uint64]struct{})
	pool := &raftpb.Pool{
		Members: []raftpb.Member{},
	}

	for i := 0; i < 5; i++ {
		m := raftpb.Member{ID: uint64(i), Local: true}
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
	p := New(context.TODO(), testConfig(t))
	p.Add(raftpb.Member{ID: p.NextID(), Local: true})
	membs := p.Snapshot()
	require.Equal(t, len(p.Members()), len(membs))
}

func TestPoolRemove(t *testing.T) {
	m := &raftpb.Member{ID: 1, Local: true}
	ctrl := gomock.NewController(t)
	cfg := NewMockConfig(ctrl)
	r := NewMockReporter(ctrl)
	r.EXPECT().ReportShutdown(gomock.Eq(m.ID)).Return()
	cfg.EXPECT().Reporter().Return(r).MaxTimes(2)

	p := New(context.TODO(), cfg)
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

func testConfig(t *testing.T) *MockConfig {
	ctrl := gomock.NewController(t)
	cfg := NewMockConfig(ctrl)
	cfg.EXPECT().Reporter().Return(nil).AnyTimes()
	cfg.EXPECT().DrainTimeout().Return(time.Duration(-1)).AnyTimes()
	cfg.EXPECT().StreamTimeout().Return(time.Duration(-1)).AnyTimes()
	return cfg
}

func mockDial(c transport.Client, err error) transport.Dial {
	return func(ctx context.Context, addr string) (transport.Client, error) {
		return c, err
	}
}
