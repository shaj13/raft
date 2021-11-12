package rafttest_test

import (
	"context"
	"testing"

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
