package raftengine

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/gogo/protobuf/types"
	"github.com/golang/mock/gomock"
	"github.com/shaj13/raft/internal/atomic"
	"github.com/shaj13/raft/internal/membership"
	membershipmock "github.com/shaj13/raft/internal/mocks/membership"
	storagemock "github.com/shaj13/raft/internal/mocks/storage"
	"github.com/shaj13/raft/internal/msgbus"
	"github.com/shaj13/raft/internal/raftpb"
	"github.com/shaj13/raft/internal/storage"
	"github.com/shaj13/raft/raftlog"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/pkg/v3/idutil"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/raft/v3"
	etcdraftpb "go.etcd.io/raft/v3/raftpb"
	"go.etcd.io/raft/v3/tracker"
)

func TestNew(t *testing.T) {
	ctrl := gomock.NewController(t)
	cfg := NewMockConfig(ctrl)

	cfg.EXPECT().Storage()
	cfg.EXPECT().Pool()
	cfg.EXPECT().StateMachine()
	cfg.EXPECT().Logger()

	eng := New(cfg)
	require.NotNil(t, eng)
}

func TestStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	cfg := NewMockConfig(ctrl)
	stg := storagemock.NewMockStorage(ctrl)
	pool := membershipmock.NewMockPool(ctrl)
	node := NewMockNode(ctrl)
	ready := make(chan raft.Ready)

	eng := &engine{
		node:         node,
		logger:       raftlog.DefaultLogger,
		storage:      stg,
		pool:         pool,
		cfg:          cfg,
		msgbus:       msgbus.New(),
		cache:        raft.NewMemoryStorage(),
		started:      atomic.NewBool(),
		snapIndex:    atomic.NewUint64(),
		appliedIndex: atomic.NewUint64(),
	}

	ctx, cancel := context.WithCancel(context.TODO())
	cancel()

	defer eng.Shutdown(ctx)

	cfg.EXPECT().Context().Return(ctx).MaxTimes(2)
	cfg.EXPECT().Logger().Return(raftlog.DefaultLogger).MaxTimes(2)
	cfg.EXPECT().RaftConfig().Return(&raft.Config{}).MaxTimes(2)
	cfg.EXPECT().TickInterval().Return(time.Second).MaxTimes(2)
	cfg.EXPECT().DrainTimeout().Return(time.Nanosecond).MaxTimes(2)
	stg.EXPECT().Exist().Return(false).MaxTimes(2)
	pool.EXPECT().RegisterTypeMatcher(gomock.Any()).MaxTimes(2)
	pool.EXPECT().TearDown(gomock.Any()).MaxTimes(2)
	stg.EXPECT().Boot(gomock.Any()).MaxTimes(2)
	stg.EXPECT().Close().MaxTimes(2)
	node.EXPECT().Ready().Return(ready).MaxTimes(2)
	node.EXPECT().Stop().MaxTimes(2)
	node.EXPECT().Status().Return(raft.Status{}).MaxTimes(2)

	eng.node = nil
	err := eng.Start(":80")
	require.Error(t, err)
	require.Contains(t, err.Error(), "node not initialized")

	eng.node = node
	err = eng.Start(":80")
	require.Equal(t, ErrStopped, err)
}

func TestReportUnreachable(t *testing.T) {
	id := uint64(1)
	ctrl := gomock.NewController(t)
	node := NewMockNode(ctrl)
	node.EXPECT().ReportUnreachable(gomock.Eq(id)).MaxTimes(1)
	eng := &engine{
		logger:  raftlog.DefaultLogger,
		node:    node,
		started: atomic.NewBool(),
	}

	// round #1 should not call ReportUnreachable when
	// daemon not started
	eng.ReportUnreachable(id)

	// round #2 should call ReportUnreachable when
	// daemon started
	eng.started.Set()
	eng.ReportUnreachable(id)
}

func TestReportSnapshot(t *testing.T) {
	id := uint64(1)
	ctrl := gomock.NewController(t)
	node := NewMockNode(ctrl)
	node.EXPECT().ReportSnapshot(gomock.Eq(id), gomock.Eq(raft.SnapshotFinish)).MaxTimes(1)
	eng := &engine{node: node, started: atomic.NewBool()}

	// round #1 should not call ReportSnapshot when
	// daemon not started
	eng.ReportSnapshot(id, raft.SnapshotFinish)

	// round #2 should call ReportSnapshot when
	// daemon started
	eng.started.Set()
	eng.ReportSnapshot(id, raft.SnapshotFinish)
}

func TestReportShutdown(t *testing.T) {
	ctrl := gomock.NewController(t)
	node := NewMockNode(ctrl)
	pool := membershipmock.NewMockPool(ctrl)
	stg := storagemock.NewMockStorage(ctrl)
	cfg := NewMockConfig(ctrl)

	node.EXPECT().Stop().MaxTimes(1)
	stg.EXPECT().Close()
	pool.EXPECT().TearDown(gomock.Any())
	cfg.EXPECT().DrainTimeout().Return(time.Nanosecond)

	eng := engine{
		node:      node,
		logger:    raftlog.DefaultLogger,
		started:   atomic.NewBool(),
		msgbus:    msgbus.New(),
		storage:   stg,
		cfg:       cfg,
		pool:      pool,
		proposec:  make(chan etcdraftpb.Message),
		msgc:      make(chan etcdraftpb.Message),
		snapshotc: make(chan chan error),
		cancel:    func() {},
	}
	eng.started.Set()
	eng.ReportShutdown(0)
	require.True(t, eng.started.False())
}

func TestPush(t *testing.T) {
	eng := &engine{
		msgc:    make(chan etcdraftpb.Message, 1),
		ctx:     context.TODO(),
		started: atomic.NewBool(),
		logger:  raftlog.DefaultLogger,
	}

	// round #1 it return err when daemon not started
	err := eng.Push(etcdraftpb.Message{})
	require.Equal(t, ErrStopped, err)

	// round #2 it return nil err when daemon started
	eng.started.Set()
	err = eng.Push(etcdraftpb.Message{})
	require.NoError(t, err)

	// round #2 it return err when buffer is full
	err = eng.Push(etcdraftpb.Message{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "buffer is full")

	// round #4 it return err when ctx.Done
	eng.ctx, eng.cancel = context.WithCancel(eng.ctx)
	eng.cancel()
	err = eng.Push(etcdraftpb.Message{})
	require.Equal(t, context.Canceled, err)
}

func TestStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	node := NewMockNode(ctrl)
	node.EXPECT().Status().Return(raft.Status{}).MaxTimes(1)
	eng := &engine{
		node:    node,
		started: atomic.NewBool(),
	}

	// round #1 it return err when daemon not started
	_, err := eng.Status()
	require.Equal(t, ErrStopped, err)

	// round #2 it return nil err when daemon started
	eng.started.Set()
	_, err = eng.Status()
	require.NoError(t, err)
}

func TestProposeReplicate(t *testing.T) {
	data := []byte("data")
	ctrl := gomock.NewController(t)
	node := NewMockNode(ctrl)
	eng := &engine{
		logger:  raftlog.DefaultLogger,
		idgen:   idutil.NewGenerator(1, time.Now()),
		node:    node,
		started: atomic.NewBool(),
		msgbus:  msgbus.New(),
		ctx:     context.TODO(),
	}

	// round #1 it return err when daemon not started
	err := eng.ProposeReplicate(context.TODO(), data)
	require.Equal(t, ErrStopped, err)

	// round #2 it return err whne node return's err
	expected := errors.New("TestProposeReplicate Error")
	eng.started.Set()
	node.EXPECT().Propose(gomock.Any(), gomock.Any()).Return(expected)
	err = eng.ProposeReplicate(context.TODO(), data)
	require.Equal(t, expected, err)

	// round #3 it return ctx done
	node = NewMockNode(ctrl)
	node.EXPECT().Propose(gomock.Any(), gomock.Any()).Return(nil)
	eng.node = node
	ctx, cancel := context.WithCancel(context.TODO())
	cancel()
	err = eng.ProposeReplicate(ctx, data)
	require.Equal(t, context.Canceled, err)
}

func TestProposeConfChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	node := NewMockNode(ctrl)
	eng := &engine{
		logger:  raftlog.DefaultLogger,
		idgen:   idutil.NewGenerator(1, time.Now()),
		node:    node,
		started: atomic.NewBool(),
		msgbus:  msgbus.New(),
		ctx:     context.TODO(),
	}

	// round #1 it return err when daemon not started
	err := eng.ProposeConfChange(context.TODO())
	require.Equal(t, ErrStopped, err)

	// round #2 it return err whne node return's err
	expected := errors.New("TestProposeConfChange Error")
	eng.started.Set()
	node.EXPECT().ProposeConfChange(gomock.Any(), gomock.Any()).Return(expected)
	err = eng.ProposeConfChange(context.TODO(), &raftpb.Member{
		Type: raftpb.VoterMember,
	})
	require.Equal(t, expected, err)

	// round #3 it return ctx done
	node = NewMockNode(ctrl)
	node.EXPECT().ProposeConfChange(gomock.Any(), gomock.Any()).Return(nil)
	eng.node = node
	ctx, cancel := context.WithCancel(context.TODO())
	cancel()
	err = eng.ProposeConfChange(ctx, &raftpb.Member{})
	require.Equal(t, context.Canceled, err)
}

func TestTransferLeadership(t *testing.T) {
	id := uint64(1)
	ctrl := gomock.NewController(t)
	cfg := NewMockConfig(ctrl)
	node := NewMockNode(ctrl)
	cfg.EXPECT().TickInterval().Return(time.Second).AnyTimes()

	eng := &engine{
		logger:  raftlog.DefaultLogger,
		started: atomic.NewBool(),
		node:    node,
		cfg:     cfg,
		ctx:     context.TODO(),
	}

	// round #1 it return err when daemon not started.
	err := eng.TransferLeadership(context.TODO(), id)
	require.Equal(t, ErrStopped, err)

	// round #2 it return err when ctx done.
	node.EXPECT().Status().Return(raft.Status{}).AnyTimes()
	node.EXPECT().TransferLeadership(gomock.Any(), gomock.Any(), gomock.Eq(id))
	eng.started.Set()
	ctx, cancel := context.WithCancel(context.TODO())
	cancel()
	err = eng.TransferLeadership(ctx, id)
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
	eng.node = node
	err = eng.TransferLeadership(context.TODO(), id)
	require.NoError(t, err)
}

func TestLinearizableRead(t *testing.T) {
	expectedErr := errors.New("TestLinearizableRead")
	ctrl := gomock.NewController(t)
	cfg := NewMockConfig(ctrl)
	node := NewMockNode(ctrl)
	eng := &engine{
		logger:  raftlog.DefaultLogger,
		cfg:     cfg,
		started: atomic.NewBool(),
		node:    node,
		idgen:   idutil.NewGenerator(1, time.Now()),
		msgbus:  msgbus.New(),
		ctx:     context.TODO(),
	}

	cfg.EXPECT().TickInterval().Return(time.Millisecond * 100).AnyTimes()

	// round #1 it return err when daemon not started.
	err := eng.LinearizableRead(context.TODO())
	require.Equal(t, ErrStopped, err)

	// round #2 it return err when read index return err.

	node.EXPECT().ReadIndex(gomock.Any(), gomock.Any()).Return(expectedErr)
	eng.started.Set()
	err = eng.LinearizableRead(context.TODO())
	require.Equal(t, expectedErr, err)

	// round #3 it return err when ctx done while waiting to read index.
	node = NewMockNode(ctrl)
	eng.node = node
	node.EXPECT().ReadIndex(gomock.Any(), gomock.Any()).Return(nil)
	ctx, cancel := context.WithCancel(context.TODO())
	cancel()
	err = eng.LinearizableRead(ctx)
	require.Equal(t, context.Canceled, err)

	// round #3 it return exit when read index equal applied index.
	node = NewMockNode(ctrl)
	eng.node = node
	node.
		EXPECT().
		ReadIndex(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, rctx []byte) error {
			sid := binary.BigEndian.Uint64(rctx)
			go func() {
				<-time.After(time.Millisecond * 50)
				eng.msgbus.Broadcast(sid, uint64(0))
			}()
			return nil
		})
	eng.appliedIndex = atomic.NewUint64()
	err = eng.LinearizableRead(context.TODO())
	require.NoError(t, err)

	// round #4 it return err when ctx done while waiting for read index to be applied.
	ctx, cancel = context.WithCancel(context.TODO())
	node = NewMockNode(ctrl)
	eng.node = node
	node.
		EXPECT().
		ReadIndex(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, rctx []byte) error {
			sid := binary.BigEndian.Uint64(rctx)
			go func() {
				<-time.After(time.Millisecond * 50)
				eng.msgbus.Broadcast(sid, uint64(1))
				<-time.After(time.Millisecond * 50)
				cancel()
			}()
			return nil
		})
	eng.appliedIndex = atomic.NewUint64()
	err = eng.LinearizableRead(ctx)
	require.Equal(t, context.Canceled, err)

	// round #5 it return err when there an error while waiting for read index to be applied.
	node = NewMockNode(ctrl)
	eng.node = node
	node.
		EXPECT().
		ReadIndex(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, rctx []byte) error {
			sid := binary.BigEndian.Uint64(rctx)
			go func() {
				dur := time.Millisecond * 50
				index := uint64(1)
				<-time.After(dur)
				eng.msgbus.Broadcast(sid, index)
				<-time.After(dur)
				eng.msgbus.Broadcast(index, expectedErr)
			}()
			return nil
		})
	eng.appliedIndex = atomic.NewUint64()
	err = eng.LinearizableRead(context.TODO())
	require.Equal(t, expectedErr, err)
}

func TestLocalCreateSnapshot(t *testing.T) {
	expectedErr := errors.New("TestCreateSnapshot")
	ctrl := gomock.NewController(t)
	cfg := NewMockConfig(ctrl)
	cfg.EXPECT().SnapInterval().Return(uint64(1))
	eng := &engine{
		logger:       raftlog.DefaultLogger,
		cfg:          cfg,
		started:      atomic.NewBool(),
		appliedIndex: atomic.NewUint64(),
		snapIndex:    atomic.NewUint64(),
		cache:        raft.NewMemoryStorage(),
	}

	eng.started.Set()

	// round #1 it refuse to create snap when indices are equaled.
	err := eng.createSnapshot()
	require.NoError(t, err)

	// round #2 it return err when fsm return err.
	fsm := NewMockStateMachine(ctrl)
	fsm.EXPECT().Snapshot().Return(nil, expectedErr)
	eng.fsm = fsm
	eng.appliedIndex.Set(1)
	err = eng.createSnapshot()
	require.Equal(t, expectedErr, err)

	// round #3 it return nil and create snap.
	pool := membershipmock.NewMockPool(ctrl)
	stg := storagemock.NewMockStorage(ctrl)
	shotter := storagemock.NewMockSnapshotter(ctrl)
	stg.EXPECT().Snapshotter().Return(shotter)
	stg.EXPECT().SaveSnapshot(gomock.Any()).Return(nil)
	shotter.EXPECT().Write(gomock.Any()).Return(nil)
	pool.EXPECT().Snapshot().Return(nil)
	fsm = NewMockStateMachine(ctrl)
	fsm.EXPECT().Snapshot().Return(nil, nil)
	eng.fsm = fsm
	eng.storage = stg
	eng.pool = pool
	eng.cache.Append([]etcdraftpb.Entry{{Index: 1}})
	err = eng.createSnapshot()
	require.NoError(t, err)
	require.Equal(t, uint64(1), eng.snapIndex.Get())
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
	cfg.EXPECT().SnapInterval().Return(uint64(100))
	node.EXPECT().Advance()
	node.EXPECT().Status()
	node.EXPECT().Ready().Return(readyc).AnyTimes()
	stg.EXPECT().SaveEntries(gomock.Any(), gomock.Any()).Return(nil).MinTimes(1)
	eng := &engine{
		appliedIndex: atomic.NewUint64(),
		snapIndex:    atomic.NewUint64(),
		node:         node,
		storage:      stg,
		ctx:          ctx,
		cfg:          cfg,
	}

	err := eng.eventLoop()
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
	eng := &engine{
		msgbus: bus,
	}
	eng.publishReadState([]raft.ReadState{rs})
	got := <-sub.Chan()
	require.Equal(t, index, got)
}

func TestPublishAppliedIndices(t *testing.T) {
	bus := msgbus.New()
	eng := &engine{
		msgbus: bus,
	}

	s1 := bus.SubscribeOnce(2)
	s2 := bus.SubscribeOnce(3)

	eng.publishAppliedIndices(1, 3)

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
	fsm := NewMockStateMachine(ctrl)

	snap := &etcdraftpb.Snapshot{
		Metadata: etcdraftpb.SnapshotMetadata{
			Index: 1,
		},
	}

	sf := &storage.Snapshot{
		SnapshotState: raftpb.SnapshotState{
			Raw: *snap,
		},
	}

	stg.EXPECT().SaveSnapshot(gomock.Any()).Return(nil)
	stg.EXPECT().Snapshotter().Return(shotter)
	shotter.EXPECT().Read(gomock.Any(), gomock.Any()).Return(sf, nil)
	pool.EXPECT().Restore(gomock.Any())
	fsm.EXPECT().Restore(gomock.Any()).Return(nil)

	eng := &engine{
		logger:       raftlog.DefaultLogger,
		cache:        raft.NewMemoryStorage(),
		storage:      stg,
		appliedIndex: atomic.NewUint64(),
		snapIndex:    atomic.NewUint64(),
		pool:         pool,
		fsm:          fsm,
	}
	eng.appliedIndex.Set(1)

	// round #1 it return's err if snap index is lower than applied index.
	err := eng.publishSnapshot(*snap)
	require.Error(t, err)
	require.Contains(t, err.Error(), " should > progress.appliedIndex")

	// round #2 it publish snapshot.
	snap.Metadata.Index = 3
	sf.Raw = *snap
	err = eng.publishSnapshot(*snap)
	require.NoError(t, err)
	require.Equal(t, snap.Metadata.Index, eng.snapIndex.Get())
	require.Equal(t, snap.Metadata.Index, eng.appliedIndex.Get())
}

func TestPublishReplicate(t *testing.T) {
	sid := uint64(1)
	data := []byte("testData")
	ctrl := gomock.NewController(t)
	fsm := NewMockStateMachine(ctrl)
	eng := &engine{
		logger: raftlog.DefaultLogger,
		fsm:    fsm,
		msgbus: msgbus.New(),
	}
	sub := eng.msgbus.SubscribeOnce(sid)
	rp := &raftpb.Replicate{
		Data: data,
		CID:  sid,
	}
	ent := etcdraftpb.Entry{
		Data: pbutil.MustMarshal(rp),
	}
	fsm.EXPECT().Apply(gomock.Eq(data))
	eng.publishReplicate(ent)
	v := <-sub.Chan()
	require.Nil(t, v)
}

func TestPublishConfChange(t *testing.T) {
	closedc := make(chan struct{})
	close(closedc)

	table := []struct {
		change etcdraftpb.ConfChangeType
		expect func(*gomock.Controller, *engine) <-chan struct{}
		err    error
	}{
		{
			change: etcdraftpb.ConfChangeAddNode,
			expect: func(ctrl *gomock.Controller, d *engine) <-chan struct{} {
				pool := membershipmock.NewMockPool(ctrl)
				pool.EXPECT().Add(gomock.Any()).MinTimes(1).Return(ErrStopped)
				d.pool = pool
				return closedc
			},
			err: ErrStopped,
		},
		{
			change: etcdraftpb.ConfChangeUpdateNode,
			expect: func(ctrl *gomock.Controller, d *engine) <-chan struct{} {
				pool := membershipmock.NewMockPool(ctrl)
				pool.EXPECT().Update(gomock.Any()).MinTimes(1)
				d.pool = pool
				return closedc
			},
		},
	}

	for _, tt := range table {
		sid := uint64(1)
		ctrl := gomock.NewController(t)
		node := NewMockNode(ctrl)
		eng := &engine{
			logger: raftlog.DefaultLogger,
			node:   node,
			msgbus: msgbus.New(),
			ctx:    context.TODO(),
		}
		sub := eng.msgbus.SubscribeOnce(sid)
		jr := raftpb.MembershipChange{
			CID: 1,
			Members: []*raftpb.Member{
				{
					ID:        1,
					CreatedAt: &types.Timestamp{},
				},
			},
		}
		cc := &etcdraftpb.ConfChangeV2{
			Changes: []etcdraftpb.ConfChangeSingle{
				{
					Type:   tt.change,
					NodeID: 1,
				},
			},
			Context: pbutil.MustMarshal(&jr),
		}
		ent := etcdraftpb.Entry{
			Data: pbutil.MustMarshal(cc),
		}

		node.EXPECT().ApplyConfChange(gomock.Eq(cc))
		wait := tt.expect(ctrl, eng)
		eng.publishConfChange(ent)
		v := <-sub.Chan()
		require.Equal(t, tt.err, v)
		<-wait
		ctrl.Finish()
	}
}

func TestProcess(t *testing.T) {
	c := make(chan etcdraftpb.Message)
	ctrl := gomock.NewController(t)
	node := NewMockNode(ctrl)
	eng := new(engine)
	eng.logger = raftlog.DefaultLogger
	eng.node = node
	eng.ctx, eng.cancel = context.WithCancel(context.TODO())

	node.EXPECT().Step(gomock.Any(), gomock.Any()).Return(ErrStopped).MinTimes(1)

	eng.process(c)

	c <- etcdraftpb.Message{}
	eng.cancel()

	// it should not process this msg.
	c <- etcdraftpb.Message{}
	eng.wg.Wait()
	ctrl.Finish()
}

func TestSend(t *testing.T) {
	table := []func(*gomock.Controller, *engine, uint64){
		func(ctrl *gomock.Controller, eng *engine, id uint64) {
			pool := membershipmock.NewMockPool(ctrl)
			pool.EXPECT().Get(gomock.Eq(id)).Return(nil, false)
			eng.pool = pool
		},
		func(ctrl *gomock.Controller, eng *engine, id uint64) {
			mem := membershipmock.NewMockMember(ctrl)
			pool := membershipmock.NewMockPool(ctrl)
			pool.EXPECT().Get(gomock.Eq(id)).Return(mem, true)
			mem.EXPECT().Send(gomock.Any()).Return(ErrStopped)
			eng.pool = pool
		},
	}

	for _, tt := range table {
		ctrl := gomock.NewController(t)
		msg := etcdraftpb.Message{To: 1}
		eng := new(engine)
		eng.logger = raftlog.DefaultLogger
		tt(ctrl, eng, msg.To)
		eng.send([]etcdraftpb.Message{msg})
		ctrl.Finish()
	}
}

func TestPromotions(t *testing.T) {
	ctrl := gomock.NewController(t)
	node := NewMockNode(ctrl)
	pool := membershipmock.NewMockPool(ctrl)
	voter := membershipmock.NewMockMember(ctrl)
	staging := membershipmock.NewMockMember(ctrl)
	cfg := NewMockConfig(ctrl)

	eng := &engine{
		logger:  raftlog.DefaultLogger,
		node:    node,
		pool:    pool,
		cfg:     cfg,
		idgen:   idutil.NewGenerator(1, time.Now()),
		started: atomic.NewBool(),
	}
	eng.ctx, eng.cancel = context.WithCancel(context.TODO())
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
		Return(ErrStopped)

	eng.started.Set()
	eng.promotions()
	ctrl.Finish()
}

func TestCreateSnapshot(t *testing.T) {
	eng := &engine{
		logger:       raftlog.DefaultLogger,
		cache:        raft.NewMemoryStorage(),
		started:      atomic.NewBool(),
		snapIndex:    atomic.NewUint64(),
		appliedIndex: atomic.NewUint64(),
		snapshotc:    make(chan chan error),
	}

	_, err := eng.CreateSnapshot()
	require.Equal(t, ErrStopped, err)

	eng.started.Set()
	_, err = eng.CreateSnapshot()
	require.NoError(t, err)

	go func() {
		c := <-eng.snapshotc
		c <- ErrNoLeader
	}()

	eng.appliedIndex.Set(10)
	_, err = eng.CreateSnapshot()
	require.Equal(t, ErrNoLeader, err)
}

func TestForceSnapshot(t *testing.T) {
	ctrl := gomock.NewController(t)
	node := NewMockNode(ctrl)
	fsm := NewMockStateMachine(ctrl)

	eng := &engine{
		node:         node,
		fsm:          fsm,
		started:      atomic.NewBool(),
		appliedIndex: atomic.NewUint64(),
		snapIndex:    atomic.NewUint64(),
		logger:       raftlog.DefaultLogger,
	}

	// round #1 it should return false when msg not snapshot.
	ok := eng.forceSnapshot(etcdraftpb.Message{})
	require.False(t, ok)

	msg := &etcdraftpb.Message{
		From: 1,
		To:   2,
		Type: etcdraftpb.MsgSnap,
		Snapshot: &etcdraftpb.Snapshot{
			Metadata: etcdraftpb.SnapshotMetadata{
				ConfState: etcdraftpb.ConfState{
					Voters: []uint64{1, 2},
				},
			},
		},
	}

	// round #2 it should return false when to exist in voters list.
	ok = eng.forceSnapshot(*msg)
	require.False(t, ok)

	// round #3 it should create snapshot and report snap as failed.
	msg.To = 4
	eng.started.Set()
	eng.appliedIndex.Set(1)
	node.EXPECT().ReportSnapshot(gomock.Eq(msg.To), gomock.Eq(raft.SnapshotFailure))
	fsm.EXPECT().Snapshot().Return(nil, ErrNoLeader)
	ok = eng.forceSnapshot(*msg)
	require.True(t, ok)
	ctrl.Finish()
}
