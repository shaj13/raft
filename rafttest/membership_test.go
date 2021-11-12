package rafttest_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateMember(t *testing.T) {
	info := []byte("additional information")
	otr := newOrchestrator(t)
	defer otr.teardown()

	nodes := otr.create(2)
	otr.start(nodes...)
	otr.waitAll()

	node := otr.anyNode()
	raw := node.rawMember()
	(&raw).Context = info

	err := node.raftnode.UpdateMember(context.Background(), &raw)
	require.NoError(t, err)

	for _, n := range nodes {
		mem, ok := n.raftnode.GetMemebr(raw.ID)
		require.True(t, ok)
		require.Equal(t, info, mem.Raw().Context)
	}
}
