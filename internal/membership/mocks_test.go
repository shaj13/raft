package membership

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

var testConfig = mockConfig{}

func mockDial(m *mockTransport, err error) Dial {
	return func(ctx context.Context, addr string) (Transport, error) {
		return m, err
	}
}

type mockTransport struct {
	mock.Mock
}

func (m *mockTransport) RoundTrip(ctx context.Context, msg raftpb.Message) error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockTransport) Close() error {
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

type mockConfig struct{}

func (m mockConfig) StreamTimeout() time.Duration {
	return time.Second
}

func (m mockConfig) DrainTimeout() time.Duration {
	return time.Second
}
