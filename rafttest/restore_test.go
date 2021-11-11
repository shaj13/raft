package rafttest_test

import (
	"context"
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

func TestForceNewCluster(t *testing.T) {
	otr := newOrchestrator(t)
	nodes := otr.create(2)
	otr.start(nodes...)
	otr.waitAll()

	leader := otr.leader()
	follower := otr.follower()
	followerID := follower.rawMembers[0].ID

	// stop the first node and verify quorum loss.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := follower.raftnode.Shutdown(ctx)
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
	err = leader.raftnode.Shutdown(ctx)
	require.NoError(t, err)

	raw := leader.rawMembers[0]
	otr.nodes = nil
	leader.startOpts = nil
	leader.rawMembers = nil
	leader.withRawMember(raw)
	leader.withStartOptions(raft.WithForceNewCluster())

	otr.start(leader)
	otr.wait(leader)

	err = leader.raftnode.Replicate(context.Background(), newBytesEntry(2, 2))
	require.NoError(t, err)

	v := leader.fsm.Read(2)
	require.Equal(t, 2, v)
}
