package membership

import (
	"context"
	"testing"

	"github.com/shaj13/raftkit/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestFactory(t *testing.T) {
	m := api.Member{
		Address: ":5052",
		ID:      123,
		Type:    api.LocalMember,
	}

	f := newFactory(
		context.Background(),
		nil,
		nil,
		nil,
	)

	mem, ok, err := f.From(m)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, m.Address, mem.Address())
	assert.Equal(t, m.ID, mem.ID())
	assert.Equal(t, m.Type, mem.Type())

	mem, ok, err = f.Cast(mem, api.RemovedMember)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, m.Address, mem.Address())
	assert.Equal(t, m.ID, mem.ID())
	assert.Equal(t, api.RemovedMember, mem.Type())

	mm := f.To(mem)
	m.Type = api.RemovedMember
	assert.Equal(t, m, mm)
}

func TestNewRemote(t *testing.T) {
	tr := &mockRPC{mock.Mock{}}
	tr.On("Close").Return(nil)
	dial := mockDial(tr, nil)
	m, _ := newRemote(context.Background(), nil, testConfig, dial, 0, "")
	m.Close()
	tr.AssertCalled(t, "Close")
}
