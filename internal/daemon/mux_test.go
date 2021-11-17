package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

const testGroupID = uint64(1)

func TestMuxOp(t *testing.T) {
	table := []struct {
		fn func(m *mux)
		ot operationType
	}{
		{
			fn: func(mux *mux) {
				mux.add(testGroupID, nil, nil)
			},
			ot: add,
		},
		{
			fn: func(mux *mux) {
				mux.remove(testGroupID)
			},
			ot: remove,
		},
		{
			fn: func(mux *mux) {
				mux.tick(testGroupID)
			},
			ot: call,
		},
		{
			fn: func(mux *mux) {
				mux.advance(testGroupID)
			},
			ot: advance,
		},
	}

	for _, tt := range table {
		mux := &mux{
			operationc: make(chan *operation),
		}
		go tt.fn(mux)
		op := <-mux.operationc
		close(op.done)
		require.Equal(t, testGroupID, op.gid)
		require.Equal(t, tt.ot, op.ot)
	}
}

func TestMuxPush(t *testing.T) {
	mux := NewMux().(*mux)
	op := new(operation)
	ctx, cancel := context.WithCancel(context.TODO())
	cancel()

	// it return err when ctx done.
	err := mux.push(ctx, op)
	require.Equal(t, context.Canceled, err)

	// it return err when mux done.
	close(mux.done)
	err = mux.push(context.TODO(), op)
	require.Equal(t, ErrStopped, err)

	mux.done = make(chan struct{})
	mux.operationc = make(chan *operation, 10)

	// it return err when ctx done and op pushed.
	ctx, cancel = context.WithTimeout(context.TODO(), time.Millisecond*50)
	defer cancel()
	err = mux.push(ctx, op)
	require.Equal(t, context.DeadlineExceeded, err)

	// it return err when mux done and op pushed.
	go func() {
		time.Sleep(time.Millisecond * 50)
		close(mux.done)
	}()
	err = mux.push(context.TODO(), op)
	require.Equal(t, ErrStopped, err)
}

func TestMux(t *testing.T) {
	ctrl := gomock.NewController(t)
	mux := NewMux()
	stg := raft.NewMemoryStorage()
	rcfg := &raft.Config{
		ID:              1,
		Storage:         stg,
		ElectionTick:    2,
		MaxInflightMsgs: 256,
		HeartbeatTick:   1,
	}

	cfg := NewMockConfig(ctrl)
	cfg.EXPECT().RaftConfig().Return(rcfg)
	cfg.EXPECT().GroupID()
	cfg.EXPECT().Mux().Return(mux)

	go mux.Start()
	defer mux.Stop()

	peers := []raft.Peer{{ID: 1}}
	node := bootstrap(cfg, peers)

	rd := <-node.Ready()
	ent := rd.CommittedEntries[0]
	stg.Append(rd.Entries)
	cc := new(etcdraftpb.ConfChange)
	pbutil.MustUnmarshal(cc, ent.Data)
	node.ApplyConfChange(cc)
	node.Advance()
	for i := 0; i <= 3; i++ {
		node.Tick()
	}

	require.Equal(t, node.Status().Lead, rcfg.ID)
}
