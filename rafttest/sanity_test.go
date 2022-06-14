package rafttest_test

import (
	"context"
	"testing"

	raft "github.com/franklee0817/raft"
	"github.com/franklee0817/raft/transport"
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

func TestGroupSanityCheck(t *testing.T) {
	// The test aims to create two raft groups.
	// each group consisting of five nodes and.
	// the data is divided into two ranges, each with its own consensus group.
	// group 1 -> range 1 - 50
	// group 2 -> range 51-99

	numOfEnt := 100
	numGroups := 2
	numNode := 5
	otr := newOrchestrator(t)
	defer otr.teardown()

	groups := make([][]*node, numGroups)
	nodeGroups := make([]*raft.NodeGroup, numNode)

	// create groups and nodes.
	for i := 0; i < numGroups; i++ {
		nodes := otr.create(numNode)
		groups[i] = nodes
	}

	for i := 0; i < numNode; i++ {
		ng := raft.NewNodeGroup(transport.GRPC)
		for j, group := range groups {
			node := group[i]
			otr.init(node)
			node.raftnode = ng.Create(uint64(j), node.fsm, node.opts...)
		}
		go ng.Start()
		nodeGroups[i] = ng
	}

	for _, group := range groups {
		otr.start(group...)
	}

	otr.waitAll()

	// split data as described above.
	gid := 0
	target := (numOfEnt / numGroups)
	count := 0
	for i := 1; i < numOfEnt; i++ {
		if count == target {
			count = 0
			gid++
		}
		count++

		node := groups[gid][0].raftnode
		data := newBytesEntry(i, i)
		err := node.Replicate(context.Background(), data)
		if err != nil {
			t.Error(err, gid, node.Whoami())
		}
	}

	// ensure linearizability in all groups.
	for _, group := range groups {
		err := group[0].raftnode.LinearizableRead(context.Background())
		require.NoError(t, err)
	}

	// verify node group x not the same data in node group y.
	for i, target := range groups {
		for j, neighbor := range groups {
			if i == j {
				continue
			}

			for key := range target[0].fsm.kv {
				_, ok := neighbor[0].fsm.kv[key]
				require.Falsef(t, ok, "Expect group %d does not exist in group %d", i, j)
			}
		}
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

func TestRestart(t *testing.T) {
	otr := newOrchestrator(t)
	defer otr.teardown()

	node := otr.create(1)[0]
	otr.start(node)
	otr.wait(node)
	otr.produceData(1)
	otr.teardown()

	node.startOpts = nil
	node.withStartOptions(raft.WithRestart())

	otr.start(node)
	otr.wait(node)

	v := node.fsm.Read(1)
	require.Equal(t, 1, v)
}
