package rafttest_test

import (
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
}
