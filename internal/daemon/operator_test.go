package daemon

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/shaj13/raftkit/internal/mocks"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/rpc"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/raft/v3"
)

func TestInvoke(t *testing.T) {
	temp := make(map[string]int, len(order))
	for k, v := range order {
		temp[k] = v
	}
	defer func() {
		order = temp
	}()

	ctrl := gomock.NewController(t)

	oprs := make([]Operator, 0)
	before := make([]*gomock.Call, 3)
	after := make([]*gomock.Call, 3)

	for i := 0; i < 3; i++ {
		opr := NewMockOperator(ctrl)
		opr.EXPECT().String().Return(fmt.Sprintf("%d", i)).AnyTimes()
		before[i] = opr.EXPECT().before(gomock.Any()).Return(nil)
		after[i] = opr.EXPECT().after(gomock.Any()).Return(nil)
		// append to index 1.
		oprs = append([]Operator{opr}, oprs...)
		// add it to order.
		order[opr.String()] = i
	}

	gomock.InOrder(before...)
	gomock.InOrder(after...)

	// it invoke operator by order.
	err := invoke(nil, oprs...)
	require.NoError(t, err)

	// it return error when operator.before return err.
	opr := NewMockOperator(ctrl)
	opr.EXPECT().before(gomock.Any()).Return(ErrStopped)
	err = invoke(nil, opr)
	require.Equal(t, ErrStopped, err)

	// it return error when operator.after return err.
	opr = NewMockOperator(ctrl)
	opr.EXPECT().before(gomock.Any()).Return(nil)
	opr.EXPECT().after(gomock.Any()).Return(ErrStopped)
	err = invoke(nil, opr)
	require.Equal(t, ErrStopped, err)
}

func TestMembers(t *testing.T) {
	d := new(daemon)
	d.ost = new(operatorsState)

	// it should return's err on invalid url.
	err := Members("").before(d)
	require.Error(t, err)
	require.Contains(t, err.Error(), "url")

	// it should parse members.
	err = Members("1=:8080", "2=:9090").before(d)
	require.NoError(t, err)
	require.Equal(t, uint64(1), d.ost.local.ID)
	require.Equal(t, ":8080", d.ost.local.Address)
	require.Equal(t, raftpb.LocalMember, d.ost.local.Type)
	require.Equal(t, 1, len(d.ost.membs))
	require.Equal(t, uint64(2), d.ost.membs[0].ID)
	require.Equal(t, ":9090", d.ost.membs[0].Address)
	require.Equal(t, raftpb.RemoteMember, d.ost.membs[0].Type)

	err = Members().after(d)
	require.NoError(t, err)
}

func TestJoin(t *testing.T) {
	d := new(daemon)
	d.ost = new(operatorsState)
	d.ost.wasExisted = true

	err := Join("", 0).before(d)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already part")
}

func TestInitCluster(t *testing.T) {
	d := new(daemon)
	d.ost = new(operatorsState)
	d.ost.wasExisted = true

	err := InitCluster().before(d)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exist")

	var peers []raft.Peer
	temp := startNode
	defer func() {
		startNode = temp
	}()

	startNode = func(c *raft.Config, p []raft.Peer) raft.Node {
		peers = p
		return nil
	}

	_ = Members("1=:8080", "2=:9090").before(d)
	err = InitCluster().after(d)
	require.NoError(t, err)
	require.Equal(t, 2, len(peers))
	require.Equal(t, uint64(1), peers[0].ID)
	require.Equal(t, uint64(2), peers[1].ID)
}

func TestRestart(t *testing.T) {
	nodeRestarted := false
	d := new(daemon)
	d.ost = new(operatorsState)

	defer mockRestartNode(&nodeRestarted)()

	err := Restart().before(d)
	require.Error(t, err)
	require.Contains(t, err.Error(), "state not found")

	err = Restart().after(d)
	require.NoError(t, err)
	require.True(t, nodeRestarted)
}

func TestFallback(t *testing.T) {
	fn := func() { Fallback().after(nil) }
	require.PanicsWithValue(t, "fallback.after called before fallback.before", fn)

	err := Fallback(noFallbackTest{}).before(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "can't be used with fallback")

	ctrl := gomock.NewController(t)
	first := NewMockOperator(ctrl)
	second := NewMockOperator(ctrl)
	first.EXPECT().before(gomock.Any()).Return(fmt.Errorf("1"))
	second.EXPECT().before(gomock.Any()).Return(fmt.Errorf("2"))
	err = Fallback(first, second).before(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "1, 2")

	first = NewMockOperator(ctrl)
	first.EXPECT().before(gomock.Any()).MaxTimes(1)
	first.EXPECT().after(gomock.Any()).MaxTimes(1)
	opr := Fallback(first)
	err = opr.before(nil)
	require.NoError(t, err)
	err = opr.after(nil)
	require.NoError(t, err)
	ctrl.Finish()
}

func TestForceJoin(t *testing.T) {
	nodeRestarted := false
	localID := uint64(1)
	membs := []raftpb.Member{
		{ID: 2},
	}
	ctrl := gomock.NewController(t)
	cfg := NewMockConfig(ctrl)
	client := mocks.NewMockClient(ctrl)
	pool := mocks.NewMockPool(ctrl)
	dial := func(context.Context, string) (rpc.Client, error) {
		return client, nil
	}
	d := new(daemon)
	d.ost = new(operatorsState)
	d.ost.local = &raftpb.Member{
		ID: 10,
	}
	d.pool = pool
	d.cfg = cfg

	// setup mocks expectation.
	cfg.
		EXPECT().
		Dial().
		Return(dial)

	client.
		EXPECT().
		Join(gomock.Any(), gomock.Eq(*d.ost.local)).
		Return(localID, membs, nil)

	pool.
		EXPECT().
		Add(gomock.Eq(membs[0])).
		Return(nil)

	defer mockRestartNode(&nodeRestarted)()

	// it call join and set ost.
	err := ForceJoin("", 0).before(d)
	require.NoError(t, err)
	require.Equal(t, localID, d.ost.local.ID)
	require.Equal(t, membs, d.ost.membs)

	// it call pool add.
	err = ForceJoin("", 0).after(d)
	require.NoError(t, err)
	require.True(t, nodeRestarted)
}

func mockRestartNode(called *bool) func() {
	temp := restartNode
	fn := func() {
		restartNode = temp
	}

	restartNode = func(c *raft.Config) raft.Node {
		*called = true
		return nil
	}
	return fn
}

type noFallbackTest struct {
	*MockOperator
}

func (noFallbackTest) noFallback()    {}
func (noFallbackTest) String() string { return "" }
