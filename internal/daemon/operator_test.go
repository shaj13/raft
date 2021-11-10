package daemon

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	membershipmock "github.com/shaj13/raftkit/internal/mocks/membership"
	storagemock "github.com/shaj13/raftkit/internal/mocks/storage"
	transportmock "github.com/shaj13/raftkit/internal/mocks/transport"

	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/storage"
	"github.com/shaj13/raftkit/internal/transport"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
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
	_, err := invoke(nil, oprs...)
	require.NoError(t, err)

	// it return error when operator.before return err.
	opr := NewMockOperator(ctrl)
	opr.EXPECT().before(gomock.Any()).Return(ErrStopped)
	_, err = invoke(nil, opr)
	require.Equal(t, ErrStopped, err)

	// it return error when operator.after return err.
	opr = NewMockOperator(ctrl)
	opr.EXPECT().before(gomock.Any()).Return(nil)
	opr.EXPECT().after(gomock.Any()).Return(ErrStopped)
	_, err = invoke(nil, opr)
	require.Equal(t, ErrStopped, err)
}

func TestMembers(t *testing.T) {
	ost := new(operatorsState)

	// it should not set local or membs.
	err := Members().before(ost)
	require.NoError(t, err)
	require.Nil(t, ost.local)
	require.Equal(t, 0, len(ost.membs))

	// it should not set local or membs.
	err = Members(raftpb.Member{ID: 1}).before(ost)
	require.NoError(t, err)
	require.NotNil(t, ost.local)
	require.Equal(t, uint64(1), ost.local.ID)
	require.Equal(t, 0, len(ost.membs))

	// it should set local and membs.
	err = Members(raftpb.Member{ID: 1}, raftpb.Member{ID: 2}).before(ost)
	require.NoError(t, err)
	require.Equal(t, uint64(1), ost.local.ID)
	require.Equal(t, 1, len(ost.membs))
	require.Equal(t, uint64(2), ost.membs[0].ID)

	err = Members().after(ost)
	require.NoError(t, err)
}

func TestJoin(t *testing.T) {
	ost := new(operatorsState)
	ost.hasExistingState = true

	err := Join("", 0).before(ost)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already part")
}

func TestInitCluster(t *testing.T) {
	ost := new(operatorsState)
	ost.daemon = new(daemon)
	ost.hasExistingState = true

	err := InitCluster().before(ost)
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

	_ = Members(raftpb.Member{ID: 1}, raftpb.Member{ID: 2}).before(ost)
	err = InitCluster().after(ost)
	require.NoError(t, err)
	require.Equal(t, 2, len(peers))
	require.Equal(t, uint64(1), peers[0].ID)
	require.Equal(t, uint64(2), peers[1].ID)
}

func TestRestart(t *testing.T) {
	nodeRestarted := false
	ost := new(operatorsState)
	ost.daemon = new(daemon)

	defer mockRestartNode(&nodeRestarted)()

	err := Restart().before(ost)
	require.Error(t, err)
	require.Contains(t, err.Error(), "state not found")

	err = Restart().after(ost)
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
	client := transportmock.NewMockClient(ctrl)
	pool := membershipmock.NewMockPool(ctrl)
	dial := func(context.Context, string) (transport.Client, error) {
		return client, nil
	}
	ost := new(operatorsState)
	ost.local = &raftpb.Member{
		ID: 10,
	}
	ost.daemon = new(daemon)
	ost.daemon.pool = pool
	ost.daemon.cfg = cfg

	// setup mocks expectation.
	cfg.
		EXPECT().
		Dial().
		Return(dial)

	client.
		EXPECT().
		Join(gomock.Any(), gomock.Eq(*ost.local)).
		Return(localID, membs, nil)

	pool.
		EXPECT().
		Add(gomock.Eq(membs[0])).
		Return(nil)

	defer mockRestartNode(&nodeRestarted)()

	// it call join and set ost.
	err := ForceJoin("", 0).before(ost)
	require.NoError(t, err)
	require.Equal(t, localID, ost.local.ID)
	require.Equal(t, membs, ost.membs)

	// it call pool add.
	err = ForceJoin("", 0).after(ost)
	require.NoError(t, err)
	require.True(t, nodeRestarted)
}

func TestSetup(t *testing.T) {
	setup := &setup{}
	local := &raftpb.Member{ID: 10}
	meta := pbutil.MustMarshal(local)
	ents := []etcdraftpb.Entry{{Index: 5}}
	hs := etcdraftpb.HardState{Term: 2}
	sf := &storage.Snapshot{}
	ctrl := gomock.NewController(t)
	stg := storagemock.NewMockStorage(ctrl)
	pool := membershipmock.NewMockPool(ctrl)
	cfg := NewMockConfig(ctrl)
	ost := new(operatorsState)
	ost.daemon = new(daemon)
	ost.daemon.storage = stg
	ost.daemon.cfg = cfg
	ost.daemon.pool = pool

	// setup mocks expectation.
	stg.EXPECT().Exist().Return(false).AnyTimes()
	stg.
		EXPECT().
		Boot(gomock.Any()).
		Return(meta, hs, ents, sf, nil)

	cfg.EXPECT().RaftConfig().Return(&raft.Config{})
	pool.EXPECT().RegisterTypeMatcher(gomock.Any())

	ids := map[uint64]struct{}{}
	for i := 0; i < 20; i++ {
		err := setup.before(ost)
		require.NoError(t, err)
		require.False(t, ost.hasExistingState)

		// assert id are auto gen.
		_, ok := ids[ost.local.ID]
		require.False(t, ok)
	}

	// assert it return's err onnn addr.
	err := setup.after(ost)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no address set")

	// assert it boot from storage.
	ost.local.Address = ":8080"
	err = setup.after(ost)
	require.NoError(t, err)
	require.Equal(t, local, ost.local)
	require.Equal(t, hs, ost.hst)
	require.Equal(t, ents, ost.ents)
	require.Equal(t, sf, ost.sf)
	require.Equal(t, local.ID, ost.cfg.ID)

}

func TestStateSetup(t *testing.T) {
	table := []struct {
		name      string
		ost       operatorsState
		called    bool
		expectErr bool
	}{
		{
			name: "it return nil error when ost.wasExited = false",
			ost: operatorsState{
				hasExistingState: false,
				sf:               &storage.Snapshot{},
			},
		},
		{
			name: "it return error when puplish snap return error",
			ost: operatorsState{
				hasExistingState: true,
				sf: &storage.Snapshot{
					SnapshotState: raftpb.SnapshotState{
						Raw: etcdraftpb.Snapshot{
							Metadata: etcdraftpb.SnapshotMetadata{Index: 1},
						},
					},
				},
			},
			called:    true,
			expectErr: true,
		},
		{
			name: "it return nil error when puplish snap success",
			ost: operatorsState{
				hasExistingState: true,
				sf: &storage.Snapshot{
					SnapshotState: raftpb.SnapshotState{
						Raw: etcdraftpb.Snapshot{
							Metadata: etcdraftpb.SnapshotMetadata{Index: 1},
						},
					},
				},
			},
			called: true,
		},
		{
			name: "it return nil error when and not call publish snap",
			ost: operatorsState{
				hasExistingState: true,
				sf:               &storage.Snapshot{},
			},
			called: false,
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			fn := func(*storage.Snapshot) error {
				called = true
				if tt.expectErr {
					return fmt.Errorf("")
				}
				return nil
			}

			ss := stateSetup{
				publishSnapshotFile: fn,
			}

			ost := &tt.ost
			ost.daemon = new(daemon)
			ost.daemon.cache = raft.NewMemoryStorage()

			err := ss.after(ost)
			require.Equal(t, tt.expectErr, err != nil)
			require.Equal(t, tt.called, called)
		})
	}
}

func TestForceNewCluster(t *testing.T) {
	ost := new(operatorsState)
	ost.local = &raftpb.Member{ID: 1}
	ost.membs = []raftpb.Member{
		{ID: 4},
		{ID: 5},
	}
	ost.sf = &storage.Snapshot{
		SnapshotState: raftpb.SnapshotState{
			Raw: etcdraftpb.Snapshot{Metadata: etcdraftpb.SnapshotMetadata{
				Index: 2,
				Term:  1,
			}},
		},
	}
	ost.hst = etcdraftpb.HardState{
		Commit: 2,
	}
	ost.ents = []etcdraftpb.Entry{
		{
			Index: 1,
			Type:  etcdraftpb.EntryConfChange,
			Data: pbutil.MustMarshal(&etcdraftpb.ConfChange{
				NodeID: 1,
				Type:   etcdraftpb.ConfChangeAddNode,
			}),
		},
		{
			Index: 2,
			Type:  etcdraftpb.EntryConfChange,
			Data: pbutil.MustMarshal(&etcdraftpb.ConfChange{
				NodeID: 2,
				Type:   etcdraftpb.ConfChangeAddNode,
			}),
		},
		{
			Index: 3,
			Type:  etcdraftpb.EntryNormal,
		},
	}
	ost.daemon = new(daemon)
	ctrl := gomock.NewController(t)
	shotter := storagemock.NewMockSnapshotter(ctrl)
	stg := storagemock.NewMockStorage(ctrl)

	stg.
		EXPECT().
		Snapshotter().
		Return(shotter).
		MaxTimes(2)

	stg.
		EXPECT().
		SaveEntries(gomock.Any(), gomock.Any()).
		Return(nil)

	shotter.
		EXPECT().
		Write(gomock.Any()).
		Return(nil)

	shotter.
		EXPECT().
		Read(gomock.Any()).
		Return(nil, nil)

	ost.daemon.storage = stg

	err := ForceNewCluster().after(ost)
	confChange := 0
	entNormal := 0
	for _, ent := range ost.ents {
		switch ent.Type {
		case etcdraftpb.EntryConfChange:
			confChange++
		case etcdraftpb.EntryNormal:
			entNormal++
		default:
			t.Error("unexpected entry type")
		}
	}

	require.NoError(t, err)
	require.Equal(t, uint64(5), ost.hst.Commit)
	require.Equal(t, 0, entNormal)
	require.Equal(t, 5, confChange)
}

func TestRestore(t *testing.T) {
	hs := etcdraftpb.HardState{
		Term:   1,
		Vote:   1,
		Commit: 1,
	}
	ctrl := gomock.NewController(t)
	stg := storagemock.NewMockStorage(ctrl)
	shotter := storagemock.NewMockSnapshotter(ctrl)
	opr := restore{}
	ost := new(operatorsState)
	ost.local = &raftpb.Member{ID: 1}
	ost.daemon = new(daemon)
	ost.daemon.storage = stg

	stg.
		EXPECT().
		Boot(gomock.Any()).
		Return(nil, etcdraftpb.HardState{}, nil, nil, nil)

	stg.
		EXPECT().
		Snapshotter().
		Return(shotter).
		MaxTimes(2)

	stg.
		EXPECT().
		SaveEntries(gomock.Eq(hs), gomock.Any()).
		Return(nil)

	stg.
		EXPECT().
		SaveSnapshot(gomock.Any()).
		Return(nil)

	stg.
		EXPECT().
		Close().
		Return(nil)

	shotter.
		EXPECT().
		ReadFromPath(gomock.Any()).
		Return(&storage.Snapshot{}, nil)

	shotter.
		EXPECT().
		Write(gomock.Any()).
		Return(nil)

	ost.hasExistingState = true
	err := opr.before(ost)
	require.Error(t, err)
	require.Contains(t, err.Error(), "found orphan node state")

	ost.hasExistingState = false
	err = opr.before(ost)
	require.NoError(t, err)
}

func TestRemovedMembers(t *testing.T) {
	ctrl := gomock.NewController(t)
	pool := membershipmock.NewMockPool(ctrl)
	rm := new(removedMembers)
	ost := new(operatorsState)
	mem := &raftpb.Member{
		ID:   1,
		Type: raftpb.RemovedMember,
	}
	cc := &etcdraftpb.ConfChange{
		Type:    etcdraftpb.ConfChangeRemoveNode,
		Context: pbutil.MustMarshal(mem),
	}
	ost.ents = []etcdraftpb.Entry{
		{
			Index: 1,
			Type:  etcdraftpb.EntryConfChange,
			Data:  pbutil.MustMarshal(cc),
		},
	}
	ost.hst = etcdraftpb.HardState{
		Commit: 1,
	}
	ost.daemon = &daemon{
		pool: pool,
	}

	pool.EXPECT().Add(gomock.Any()).Return(ErrStopped)
	err := rm.after(ost)
	require.Equal(t, ErrStopped, err)
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
