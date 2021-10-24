package membership

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	transportmock "github.com/shaj13/raftkit/internal/mocks/transport"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/transport"
	"github.com/stretchr/testify/require"
)

func TestFactory(t *testing.T) {
	ctrl := gomock.NewController(t)
	cfg := NewMockConfig(ctrl)
	cfg.EXPECT().Reporter().Return(nil)

	m := raftpb.Member{
		Address: ":5052",
		ID:      123,
		Type:    raftpb.LocalMember,
	}

	f := newFactory(context.TODO(), cfg)

	mem, ok, err := f.From(m)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, m.Address, mem.Address())
	require.Equal(t, m.ID, mem.ID())
	require.Equal(t, m.Type, mem.Type())

	mem, ok, err = f.Cast(mem, raftpb.RemovedMember)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, m.Address, mem.Address())
	require.Equal(t, m.ID, mem.ID())
	require.Equal(t, raftpb.RemovedMember, mem.Type())

	mm := f.To(mem)
	m.Type = raftpb.RemovedMember
	require.Equal(t, m, mm)
}

func TestNewRemote(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := transportmock.NewMockClient(ctrl)
	cfg := NewMockConfig(ctrl)
	client.EXPECT().Close().Return(nil)
	dial := mockDial(client, nil)
	cfg.EXPECT().Dial().Return(dial).MaxTimes(2)
	cfg.EXPECT().Reporter().Return(nil)
	cfg.EXPECT().DrainTimeout().Return(time.Duration(-1))

	m, err := newRemote(context.Background(), cfg, 0, "")
	require.NoError(t, err)
	require.NotNil(t, m)

	err = m.Close()
	require.NoError(t, err)
}

func mockDial(c transport.Client, err error) transport.Dial {
	return func(ctx context.Context, addr string) (transport.Client, error) {
		return c, err
	}
}
