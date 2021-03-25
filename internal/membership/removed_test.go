package membership

import (
	"testing"
	"time"

	"github.com/shaj13/raftkit/api"
	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func TestRemoved(t *testing.T) {
	addr := ":50051"
	id := uint64(1)
	r := removed{
		id:   id,
		addr: addr,
	}

	assert.Equal(t, id, r.ID())
	assert.Equal(t, addr, r.Address())
	assert.False(t, r.IsActive())
	assert.Equal(t, time.Time{}, r.Since())
	assert.Equal(t, api.RemovedMember, r.Type())
	assert.Equal(t, r.Send(raftpb.Message{}), ErrRemovedMember)
	assert.Equal(t, r.Update(""), ErrRemovedMember)
	assert.Equal(t, addr, r.Address())
}
