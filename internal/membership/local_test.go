package membership

import (
	"testing"
	"time"

	"github.com/franklee0817/raft/internal/raftpb"
	"github.com/stretchr/testify/require"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

func TestLocal(t *testing.T) {
	addr := ":8080"
	id := uint64(1)
	raw := raftpb.Member{ID: 2}
	l := local{}
	l.raw.Store(raftpb.Member{
		ID:      id,
		Address: addr,
		Type:    raftpb.LearnerMember,
	})
	require.Equal(t, l.ID(), id)
	require.Equal(t, l.Address(), addr)
	require.False(t, l.IsActive())
	require.Equal(t, l.ActiveSince(), time.Time{})
	require.Equal(t, l.Type(), raftpb.LearnerMember)
	require.NoError(t, l.Update(raw))
	require.Empty(t, l.Address())
	require.Panics(t, func() { l.Send(etcdraftpb.Message{}) })
	require.Equal(t, raw, l.Raw())
}
