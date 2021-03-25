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
	r.cfg = testConfig
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

func TestRemoteSend(t *testing.T) {
	m := &mockReporter{mock.Mock{}}
	m.On("ReportUnreachable", uint64(0)).Return()

	r := new(remote)
	r.msgc = make(chan raftpb.Message)
	r.r = m

	// Round #1 it return error when ctx canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.ctx = ctx
	err := r.Send(raftpb.Message{})
	assert.Contains(t, err.Error(), "canceled")

	// Round #2 it return error when chan is full
	r.ctx = context.Background()
	err = r.Send(raftpb.Message{})
	assert.Contains(t, err.Error(), "buffer is full")
}

func TestRemoteDrain(t *testing.T) {
	mt := &mockTransport{mock.Mock{}}
	mt.On("RoundTrip").Return(nil)
	r := new(remote)
	r.msgc = make(chan raftpb.Message, 1)
	r.tr = mt
	r.cfg = testConfig
	r.ctx = context.Background()

	// Round #1 it return error when ctx done
	_ = r.Send(raftpb.Message{})
	err := r.drain()
	assert.Contains(t, err.Error(), "deadline exceeded")

	// Round #2 it return nil error when all msgs flushed
	_ = r.Send(raftpb.Message{})
	close(r.msgc)
	err = r.drain()
	assert.NoError(t, err)
	mt.AssertCalled(t, "RoundTrip")
}

func TestRemoteRun(t *testing.T) {
	mr := &mockReporter{mock.Mock{}}
	mt := &mockTransport{mock.Mock{}}
	mt.On("RoundTrip").Return(fmt.Errorf("TestRemoteRun RoundTrip error"))
	mt.On("Close").Return(nil)
	mr.On("ReportUnreachable", uint64(0)).Return()
	r := new(remote)
	r.r = mr
	r.cfg = testConfig
	r.tr = mt
	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.active = true
	r.done = make(chan struct{})
	r.msgc = make(chan raftpb.Message, 1)
	go r.run()

	_ = r.Send(raftpb.Message{})

	for i := 0; i < 5; i++ {
		if len(r.msgc) == 0 {
			break
		}
		if i == 4 {
			t.Error("run method haven't read from msgc")
			break
		}
		time.Sleep(time.Second)
	}

	assert.False(t, r.active)
	mt.AssertCalled(t, "RoundTrip")
	mr.AssertCalled(t, "ReportUnreachable", r.id)

	// reset active to ensure close set remote as inactive
	r.active = true
	r.Close()

	mt.AssertCalled(t, "Close")
	assert.False(t, r.active)
}
