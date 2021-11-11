package rafttest_test

import (
	"context"
	"fmt"
	"testing"

	raft "github.com/shaj13/raftkit"
	"github.com/stretchr/testify/require"
)

func TestSanityCheck(t *testing.T) {
	numOfnode := 5
	numOfEnt := 100
	otr := newOrchestrator(t)

	for i := 1; i <= numOfnode; i++ {
		raw := raft.RawMember{
			ID:      uint64(i),
			Address: fmt.Sprintf(":%d", i),
		}

		node := newNode().withRawMember(raw).withStartOptions(raft.WithInitCluster())

		// add other nodes raws.
		for j := 1; j <= numOfnode; j++ {
			if j == i {
				continue
			}

			raw := raft.RawMember{
				ID:      uint64(j),
				Address: fmt.Sprintf(":%d", j),
			}
			node.withRawMember(raw)
		}

		otr.start(node)
	}

	otr.waitAll()

	for i := 0; i <= numOfEnt; i++ {
		node := otr.anyNode().raftnode
		data := newBytesEntry(i, i)
		err := node.Replicate(context.Background(), data)
		require.NoError(t, err)
	}

	for i := 0; i <= numOfEnt; i++ {
		node := otr.anyNode()

		err := node.raftnode.LinearizableRead(context.Background())
		require.NoError(t, err)

		v := node.fsm.Read(i)
		require.Equal(t, i, v)
	}
}
