package daemon

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/shaj13/raftkit/internal/atomic"
	"github.com/shaj13/raftkit/internal/membership"
	membershipmock "github.com/shaj13/raftkit/internal/mocks/membership"
	storagemock "github.com/shaj13/raftkit/internal/mocks/storage"
	"github.com/shaj13/raftkit/internal/msgbus"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/storage"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/pkg/v3/idutil"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/raft/v3/tracker"
)

func TestNew(t *testing.T) {
	ctrl := gomock.NewController(t)
	cfg := NewMockConfig(ctrl)

	cfg.EXPECT().Storage()
	cfg.EXPECT().Pool()

	d := New(cfg)
	require.NotNil(t, d)
}

func TestStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	cfg := NewMockConfig(ctrl)
	stg := storagemock.NewMockStorage(ctrl)
	pool := membershipmock.NewMockPool(ctrl)
	node := NewMockNode(ctrl)

	d := &daemon{
		node:    node,
		storage: stg,
		pool:    pool,
		cfg:     cfg,
		msgbus:  msgbus.New(),
		cache:   raft.NewMemoryStorage(),
		started: atomic.NewBool(),
	}

	ctx, cancel := context.WithCancel(context.TODO())
	cancel()
	cfg.EXPECT().Context().Return(ctx)
	cfg.EXPECT().RaftConfig().Return(&raft.Config{})
	cfg.EXPECT().TickInterval().Return(time.Second)
	stg.EXPECT().Exist().Return(false)
	pool.EXPECT().RegisterTypeMatcher(gomock.Any())
	stg.EXPECT().Boot(gomock.Any())
	stg.EXPECT().Close()
	node.EXPECT().Ready()
	node.EXPECT().Stop()

	err := d.Start(":80")
	require.Equal(t, ErrStopped, err)
}

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

func TestEventLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	count := 0
	ctrl := gomock.NewController(t)
	node := NewMockNode(ctrl)
	stg := storagemock.NewMockStorage(ctrl)
	cfg := NewMockConfig(ctrl)
	readyc := make(chan raft.Ready, 1)

	readyc <- raft.Ready{}
	node.EXPECT().Tick().MaxTimes(3).Do(func() {
		if count == 2 {
			cancel()
		}
		count++
	})

	cfg.EXPECT().TickInterval().Return(time.Millisecond * 100)
	node.EXPECT().Advance()
	node.EXPECT().Ready().Return(readyc).AnyTimes()
	stg.EXPECT().SaveEntries(gomock.Any(), gomock.Any()).Return(nil).MinTimes(1)
	d := new(daemon)
	d.appliedIndex = atomic.NewUint64()
	d.node = node
	d.storage = stg
	d.notify = make(chan struct{}, 1)
	d.ctx = ctx
	d.cfg = cfg

	err := d.eventLoop()
	require.Equal(t, ErrStopped, err)
}

func TestPublishReadState(t *testing.T) {
	buf := make([]byte, 8)
	sid := uint64(1)
	index := uint64(2)
	binary.BigEndian.PutUint64(buf, sid)
	rs := raft.ReadState{
		Index:      index,
		RequestCtx: buf,
	}
	bus := msgbus.New()
	sub := bus.SubscribeOnce(sid)
	d := new(daemon)
	d.msgbus = bus
	d.publishReadState([]raft.ReadState{rs})
	got := <-sub.Chan()
	require.Equal(t, index, got)
}

func TestPublishAppliedIndices(t *testing.T) {
	bus := msgbus.New()
	d := new(daemon)
	d.msgbus = bus

	s1 := bus.SubscribeOnce(2)
	s2 := bus.SubscribeOnce(3)

	d.publishAppliedIndices(1, 3)

	v1 := <-s1.Chan()
	v2 := <-s2.Chan()

	require.Nil(t, v2)
	require.Nil(t, v1)
}

func TestPublishSnapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	stg := storagemock.NewMockStorage(ctrl)
	shotter := storagemock.NewMockSnapshotter(ctrl)
	pool := membershipmock.NewMockPool(ctrl)
	fsm := NewMockFSM(ctrl)

	snap := &etcdraftpb.Snapshot{
		Metadata: etcdraftpb.SnapshotMetadata{
			Index: 1,
		},
	}

	sf := &storage.SnapshotFile{
		Snap: snap,
		Pool: new(raftpb.Pool),
	}

	stg.EXPECT().SaveSnapshot(gomock.Any()).Return(nil)
	stg.EXPECT().Snapshotter().Return(shotter)
	shotter.EXPECT().Read(gomock.Any()).Return(sf, nil)
	pool.EXPECT().Restore(gomock.Any())
	fsm.EXPECT().Restore(gomock.Any()).Return(nil)

	daemon := new(daemon)
	daemon.cache = raft.NewMemoryStorage()
	daemon.storage = stg
	daemon.appliedIndex = atomic.NewUint64()
	daemon.snapIndex = atomic.NewUint64()
	daemon.pool = pool
	daemon.fsm = fsm
	daemon.appliedIndex.Set(1)

	// round #1 it return's err if snap index is lower than applied index.
	err := daemon.publishSnapshot(*snap)
	require.Error(t, err)
	require.Contains(t, err.Error(), " should > progress.appliedIndex")

	// round #2 it publish snapshot.
	snap.Metadata.Index = 3
	err = daemon.publishSnapshot(*snap)
	require.NoError(t, err)
	require.Equal(t, snap.Metadata.Index, daemon.snapIndex.Get())
	require.Equal(t, snap.Metadata.Index, daemon.appliedIndex.Get())
}

func TestPublishReplicate(t *testing.T) {
	sid := uint64(1)
	data := []byte("testData")
	ctrl := gomock.NewController(t)
	fsm := NewMockFSM(ctrl)
	d := new(daemon)
	d.fsm = fsm
	d.msgbus = msgbus.New()
	sub := d.msgbus.SubscribeOnce(sid)
	rp := &raftpb.Replicate{
		Data: data,
		CID:  sid,
	}
	ent := etcdraftpb.Entry{
		Data: pbutil.MustMarshal(rp),
	}
	fsm.EXPECT().Apply(gomock.Eq(data))
	d.publishReplicate(ent)
	v := <-sub.Chan()
	require.Nil(t, v)
}

func TestPublishConfChange(t *testing.T) {
	closedc := make(chan struct{})
	close(closedc)

	table := []struct {
		change etcdraftpb.ConfChangeType
		expect func(*gomock.Controller, *daemon) <-chan struct{}
		err    error
	}{
		{
			change: etcdraftpb.ConfChangeAddNode,
			expect: func(ctrl *gomock.Controller, d *daemon) <-chan struct{} {
				pool := membershipmock.NewMockPool(ctrl)
				pool.EXPECT().Add(gomock.Any()).MinTimes(1).Return(ErrStopped)
				d.pool = pool
				return closedc
			},
			err: ErrStopped,
		},
		{
			change: etcdraftpb.ConfChangeUpdateNode,
			expect: func(ctrl *gomock.Controller, d *daemon) <-chan struct{} {
				pool := membershipmock.NewMockPool(ctrl)
				pool.EXPECT().Update(gomock.Any()).MinTimes(1)
				d.pool = pool
				return closedc
			},
		},
		{
			change: etcdraftpb.ConfChangeRemoveNode,
			expect: func(ctrl *gomock.Controller, d *daemon) <-chan struct{} {
				c := make(chan struct{})
				pool := membershipmock.NewMockPool(ctrl)
				cfg := NewMockConfig(ctrl)
				pool.EXPECT().Remove(gomock.Any()).DoAndReturn(func(raftpb.Member) error {
					c <- struct{}{}
					return ErrStopped
				}).MinTimes(1)
				cfg.EXPECT().TickInterval().Return(time.Duration(-1))
				d.pool = pool
				d.cfg = cfg
				return c
			},
		},
	}

	for _, tt := range table {
		sid := uint64(1)
		ctrl := gomock.NewController(t)
		node := NewMockNode(ctrl)
		d := new(daemon)
		d.node = node
		d.msgbus = msgbus.New()
		sub := d.msgbus.SubscribeOnce(sid)
		mem := &raftpb.Member{
			ID: 1,
		}
		cc := &etcdraftpb.ConfChange{
			Type:    tt.change,
			ID:      sid,
			Context: pbutil.MustMarshal(mem),
		}
		ent := etcdraftpb.Entry{
			Data: pbutil.MustMarshal(cc),
		}

		node.EXPECT().ApplyConfChange(gomock.Eq(cc))
		wait := tt.expect(ctrl, d)
		d.publishConfChange(ent)
		v := <-sub.Chan()
		require.Equal(t, tt.err, v)
		<-wait
		ctrl.Finish()
	}
}

func TestProcess(t *testing.T) {
	c := make(chan etcdraftpb.Message)
	done := make(chan struct{})
	ctrl := gomock.NewController(t)
	node := NewMockNode(ctrl)
	d := new(daemon)
	d.node = node
	d.ctx, d.cancel = context.WithCancel(context.TODO())

	node.EXPECT().Step(gomock.Any(), gomock.Any()).Return(ErrStopped).MinTimes(1)
	go func() {
		d.process(c)
		done <- struct{}{}
	}()

	c <- etcdraftpb.Message{}
	d.cancel()
	<-done
	ctrl.Finish()
}

func TestSend(t *testing.T) {
	table := []func(*gomock.Controller, *daemon, uint64){
		func(ctrl *gomock.Controller, d *daemon, id uint64) {
			pool := membershipmock.NewMockPool(ctrl)
			pool.EXPECT().Get(gomock.Eq(id)).Return(nil, false)
			d.pool = pool
		},
		func(ctrl *gomock.Controller, d *daemon, id uint64) {
			mem := membershipmock.NewMockMember(ctrl)
			pool := membershipmock.NewMockPool(ctrl)
			pool.EXPECT().Get(gomock.Eq(id)).Return(mem, true)
			mem.EXPECT().Send(gomock.Any()).Return(ErrStopped)
			d.pool = pool
		},
	}

	for _, tt := range table {
		ctrl := gomock.NewController(t)
		msg := etcdraftpb.Message{To: 1}
		d := new(daemon)
		tt(ctrl, d, msg.To)
		d.send([]etcdraftpb.Message{msg})
		ctrl.Finish()
	}
}

func TestSnapshots(t *testing.T) {
	c := make(chan struct{})
	done := make(chan struct{})
	ctrl := gomock.NewController(t)
	fsm := NewMockFSM(ctrl)
	cfg := NewMockConfig(ctrl)
	d := new(daemon)
	d.cfg = cfg
	d.appliedIndex = atomic.NewUint64()
	d.snapIndex = atomic.NewUint64()
	d.fsm = fsm

	cfg.EXPECT().SnapInterval().Return(uint64(1)).MaxTimes(2)
	fsm.EXPECT().Snapshot().Return(nil, ErrStopped).MinTimes(1)

	go func() {
		d.snapshots(c)
		done <- struct{}{}
	}()

	// round #1 it should not create snapshot.
	c <- struct{}{}

	// round #2 it create snapshot.
	d.appliedIndex.Set(10)
	c <- struct{}{}

	close(c)
	<-done
	ctrl.Finish()
}

func TestPromotions(t *testing.T) {
	c := make(chan struct{})
	done := make(chan struct{})
	ctrl := gomock.NewController(t)
	node := NewMockNode(ctrl)
	pool := membershipmock.NewMockPool(ctrl)
	voter := membershipmock.NewMockMember(ctrl)
	staging := membershipmock.NewMockMember(ctrl)
	cfg := NewMockConfig(ctrl)

	d := new(daemon)
	d.node = node
	d.pool = pool
	d.cfg = cfg
	d.idgen = idutil.NewGenerator(1, time.Now())
	d.started = atomic.NewBool()

	rs := raft.Status{
		BasicStatus: raft.BasicStatus{
			ID: 1,
		},
		Progress: map[uint64]tracker.Progress{
			1: {Match: 100},
			2: {Match: 90},
		},
	}

	cfg.EXPECT().TickInterval().Return(time.Duration(-1))
	voter.EXPECT().Raw().Return(raftpb.Member{ID: 1})
	voter.EXPECT().IsActive().Return(true)
	staging.EXPECT().Raw().Return(raftpb.Member{ID: 2, Type: raftpb.StagingMember})
	staging.EXPECT().IsActive().Return(true)
	pool.EXPECT().Members().Return([]membership.Member{voter, staging})
	node.EXPECT().Status().Return(rs)
	node.
		EXPECT().
		ProposeConfChange(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, cc etcdraftpb.ConfChangeI) error {
			done <- struct{}{}
			return ErrStopped
		})

	d.started.Set()

	go d.promotions(c)

	c <- struct{}{}
	close(c)
	<-done
	ctrl.Finish()
}
