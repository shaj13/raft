package rafttest_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	raft "github.com/shaj13/raftkit"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/stretchr/testify/require"
)

func TestSnapshotShare(t *testing.T) {
	numOfEnt := 5
	otr := newOrchestrator(t)

	node := otr.create(1)[0]
	node.withOptions(raft.WithSnapshotInterval(uint64(numOfEnt)))
	otr.start(node)
	otr.waitAll()
	otr.produceData(numOfEnt)

	raw := raftpb.Member{
		ID:      2,
		Address: ":2",
	}

	// join prev node cluster.
	node = newNode().withRawMember(raw)
	node.withStartOptions(raft.WithJoin(":1", time.Second))
	otr.start(node)
	otr.wait(node)

	// snapshot must be forwarded, verify data.
	v := node.fsm.Read(numOfEnt)
	require.Equal(t, numOfEnt, v)

	// verify node 1 snapshot copied to node 2.
	cfg := otr.loopback.get(raw.Address)
	_, err := cfg.Snapshotter().Read(2, 9)
	require.NoError(t, err)
}

func TestSnapshotRestore(t *testing.T) {
	path := t.TempDir() + "/restore.snap"
	opt := raft.WithRestore(path)
	fn := func() {
		base := filepath.Dir(t.TempDir())

		// copy one of nodes snapshots.
		files, _ := filepath.Glob(base + "/*/snap/*.snap")
		err := os.Rename(files[0], path)
		require.NoError(t, err)

		// remove old state (wal, etc) for all nodes.
		files, _ = filepath.Glob(base + "/*")
		for _, f := range files {
			if strings.HasPrefix(path, f) {
				continue
			}
			err = os.RemoveAll(f)
			require.NoError(t, err)
		}
	}

	testRestore(t, fn, opt, 10, 10)
}

func TestForceNewClusterWal(t *testing.T) {
	// verify force new cluster from wal.
	opt := raft.WithForceNewCluster()
	testRestore(t, nil, opt, 100, 10)
}

func TestForceNewClusterSnapshot(t *testing.T) {
	// verify force new cluster from wal and snapshot.
	opt := raft.WithForceNewCluster()
	testRestore(t, nil, opt, 10, 15)
}

func testRestore(t *testing.T, cb func(), opt raft.StartOption, interval uint64, num int) {
	otr := newOrchestrator(t)
	nodes := otr.create(2)
	for _, n := range nodes {
		n.withOptions(raft.WithSnapshotInterval(interval))
	}
	otr.start(nodes...)
	otr.waitAll()
	otr.produceData(num)

	leader := otr.leader()
	follower := otr.follower()
	followerID := follower.rawMembers[0].ID

	// stop the first node and verify quorum loss.
	err := follower.raftnode.Shutdown(canceledctx)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		mem, _ := leader.raftnode.GetMemebr(followerID)
		if !mem.IsActive() {
			break
		}

		time.Sleep(time.Millisecond * 500)
	}

	err = leader.raftnode.RemoveMember(context.Background(), followerID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "quorum lost")

	// stop the leader and force new cluster.
	err = leader.raftnode.Shutdown(canceledctx)
	require.NoError(t, err)

	raw := leader.rawMembers[0]
	otr.nodes = nil
	leader.startOpts = nil
	leader.rawMembers = nil
	leader.withRawMember(raw)
	leader.withStartOptions(opt)
	if cb != nil {
		cb()
	}

	otr.start(leader)
	otr.wait(leader)

	err = leader.raftnode.Replicate(context.Background(), newBytesEntry(num+1, num+1))
	require.NoError(t, err)

	for i := num; i <= num+1; i++ {
		v := leader.fsm.Read(i)
		require.Equal(t, i, v)
	}
}
