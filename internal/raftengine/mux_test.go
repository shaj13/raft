package raftengine

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/raft/v3"
	etcdraftpb "go.etcd.io/raft/v3/raftpb"
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
			ot: tick,
		},
		{
			fn: func(mux *mux) {
				mux.advance(testGroupID)
			},
			ot: advance,
		},
		{
			fn: func(mux *mux) {
				msg := etcdraftpb.Message{
					Type: etcdraftpb.MsgHeartbeat,
				}
				mux.step(context.TODO(), testGroupID, msg)
			},
			ot: heartbeat,
		},
		{
			fn: func(mux *mux) {
				mux.step(context.TODO(), testGroupID, etcdraftpb.Message{})
			},
			ot: call,
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
	t.Skip()
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

func TestHeartbeatsSuppress(t *testing.T) {
	key := "ctx data"
	rd := raft.Ready{
		Messages: []etcdraftpb.Message{
			{
				Type:    etcdraftpb.MsgHeartbeat,
				Context: []byte(key),
			},
			{
				Type: etcdraftpb.MsgHeartbeatResp,
			},
			{
				Type: etcdraftpb.MsgHeartbeatResp,
			},
			{
				Type: etcdraftpb.MsgApp,
			},
		},
	}

	hb := newHeartbeats()
	rd = hb.suppress(rd)
	_, ok := hb.pending[key]
	require.Equal(t, 1, len(rd.Messages))
	require.Equal(t, rd.Messages[0].Type, etcdraftpb.MsgApp)
	require.True(t, ok)
}

func TestHeartbeatsCoalesced(t *testing.T) {
	peers := []raft.Peer{
		{ID: 1},
		{ID: 2},
	}

	nodes := map[uint64]*nodeState{
		1: testNodeState(t, peers),
		2: testNodeState(t, peers),
	}

	hb := newHeartbeats()
	hb.pending["test"] = struct{}{}
	hb.id = 1

	hb.coalesced(nodes)

	got := []etcdraftpb.Message{}

	for _, n := range nodes {
		select {
		case rd := <-n.readyc:
			got = append(got, rd.Messages...)

		default:
		}
	}

	require.Equal(t, 1, len(got))
	require.Equal(t, etcdraftpb.MsgHeartbeat, got[0].Type)
	require.Equal(t, hb.id, got[0].From)
	require.Equal(t, peers[1].ID, got[0].To)
	require.Equal(t, `{"Buffers":["dGVzdA=="]}`, string(got[0].Context))
}

func TestFanout(t *testing.T) {
	msg := etcdraftpb.Message{
		Type:    etcdraftpb.MsgHeartbeat,
		From:    1,
		To:      1,
		Context: []byte(`{"Buffers":["dGVzdA=="]}`),
	}

	hb := newHeartbeats()
	hb.id = msg.To
	count := 0
	hb.step = func(_ *nodeState, cmsg etcdraftpb.Message) {
		count++
		require.Equal(t, msg.Type, cmsg.Type)
		require.Equal(t, msg.From, cmsg.From)
		require.Equal(t, msg.To, cmsg.To)
		require.Equal(t, "test", string(cmsg.Context))
	}

	node := testNodeState(t, nil)
	node.lead = msg.From

	nodes := map[uint64]*nodeState{
		1: node,
		2: testNodeState(t, nil),
	}

	hb.fanout(nodes, msg)
	got := []etcdraftpb.Message{}

	for _, n := range nodes {
		select {
		case rd := <-n.readyc:
			got = append(got, rd.Messages...)

		default:
		}
	}

	require.Equal(t, 1, len(got))
	require.Equal(t, 2, count)
	require.Equal(t, etcdraftpb.MsgHeartbeatResp, got[0].Type)
	require.Equal(t, msg.To, got[0].From)
	require.Equal(t, msg.From, got[0].To)
	require.Equal(t, msg.Context, got[0].Context)
}

func testNodeState(t *testing.T, peers []raft.Peer) *nodeState {
	cfg := &raft.Config{
		ID:              1,
		Storage:         raft.NewMemoryStorage(),
		ElectionTick:    2,
		MaxInflightMsgs: 256,
		HeartbeatTick:   1,
	}

	rn, err := raft.NewRawNode(cfg)
	require.NoError(t, err)

	if len(peers) > 0 {
		err = rn.Bootstrap(peers)
		require.NoError(t, err)
	}

	return &nodeState{
		readyc: make(chan raft.Ready, 1),
		cfg:    cfg,
		rn:     rn,
	}

}
