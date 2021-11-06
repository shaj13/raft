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

func TestNewRemote(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := transportmock.NewMockClient(ctrl)
	cfg := NewMockConfig(ctrl)
	client.EXPECT().Close().Return(nil)
	dial := mockDial(client, nil)
	cfg.EXPECT().Dial().Return(dial).MaxTimes(2)
	cfg.EXPECT().Reporter().Return(nil)
	cfg.EXPECT().DrainTimeout().Return(time.Duration(-1))

	m, err := newRemote(context.Background(), cfg, raftpb.Member{})
	require.NoError(t, err)
	require.NotNil(t, m)

	err = m.Close()
	require.NoError(t, err)
}

func TestRemote(t *testing.T) {
	id := uint64(1)
	addr := ":50051"
	r := remote{
		raw: raftpb.Member{
			ID:      id,
			Address: addr,
		},
	}

	require.Equal(t, r.ID(), id)
	require.Equal(t, r.Address(), addr)
	require.False(t, r.IsActive())
	require.Equal(t, r.ActiveSince(), time.Time{})
	require.Equal(t, r.Type(), raftpb.VoterMember)
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
	r.raw = raftpb.Member{Address: addr}
	r.rc = client
	r.ctx = context.TODO()
	r.dial = mockDial(nil, err)

	// Round #1 it update raw and does not close rc.
	got := r.Update(raftpb.Member{Address: addr})
	require.NoError(t, got)
	require.Equal(t, addr, r.Address())

	// Round #2 it return error whn dial return error
	got = r.Update(raftpb.Member{Address: uaddr})
	require.Equal(t, err, got)
	require.Equal(t, addr, r.Address())

	// Round #3 it update addr and close old tr
	r.dial = mockDial(client, nil)
	got = r.Update(raftpb.Member{Address: uaddr})
	require.NoError(t, got)
	require.Equal(t, uaddr, r.Address())
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
			r.raw = raftpb.Member{ID: id}
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
	r.raw = raftpb.Member{}

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

func TestRemoteProcess(t *testing.T) {
	ctrl := gomock.NewController(t)
	rep := NewMockReporter(ctrl)
	client := transportmock.NewMockClient(ctrl)

	rep.EXPECT().ReportUnreachable(gomock.Any())
	client.EXPECT().Message(gomock.Any(), gomock.Any()).Return(fmt.Errorf("TestRemoteRun Message error"))
	client.EXPECT().Close().Return(nil)

	r := new(remote)
	r.r = rep
	r.raw = raftpb.Member{}
	r.cfg = testConfig(t)
	r.rc = client
	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.active = true
	r.done = make(chan struct{})
	r.msgc = make(chan etcdraftpb.Message, 1)
	go r.process(context.TODO())

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
