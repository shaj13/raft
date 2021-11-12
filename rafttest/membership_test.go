package rafttest_test

import (
	"context"
	"testing"
	"time"

	raft "github.com/shaj13/raftkit"
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

func TestRemoveMember(t *testing.T) {
	otr := newOrchestrator(t)
	defer otr.teardown()

	nodes := otr.create(3)
	otr.start(nodes...)
	otr.waitAll()

	leader := otr.leader()

	// remove all nodes exempt leader
	for _, n := range nodes {
		id := n.rawMember().ID
		if id == leader.rawMember().ID {
			continue
		}

		err := leader.raftnode.RemoveMember(context.Background(), id)
		require.NoError(t, err)

		// wait until mem removed.
		for i := 0; i <= 5; i++ {
			mem, _ := leader.raftnode.GetMemebr(id)
			if mem.Type() == raft.RemovedMember {
				break
			}

			time.Sleep(time.Millisecond * 500)
		}

		// verify node shutdown itself.
		err = n.raftnode.Shutdown(canceledctx)
		require.Equal(t, raft.ErrNodeStopped, err)
	}

	otr.teardown()

	// restart leader and prdouce data
	leader.startOpts = nil
	leader.withStartOptions(raft.WithRestart())

	otr.start(leader)
	otr.wait(leader)
	otr.produceData(1)

	v := leader.fsm.Read(1)
	require.Equal(t, 1, v)
}

func TestLearnerMember(t *testing.T) {
	otr := newOrchestrator(t)
	defer otr.teardown()

	nodes := otr.create(2)
	otr.start(nodes...)
	otr.waitAll()

	raw := raft.RawMember{
		ID:      3,
		Address: ":3",
		Type:    raft.LearnerMember,
	}

	learner := newNode().withRawMember(raw).withStartOptions(raft.WithJoin(":2", time.Second))
	otr.start(learner)
	otr.wait(learner)

	// check learner cannt participate.
	err := learner.raftnode.Replicate(canceledctx, newBytesEntry(1, 1))
	require.Error(t, err)
	require.Contains(t, err.Error(), "is a learner not a voter")

	// check learner does not impact cluster availability
	err = learner.raftnode.Shutdown(canceledctx)
	require.NoError(t, err)

	err = otr.leader().raftnode.Replicate(context.Background(), newBytesEntry(1, 1))
	require.NoError(t, err)
}

func TestPromoteMember(t *testing.T) {
	otr := newOrchestrator(t)
	defer otr.teardown()

	nodes := otr.create(2)
	otr.start(nodes...)
	otr.waitAll()

	raw := raft.RawMember{
		ID:      3,
		Address: ":3",
		Type:    raft.LearnerMember,
	}

	learner := newNode().withRawMember(raw).withStartOptions(raft.WithJoin(":1", time.Second))
	otr.start(learner)
	otr.wait(learner)

	// check learner cannt participate.
	err := learner.raftnode.Replicate(canceledctx, newBytesEntry(1, 1))
	require.Error(t, err)
	require.Contains(t, err.Error(), "is a learner not a voter")

	// promote learner,
	err = otr.follower().raftnode.PromoteMember(context.Background(), learner.rawMember().ID)
	require.NoError(t, err)

	// check learner can participate after the promotion.
	err = learner.raftnode.Replicate(canceledctx, newBytesEntry(1, 1))
	require.Error(t, err)
}
