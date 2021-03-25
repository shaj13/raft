package membership

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shaj13/raftkit/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func TestRemote(t *testing.T) {
	id := uint64(1)
	addr := ":50051"
	r := remote{
		id:   id,
		addr: addr,
	}

	assert.Equal(t, r.ID(), id)
	assert.Equal(t, r.Address(), addr)
	assert.False(t, r.IsActive())
	assert.Equal(t, r.Since(), time.Time{})
	assert.Equal(t, r.Type(), api.RemoteMember)
	assert.Nil(t, r.transport())
}

func TestRemoteSetStatus(t *testing.T) {
	table := []struct {
		name         string
		in           bool
		currentstate bool
	}{
		{
			name:         "it set status to active if member was in active",
			in:           true,
			currentstate: false,
		},
		{
			name:         "it set status to in-active if member was active",
			in:           false,
			currentstate: true,
		},
		{
			name:         "it keep status as is if member was active",
			in:           true,
			currentstate: true,
		},
		{
			name:         "it keep status as is if member was in-active",
			in:           false,
			currentstate: false,
		},
	}

	for _, tt := range table {
		r := new(remote)
		r.active = tt.currentstate
		r.setStatus(tt.in)
		assert.Equal(t, tt.in, r.IsActive())
	}
}

func TestRemoteUpdate(t *testing.T) {
	err := fmt.Errorf("TestRemoteUpdate dial error")
	addr := ":5050"
	uaddr := ":5051"

	m := &mockTransport{mock.Mock{}}
	m.On("Close").Return(nil)

	r := new(remote)
	r.addr = addr
	r.tr = m
	r.dial = mockDial(nil, err)

	// Round #1 it does not update addr if are the same
	got := r.Update(addr)
	assert.NoError(t, got)
	assert.Equal(t, addr, r.addr)

	// Round #2 it return error whn dial return error
	got = r.Update(uaddr)
	assert.Equal(t, err, got)
	assert.Equal(t, addr, r.addr)

	// Round #3 it update addr and close old tr
	r.dial = mockDial(m, nil)
	got = r.Update(uaddr)
	assert.NoError(t, got)
	assert.Equal(t, uaddr, r.addr)
	m.AssertCalled(t, "Close")
}

func TestRemoteStream(t *testing.T) {
	m := &mockTransport{mock.Mock{}}
	m.On("RoundTrip").Return(nil)
	r := new(remote)
	r.tr = m
	_ = r.stream(context.Background(), raftpb.Message{})
	m.AssertCalled(t, "RoundTrip")
}

func TestRemoteReport(t *testing.T) {
	id := uint64(1)
	err := fmt.Errorf("TestRemoteReport error")

	table := []struct {
		name     string
		msg      raftpb.Message
		err      error
		called   string
		callargs []interface{}
	}{
		{
			name:     "it call ReportSnapshot with SnapshotFinish status",
			msg:      raftpb.Message{Type: raftpb.MsgSnap},
			called:   "ReportSnapshot",
			callargs: []interface{}{id, raft.SnapshotFinish},
		},
		{
			name:     "it call ReportSnapshot with SnapshotFailure status",
			msg:      raftpb.Message{Type: raftpb.MsgSnap},
			err:      err,
			called:   "ReportSnapshot",
			callargs: []interface{}{id, raft.SnapshotFailure},
		},
		{
			name:     "it call ReportUnreachables",
			msg:      raftpb.Message{},
			err:      err,
			called:   "ReportUnreachable",
			callargs: []interface{}{id},
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			m := &mockReporter{mock.Mock{}}
			m.On(tt.called, tt.callargs...).Return()

			r := new(remote)
			r.r = m
			r.id = id

			r.report(tt.msg, tt.err)
			m.AssertCalled(t, tt.called, tt.callargs...)
		})
	}
}

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
