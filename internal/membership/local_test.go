package membership

import (
	"testing"
	"time"

	"github.com/shaj13/raftkit/api"
	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func TestLocal(t *testing.T) {
	addr := ":50051"
	id := uint64(1)
	l := Local{
		id:   id,
		addr: addr,
	}

	assert.Equal(t, l.ID(), id)
	assert.Equal(t, l.Address(), addr)
	assert.False(t, l.IsActive())
	assert.Equal(t, l.Since(), time.Time{})
	assert.Equal(t, l.Type(), api.SelfMember)
	assert.NoError(t, l.Send(raftpb.Message{}))
	assert.NoError(t, l.Update(""))
	assert.Empty(t, l.Address())
}
