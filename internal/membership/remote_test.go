package membership

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	transportmock "github.com/shaj13/raftkit/internal/mocks/transport"
	"github.com/shaj13/raftkit/internal/raftpb"
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
	ctrl := gomock.NewController(t)
	client := transportmock.NewMockClient(ctrl)

	client.EXPECT().Close().Return(nil)

	r := new(remote)
	r.addr = addr
	r.rc = client
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
	r.dial = mockDial(client, nil)
	got = r.Update(uaddr)
	require.NoError(t, got)
	require.Equal(t, uaddr, r.addr)
}

func TestRemoteStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := transportmock.NewMockClient(ctrl)
	client.EXPECT().Message(gomock.Any(), gomock.Any()).Return(nil)

	r := new(remote)
	r.rc = client
	r.cfg = testConfig(t)
	err := r.stream(context.TODO(), etcdraftpb.Message{})
	require.NoError(t, err)
}

func TestRemoteReport(t *testing.T) {
	id := uint64(1)
	err := fmt.Errorf("TestRemoteReport error")

	table := []struct {
		name     string
		msg      etcdraftpb.Message
		expect   func(*MockReporter)
		err      error
		called   string
		callargs []interface{}
	}{
		{
			name: "it call ReportSnapshot with SnapshotFinish status",
			msg:  etcdraftpb.Message{Type: etcdraftpb.MsgSnap},
			expect: func(mr *MockReporter) {
				mr.
					EXPECT().
					ReportSnapshot(gomock.Eq(id), gomock.Eq(raft.SnapshotFinish))
			},
		},
		{
			name: "it call ReportSnapshot with SnapshotFailure status",
			msg:  etcdraftpb.Message{Type: etcdraftpb.MsgSnap},
			err:  err,
			expect: func(mr *MockReporter) {
				mr.
					EXPECT().
					ReportSnapshot(gomock.Eq(id), gomock.Eq(raft.SnapshotFailure))
			},
		},
		{
			name: "it call ReportUnreachables",
			msg:  etcdraftpb.Message{},
			err:  err,
			expect: func(mr *MockReporter) {
				mr.
					EXPECT().
					ReportUnreachable(gomock.Eq(id))
			},
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			rep := NewMockReporter(ctrl)
			tt.expect(rep)

			r := new(remote)
			r.r = rep
			r.id = id
			r.report(tt.msg, tt.err)
		})
	}
}

func TestRemoteSend(t *testing.T) {
	ctrl := gomock.NewController(t)
	rep := NewMockReporter(ctrl)
	rep.EXPECT().ReportUnreachable(gomock.Any()).MaxTimes(2)

	r := new(remote)
	r.msgc = make(chan etcdraftpb.Message)
	r.r = rep

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
	table := []struct {
		name    string
		timeout time.Duration
		err     string
	}{
		{
			name:    "it return error when ctx done",
			timeout: -1,
			err:     "deadline exceeded",
		},
		{
			name:    "it return nil when all msgs flushed",
			timeout: time.Second * 3,
			err:     "deadline exceeded",
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			client := transportmock.NewMockClient(ctrl)
			client.EXPECT().Message(gomock.Any(), gomock.Any()).Return(nil)

			cfg := testConfig(t)
			cfg.EXPECT().DrainTimeout().Return(tt.timeout)

			r := new(remote)
			r.msgc = make(chan etcdraftpb.Message, 1)
			r.rc = client
			r.cfg = cfg
			r.ctx = context.TODO()

			_ = r.Send(etcdraftpb.Message{})
			err := r.drain()
			if tt.err != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRemoteRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	rep := NewMockReporter(ctrl)
	client := transportmock.NewMockClient(ctrl)

	rep.EXPECT().ReportUnreachable(gomock.Any())
	client.EXPECT().Message(gomock.Any(), gomock.Any()).Return(fmt.Errorf("TestRemoteRun Message error"))
	client.EXPECT().Close().Return(nil)

	r := new(remote)
	r.r = rep
	r.cfg = testConfig(t)
	r.rc = client
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

	// reset active to ensure close set remote as inactive
	r.active = true
	r.Close()
	require.False(t, r.active)
}
