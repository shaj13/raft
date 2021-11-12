package rafttest_test

import (
	"context"
	"testing"

	raft "github.com/shaj13/raftkit"
	"github.com/stretchr/testify/require"
)

func TestSanityCheck(t *testing.T) {
	numOfEnt := 100
	otr := newOrchestrator(t)
	defer otr.teardown()

	nodes := otr.create(5)
	otr.start(nodes...)
	otr.waitAll()
	otr.produceData(numOfEnt)

	for i := 0; i <= numOfEnt; i++ {
		node := otr.anyNode()

		err := node.raftnode.LinearizableRead(context.Background())
		require.NoError(t, err)

		v := node.fsm.Read(i)
		require.Equal(t, i, v)
	}
}

func TestDisableForwarding(t *testing.T) {
	otr := newOrchestrator(t)
	defer otr.teardown()

	nodes := otr.create(2)

	for _, n := range nodes {
		n.withOptions(raft.WithDisableProposalForwarding())
	}

	otr.start(nodes...)
	otr.waitAll()

	ctx := context.Background()

	err := otr.follower().raftnode.Replicate(ctx, []byte{})
	require.Equal(t, raft.ErrNotLeader, err)

	err = otr.leader().raftnode.Replicate(ctx, newBytesEntry(1, 1))
	require.NoError(t, err)
}
