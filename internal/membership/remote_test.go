package membership

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

func TestRemote(t *testing.T) {
	id := uint64(1)
	addr := ":50051"
	r := remote{
		id:   id,
		addr: addr,
	}

	require.Equal(t, r.ID(), id)
	require.Equal(t, r.Address(), addr)
	require.False(t, r.IsActive())
	require.Equal(t, r.Since(), time.Time{})
	require.Equal(t, r.Type(), raftpb.RemoteMember)
	require.Nil(t, r.client())
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
		require.Equal(t, tt.in, r.IsActive())
	}
}

func TestRemoteUpdate(t *testing.T) {
	err := fmt.Errorf("TestRemoteUpdate dial error")
	addr := ":5050"
	uaddr := ":5051"

	m := &mockRPC{mock.Mock{}}
	m.On("Close").Return(nil)

	r := new(remote)
	r.addr = addr
	r.rc = m
	r.ctx = context.TODO()
	r.dial = mockDial(nil, err)

	// Round #1 it does not update addr if are the same
	got := r.Update(addr)
	require.NoError(t, got)
	require.Equal(t, addr, r.addr)

	// Round #2 it return error whn dial return error
	got = r.Update(uaddr)
	require.Equal(t, err, got)
	require.Equal(t, addr, r.addr)

	// Round #3 it update addr and close old tr
	r.dial = mockDial(m, nil)
	got = r.Update(uaddr)
	require.NoError(t, got)
	require.Equal(t, uaddr, r.addr)
	m.AssertCalled(t, "Close")
}

func TestRemoteStream(t *testing.T) {
	m := &mockRPC{mock.Mock{}}
	m.On("Message").Return(nil)
	r := new(remote)
	r.rc = m
	r.cfg = testConfig
	_ = r.stream(context.Background(), etcdraftpb.Message{})
	m.AssertCalled(t, "Message")
}

func TestRemoteReport(t *testing.T) {
	id := uint64(1)
	err := fmt.Errorf("TestRemoteReport error")

	table := []struct {
		name     string
		msg      etcdraftpb.Message
		err      error
		called   string
		callargs []interface{}
	}{
		{
			name:     "it call ReportSnapshot with SnapshotFinish status",
			msg:      etcdraftpb.Message{Type: etcdraftpb.MsgSnap},
			called:   "ReportSnapshot",
			callargs: []interface{}{id, raft.SnapshotFinish},
		},
		{
			name:     "it call ReportSnapshot with SnapshotFailure status",
			msg:      etcdraftpb.Message{Type: etcdraftpb.MsgSnap},
			err:      err,
			called:   "ReportSnapshot",
			callargs: []interface{}{id, raft.SnapshotFailure},
		},
		{
			name:     "it call ReportUnreachables",
			msg:      etcdraftpb.Message{},
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
	r.msgc = make(chan etcdraftpb.Message)
	r.r = m

	// Round #1 it return error when ctx canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.ctx = ctx
	err := r.Send(etcdraftpb.Message{})
	require.Contains(t, err.Error(), "canceled")

	// Round #2 it return error when chan is full
	r.ctx = context.Background()
	err = r.Send(etcdraftpb.Message{})
	require.Contains(t, err.Error(), "buffer is full")
}

func TestRemoteDrain(t *testing.T) {
	mt := &mockRPC{mock.Mock{}}
	mt.On("Message").Return(nil)
	r := new(remote)
	r.msgc = make(chan etcdraftpb.Message, 1)
	r.rc = mt
	r.cfg = testConfig
	r.ctx = context.Background()

	// Round #1 it return error when ctx done
	_ = r.Send(etcdraftpb.Message{})
	err := r.drain()
	require.Contains(t, err.Error(), "deadline exceeded")

	// Round #2 it return nil error when all msgs flushed
	_ = r.Send(etcdraftpb.Message{})
	close(r.msgc)
	err = r.drain()
	require.NoError(t, err)
	mt.AssertCalled(t, "Message")
}

func TestRemoteRun(t *testing.T) {
	mr := &mockReporter{mock.Mock{}}
	mt := &mockRPC{mock.Mock{}}
	mt.On("Message").Return(fmt.Errorf("TestRemoteRun Message error"))
	mt.On("Close").Return(nil)
	mr.On("ReportUnreachable", uint64(0)).Return()
	r := new(remote)
	r.r = mr
	r.cfg = testConfig
	r.rc = mt
	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.active = true
	r.done = make(chan struct{})
	r.msgc = make(chan etcdraftpb.Message, 1)
	go r.run()

	_ = r.Send(etcdraftpb.Message{})

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

	require.False(t, r.active)
	mt.AssertCalled(t, "Message")
	mr.AssertCalled(t, "ReportUnreachable", r.id)

	// reset active to ensure close set remote as inactive
	r.active = true
	r.Close()

	mt.AssertCalled(t, "Close")
	require.False(t, r.active)
}
