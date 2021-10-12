package membership

import (
	"context"
	"time"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/rpc"
	"github.com/stretchr/testify/mock"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

var testConfig = mockConfig{}

func mockDial(m *mockRPC, err error) rpc.Dial {
	return func(ctx context.Context, addr string) (rpc.Client, error) {
		return m, err
	}
}

type mockRPC struct {
	mock.Mock
}

func (m *mockRPC) Message(context.Context, raftpb.Message) error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockRPC) Join(context.Context, api.Member) (uint64, []api.Member, error) {
	args := m.Called()
	return 0, nil, args.Error(2)
}

func (m *mockRPC) Close() error {
	args := m.Called()
	return args.Error(0)
}

type mockReporter struct {
	mock.Mock
}

func (m *mockReporter) ReportUnreachable(id uint64) {
	m.Called(id)
}
func (m *mockReporter) ReportShutdown(id uint64) {
	m.Called(id)
}
func (m *mockReporter) ReportSnapshot(id uint64, status raft.SnapshotStatus) {
	m.Called(id, status)
}

type mockConfig struct {
	d rpc.Dial
	r *mockReporter
}

func (m mockConfig) StreamTimeout() time.Duration {
	return time.Second
}

func (m mockConfig) DrainTimeout() time.Duration {
	return time.Second
}

func (m mockConfig) Reporter() Reporter {
	return m.r
}

func (m mockConfig) Dial() rpc.Dial {
	return m.d
}
