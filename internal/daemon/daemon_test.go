package daemon

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shaj13/raftkit/internal/atomic"
	"github.com/shaj13/raftkit/internal/msgbus"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.etcd.io/etcd/pkg/v3/idutil"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

func TestReportUnreachable(t *testing.T) {
	id := uint64(1)
	method := "ReportUnreachable"
	m := &mockNode{Mock: mock.Mock{}}
	d := daemon{node: m, started: atomic.NewBool()}
	m.On(method, id).Return()

	// round #1 should not call ReportUnreachable when
	// daemon not started
	d.ReportUnreachable(id)
	m.AssertNotCalled(t, method, id)

	// round #2 should call ReportUnreachable when
	// daemon started
	d.started.Set()
	d.ReportUnreachable(id)
	m.AssertCalled(t, method, id)
}

func TestReportSnapshot(t *testing.T) {
	id := uint64(1)
	method := "ReportSnapshot"
	m := &mockNode{Mock: mock.Mock{}}
	d := daemon{node: m, started: atomic.NewBool()}
	m.On(method, id, raft.SnapshotFinish).Return()

	// round #1 should not call ReportSnapshot when
	// daemon not started
	d.ReportSnapshot(id, raft.SnapshotFinish)
	m.AssertNotCalled(t, method, id, raft.SnapshotFinish)

	// round #2 should call ReportSnapshot when
	// daemon started
	d.started.Set()
	d.ReportSnapshot(id, raft.SnapshotFinish)
	m.AssertCalled(t, method, id, raft.SnapshotFinish)
}

func TestReportShutdown(t *testing.T) {
	t.Skip("TODO: add test ReportShutdown ")
}

func TestPush(t *testing.T) {
	t.Skip("fix me")
	d := &daemon{
		msgbus:  msgbus.New(),
		started: atomic.NewBool(),
	}

	// round #1 it return err when daemon not started
	err := d.Push(etcdraftpb.Message{})
	assert.Equal(t, ErrStopped, err)

	// round #2 it return nil err when daemon started
	d.started.Set()
	err = d.Push(etcdraftpb.Message{})
	assert.NoError(t, err)
}

func TestStatus(t *testing.T) {
	method := "Status"
	m := &mockNode{Mock: mock.Mock{}}
	d := &daemon{
		node:    m,
		started: atomic.NewBool(),
	}
	m.On(method).Return(raft.Status{})

	// round #1 it return err when daemon not started
	_, err := d.Status()
	m.AssertNotCalled(t, method)
	assert.Equal(t, ErrStopped, err)

	// round #2 it return nil err when daemon started
	d.started.Set()
	_, err = d.Status()
	m.AssertCalled(t, method)
	assert.NoError(t, err)
}

func TestClose(t *testing.T) {
	t.Skip("TODO: add test Close")
}

func TestProposeReplicate(t *testing.T) {
	method := "Propose"
	data := []byte("data")
	m := &mockNode{Mock: mock.Mock{}}
	d := &daemon{
		idgen:   idutil.NewGenerator(1, time.Now()),
		node:    m,
		started: atomic.NewBool(),
		msgbus:  msgbus.New(),
	}

	// round #1 it return err when daemon not started
	err := d.ProposeReplicate(context.TODO(), data)
	m.AssertNotCalled(t, method)
	assert.Equal(t, ErrStopped, err)

	// round #2 it return err whne node return's err
	expected := errors.New("TestProposeReplicate Error")
	m.On(method, mock.Anything, mock.Anything).Return(expected)
	d.started.Set()
	err = d.ProposeReplicate(context.TODO(), data)
	assert.Equal(t, expected, err)

	// round #3 it return ctx done
	m = &mockNode{}
	d.node = m
	m.On(method, mock.Anything, mock.Anything).Return(nil)
	ctx, cancel := context.WithCancel(context.TODO())
	cancel()
	err = d.ProposeReplicate(ctx, data)
	assert.Contains(t, err.Error(), "canceled")
}

func TestProposeConfChange(t *testing.T) {
	method := "ProposeConfChange"
	// data := []byte("data")
	m := &mockNode{Mock: mock.Mock{}}
	d := &daemon{
		idgen:   idutil.NewGenerator(1, time.Now()),
		node:    m,
		started: atomic.NewBool(),
		msgbus:  msgbus.New(),
	}

	// round #1 it return err when daemon not started
	err := d.ProposeConfChange(context.TODO(), nil, etcdraftpb.ConfChangeAddNode)
	m.AssertNotCalled(t, method)
	assert.Equal(t, ErrStopped, err)

	// round #2 it return err whne node return's err
	expected := errors.New("TestProposeReplicate Error")
	m.On(method, mock.Anything, mock.Anything).Return(expected)
	d.started.Set()
	err = d.ProposeConfChange(context.TODO(), &raftpb.Member{}, etcdraftpb.ConfChangeAddNode)
	assert.Equal(t, expected, err)

	// round #3 it return ctx done
	m = &mockNode{}
	d.node = m
	m.On(method, mock.Anything, mock.Anything).Return(nil)
	ctx, cancel := context.WithCancel(context.TODO())
	cancel()
	err = d.ProposeConfChange(ctx, &raftpb.Member{}, etcdraftpb.ConfChangeAddNode)
	assert.Contains(t, err.Error(), "canceled")
}

type mockNode struct {
	mock.Mock
}

func (m *mockNode) Tick() {
	m.Called()
}

func (m *mockNode) Campaign(_ context.Context) error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockNode) Propose(ctx context.Context, buf []byte) error {
	args := m.Called(ctx, buf)
	return args.Error(0)
}

func (m *mockNode) ProposeConfChange(ctx context.Context, cc etcdraftpb.ConfChangeI) error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockNode) Step(ctx context.Context, msg etcdraftpb.Message) error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockNode) Ready() <-chan raft.Ready {
	return nil
}

func (m *mockNode) Advance() {
	m.Called()
}

func (m *mockNode) ApplyConfChange(cc etcdraftpb.ConfChangeI) *etcdraftpb.ConfState {
	args := m.Called()
	return args.Get(0).(*etcdraftpb.ConfState)
}

func (m *mockNode) TransferLeadership(ctx context.Context, lead, transferee uint64) {
	m.Called()
}

func (m *mockNode) ReadIndex(ctx context.Context, rctx []byte) error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockNode) Status() raft.Status {
	args := m.Called()
	return args.Get(0).(raft.Status)
}

func (m *mockNode) ReportUnreachable(id uint64) {
	m.Called(id)
}

func (m *mockNode) ReportSnapshot(id uint64, status raft.SnapshotStatus) {
	m.Called(id, status)
}

func (m *mockNode) Stop() {
	m.Called()
}
