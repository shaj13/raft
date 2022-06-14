package membership

import (
	"testing"
	"time"

	"github.com/franklee0817/raft/internal/raftpb"
	"github.com/stretchr/testify/require"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

func TestRemoved(t *testing.T) {
	addr := ":50051"
	id := uint64(1)
	r := removed{
		raw: raftpb.Member{
			ID:      id,
			Address: addr,
		},
	}

	require.Equal(t, id, r.ID())
	require.Equal(t, addr, r.Address())
	require.False(t, r.IsActive())
	require.Equal(t, time.Time{}, r.ActiveSince())
	require.Equal(t, raftpb.RemovedMember, r.Type())
	require.Equal(t, r.Send(etcdraftpb.Message{}), errRemovedMember)
	require.Equal(t, r.Update(raftpb.Member{}), errRemovedMember)
	require.Equal(t, addr, r.Address())
	require.Equal(t, addr, r.Raw().Address)
}
