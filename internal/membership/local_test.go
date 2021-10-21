package membership

import (
	"testing"
	"time"

	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/stretchr/testify/require"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

func TestLocal(t *testing.T) {
	addr := ":50051"
	id := uint64(1)
	l := local{
		id:   id,
		addr: addr,
	}

	require.Equal(t, l.ID(), id)
	require.Equal(t, l.Address(), addr)
	require.False(t, l.IsActive())
	require.Equal(t, l.Since(), time.Time{})
	require.Equal(t, l.Type(), raftpb.LocalMember)
	require.NoError(t, l.Send(etcdraftpb.Message{}))
	require.NoError(t, l.Update(""))
	require.Empty(t, l.Address())
}
