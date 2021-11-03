package daemon

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/shaj13/raftkit/internal/atomic"
	membershipmock "github.com/shaj13/raftkit/internal/mocks/membership"
	storagemock "github.com/shaj13/raftkit/internal/mocks/storage"
	"github.com/shaj13/raftkit/internal/msgbus"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/pkg/v3/idutil"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

func TestReportUnreachable(t *testing.T) {
	id := uint64(1)
	ctrl := gomock.NewController(t)
	node := NewMockNode(ctrl)
	node.EXPECT().ReportUnreachable(gomock.Eq(id)).MaxTimes(1)
	d := daemon{node: node, started: atomic.NewBool()}

	// round #1 should not call ReportUnreachable when
	// daemon not started
	d.ReportUnreachable(id)

	// round #2 should call ReportUnreachable when
	// daemon started
	d.started.Set()
	d.ReportUnreachable(id)
}

func TestReportSnapshot(t *testing.T) {
	id := uint64(1)
	ctrl := gomock.NewController(t)
	node := NewMockNode(ctrl)
	node.EXPECT().ReportSnapshot(gomock.Eq(id), gomock.Eq(raft.SnapshotFinish)).MaxTimes(1)
	d := daemon{node: node, started: atomic.NewBool()}

	// round #1 should not call ReportSnapshot when
	// daemon not started
	d.ReportSnapshot(id, raft.SnapshotFinish)

	// round #2 should call ReportSnapshot when
	// daemon started
	d.started.Set()
	d.ReportSnapshot(id, raft.SnapshotFinish)
}

func TestReportShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	node := NewMockNode(ctrl)
	node.EXPECT().Stop().MaxTimes(1)
	d := daemon{
		node:    node,
		started: atomic.NewBool(),
		msgbus:  msgbus.New(),
		cancel:  func() {},
	}

	d.started.Set()
	d.ReportShutdown(0)
	require.True(t, d.started.False())
}

func TestPush(t *testing.T) {
	d := &daemon{
		msgc:    make(chan etcdraftpb.Message, 1),
		started: atomic.NewBool(),
	}

	// round #1 it return err when daemon not started
	err := d.Push(etcdraftpb.Message{})
	require.Equal(t, ErrStopped, err)

	// round #2 it return nil err when daemon started
	d.started.Set()
	err = d.Push(etcdraftpb.Message{})
	require.NoError(t, err)
}

func TestStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	node := NewMockNode(ctrl)
	node.EXPECT().Status().Return(raft.Status{}).MaxTimes(1)
	d := &daemon{
		node:    node,
		started: atomic.NewBool(),
	}

	// round #1 it return err when daemon not started
	_, err := d.Status()
	require.Equal(t, ErrStopped, err)

	// round #2 it return nil err when daemon started
	d.started.Set()
	_, err = d.Status()
	require.NoError(t, err)
}

func TestProposeReplicate(t *testing.T) {
	data := []byte("data")
	ctrl := gomock.NewController(t)
	node := NewMockNode(ctrl)
	d := &daemon{
		idgen:   idutil.NewGenerator(1, time.Now()),
		node:    node,
		started: atomic.NewBool(),
		msgbus:  msgbus.New(),
	}

	// round #1 it return err when daemon not started
	err := d.ProposeReplicate(context.TODO(), data)
	require.Equal(t, ErrStopped, err)

	// round #2 it return err whne node return's err
	expected := errors.New("TestProposeReplicate Error")
	d.started.Set()
	node.EXPECT().Propose(gomock.Any(), gomock.Any()).Return(expected)
	err = d.ProposeReplicate(context.TODO(), data)
	require.Equal(t, expected, err)

	// round #3 it return ctx done
	node = NewMockNode(ctrl)
	node.EXPECT().Propose(gomock.Any(), gomock.Any()).Return(nil)
	d.node = node
	ctx, cancel := context.WithCancel(context.TODO())
	cancel()
	err = d.ProposeReplicate(ctx, data)
	require.Equal(t, context.Canceled, err)
}

func TestProposeConfChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	node := NewMockNode(ctrl)
	d := &daemon{
		idgen:   idutil.NewGenerator(1, time.Now()),
		node:    node,
		started: atomic.NewBool(),
		msgbus:  msgbus.New(),
	}

	// round #1 it return err when daemon not started
	err := d.ProposeConfChange(context.TODO(), nil, etcdraftpb.ConfChangeAddNode)
	require.Equal(t, ErrStopped, err)

	// round #2 it return err whne node return's err
	expected := errors.New("TestProposeConfChange Error")
	d.started.Set()
	node.EXPECT().ProposeConfChange(gomock.Any(), gomock.Any()).Return(expected)
	err = d.ProposeConfChange(context.TODO(), &raftpb.Member{}, etcdraftpb.ConfChangeAddNode)
	require.Equal(t, expected, err)

	// round #3 it return ctx done
	node = NewMockNode(ctrl)
	node.EXPECT().ProposeConfChange(gomock.Any(), gomock.Any()).Return(nil)
	d.node = node
	ctx, cancel := context.WithCancel(context.TODO())
	cancel()
	err = d.ProposeConfChange(ctx, &raftpb.Member{}, etcdraftpb.ConfChangeAddNode)
	require.Equal(t, context.Canceled, err)
}

func TestTransferLeadership(t *testing.T) {
	id := uint64(1)
	ctrl := gomock.NewController(t)
	cfg := NewMockConfig(ctrl)
	node := NewMockNode(ctrl)
	cfg.EXPECT().TickInterval().Return(time.Second).AnyTimes()

	d := &daemon{
		started: atomic.NewBool(),
		node:    node,
		cfg:     cfg,
	}

	// round #1 it return err when daemon not started.
	err := d.TransferLeadership(context.TODO(), id)
	require.Equal(t, ErrStopped, err)

	// round #2 it return err when ctx done.
	node.EXPECT().Status().Return(raft.Status{}).AnyTimes()
	node.EXPECT().TransferLeadership(gomock.Any(), gomock.Any(), gomock.Eq(id))
	d.started.Set()
	ctx, cancel := context.WithCancel(context.TODO())
	cancel()
	err = d.TransferLeadership(ctx, id)
	require.Equal(t, context.Canceled, err)

	// round #3 it return nil err when TransferLeadership success.
	count := 0
	node = NewMockNode(ctrl)
	node.EXPECT().Status().AnyTimes().DoAndReturn(func() raft.Status {
		// it wait for TransferLeadership to be done.
		if count == 3 {
			return raft.Status{
				BasicStatus: raft.BasicStatus{
					SoftState: raft.SoftState{
						Lead: id,
					},
				},
			}
		}
		count++
		return raft.Status{}
	})
	node.EXPECT().TransferLeadership(gomock.Any(), gomock.Any(), gomock.Eq(id))
	d.node = node
	err = d.TransferLeadership(context.TODO(), id)
	require.NoError(t, err)
}

func TestLinearizableRead(t *testing.T) {
	expectedErr := errors.New("TestLinearizableRead")
	ctrl := gomock.NewController(t)
	node := NewMockNode(ctrl)
	d := &daemon{
		started: atomic.NewBool(),
		node:    node,
		idgen:   idutil.NewGenerator(1, time.Now()),
		msgbus:  msgbus.New(),
	}

	// round #1 it return err when daemon not started.
	err := d.LinearizableRead(context.TODO(), time.Second)
	require.Equal(t, ErrStopped, err)

	// round #2 it return err when read index return err.

	node.EXPECT().ReadIndex(gomock.Any(), gomock.Any()).Return(expectedErr)
	d.started.Set()
	err = d.LinearizableRead(context.TODO(), time.Second)
	require.Equal(t, expectedErr, err)

	// round #3 it return err when ctx done while waiting to read index.
	node = NewMockNode(ctrl)
	d.node = node
	node.EXPECT().ReadIndex(gomock.Any(), gomock.Any()).Return(nil)
	ctx, cancel := context.WithCancel(context.TODO())
	cancel()
	err = d.LinearizableRead(ctx, time.Second)
	require.Equal(t, context.Canceled, err)

	// round #3 it return exit when read index equal applied index.
	node = NewMockNode(ctrl)
	d.node = node
	node.
		EXPECT().
		ReadIndex(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, rctx []byte) error {
			sid := binary.BigEndian.Uint64(rctx)
			go func() {
				<-time.After(time.Millisecond * 50)
				d.msgbus.Broadcast(sid, uint64(0))
			}()
			return nil
		})
	d.appliedIndex = atomic.NewUint64()
	err = d.LinearizableRead(context.TODO(), time.Second)
	require.NoError(t, err)

	// round #4 it return err when ctx done while waiting for read index to be applied.
	ctx, cancel = context.WithCancel(context.TODO())
	node = NewMockNode(ctrl)
	d.node = node
	node.
		EXPECT().
		ReadIndex(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, rctx []byte) error {
			sid := binary.BigEndian.Uint64(rctx)
			go func() {
				<-time.After(time.Millisecond * 50)
				d.msgbus.Broadcast(sid, uint64(1))
				<-time.After(time.Millisecond * 50)
				cancel()
			}()
			return nil
		})
	d.appliedIndex = atomic.NewUint64()
	err = d.LinearizableRead(ctx, time.Second)
	require.Equal(t, context.Canceled, err)

	// round #5 it return err when there an error while waiting for read index to be applied.
	node = NewMockNode(ctrl)
	d.node = node
	node.
		EXPECT().
		ReadIndex(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, rctx []byte) error {
			sid := binary.BigEndian.Uint64(rctx)
			go func() {
				dur := time.Millisecond * 50
				index := uint64(1)
				<-time.After(dur)
				d.msgbus.Broadcast(sid, index)
				<-time.After(dur)
				d.msgbus.Broadcast(index, expectedErr)
			}()
			return nil
		})
	d.appliedIndex = atomic.NewUint64()
	err = d.LinearizableRead(context.TODO(), time.Second)
	require.Equal(t, expectedErr, err)
}

func TestCreateSnapshot(t *testing.T) {
	expectedErr := errors.New("TestCreateSnapshot")
	ctrl := gomock.NewController(t)
	cfg := NewMockConfig(ctrl)
	cfg.EXPECT().SnapInterval().Return(uint64(1))
	d := &daemon{
		cfg:          cfg,
		started:      atomic.NewBool(),
		appliedIndex: atomic.NewUint64(),
		snapIndex:    atomic.NewUint64(),
		cache:        raft.NewMemoryStorage(),
	}

	// round #1 it return's latest snap when indices are equaled.
	_, err := d.CreateSnapshot()
	require.NoError(t, err)

	// round #2 it return err when fsm return err.
	fsm := NewMockFSM(ctrl)
	fsm.EXPECT().Snapshot().Return(nil, expectedErr)
	d.fsm = fsm
	d.appliedIndex.Set(1)
	_, err = d.CreateSnapshot()
	require.Equal(t, expectedErr, err)

	// round #3 it return nil and create snap.
	pool := membershipmock.NewMockPool(ctrl)
	stg := storagemock.NewMockStorage(ctrl)
	shotter := storagemock.NewMockSnapshotter(ctrl)
	stg.EXPECT().Snapshotter().Return(shotter)
	stg.EXPECT().SaveSnapshot(gomock.Any()).Return(nil)
	shotter.EXPECT().Write(gomock.Any()).Return(nil)
	pool.EXPECT().Snapshot().Return(nil)
	fsm = NewMockFSM(ctrl)
	fsm.EXPECT().Snapshot().Return(nil, nil)
	d.fsm = fsm
	d.storage = stg
	d.pool = pool
	d.cache.Append([]etcdraftpb.Entry{{Index: 1}})
	_, err = d.CreateSnapshot()
	require.NoError(t, err)
	require.Equal(t, uint64(1), d.snapIndex.Get())
}
