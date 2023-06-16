package rafttest_test

import (
	"context"
	"testing"
	"time"

	raft "github.com/rakoo/raft"
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

	joinAddr := nodes[1].rawMembers[0].Address
	raw := raft.RawMember{
		ID:      3,
		Address: ":3",
		Type:    raft.LearnerMember,
	}

	learner := newNode().withRawMember(raw).withStartOptions(raft.WithJoin(joinAddr, time.Second))
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

	joinAddr := nodes[0].rawMembers[0].Address

	raw := raft.RawMember{
		ID:      3,
		Address: ":3",
		Type:    raft.LearnerMember,
	}

	learner := newNode().withRawMember(raw).withStartOptions(raft.WithJoin(joinAddr, time.Second))
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

func TestDemoteMember(t *testing.T) {
	otr := newOrchestrator(t)
	defer otr.create(2)

	nodes := otr.create(2)
	otr.start(nodes...)
	otr.waitAll()

	follower := otr.follower()

	// check follower can participate before the demoteion.
	err := follower.raftnode.Replicate(context.Background(), newBytesEntry(1, 1))
	require.NoError(t, err)

	err = follower.raftnode.DemoteMember(context.Background(), follower.rawMember().ID)
	require.NoError(t, err)

	// check follower cannt participate after the demoteion.
	err = follower.raftnode.Replicate(context.Background(), newBytesEntry(1, 1))
	require.Error(t, err)
	require.Contains(t, err.Error(), "is a learner not a voter")
}

func TestLeaderStepDown(t *testing.T) {
	otr := newOrchestrator(t)
	defer otr.teardown()

	nodes := otr.create(3)
	otr.start(nodes...)
	otr.waitAll()

	leader := otr.leader()
	err := leader.raftnode.Stepdown(context.Background())
	require.NoError(t, err)

	newLeader := otr.leader()
	require.NotEqual(t, leader.rawMember().ID, newLeader.rawMember().ID)
}

func TestTransferLeadership(t *testing.T) {
	otr := newOrchestrator(t)
	defer otr.teardown()

	nodes := otr.create(3)
	otr.start(nodes...)
	otr.waitAll()

	leader := otr.leader()
	followerID := otr.follower().rawMember().ID

	err := leader.raftnode.TransferLeadership(context.Background(), followerID)
	require.NoError(t, err)

	leaderID := otr.leader().rawMember().ID
	require.NotEqual(t, leader.rawMember().ID, leaderID)
	require.Equal(t, followerID, leaderID)
}

func TestStagingMember(t *testing.T) {
	otr := newOrchestrator(t)
	defer otr.teardown()

	nodes := otr.create(1)
	otr.start(nodes...)
	otr.waitAll()
	otr.produceData(100)

	joinAddr := nodes[0].rawMembers[0].Address
	raw := raft.RawMember{
		ID:      3,
		Address: ":3",
		Type:    raft.StagingMember,
	}

	staging := newNode().withRawMember(raw).withStartOptions(raft.WithJoin(joinAddr, time.Second))
	otr.start(staging)
	otr.wait(staging)

	promoted := false
	for i := 1; i < 5; i++ {
		mem, _ := nodes[0].raftnode.GetMemebr(raw.ID)
		if mem.Type() == raft.VoterMember {
			promoted = true
			break
		}
		time.Sleep(time.Millisecond * 500)
	}

	if !promoted {
		t.Fatal("expected stagingMember to be auto promoted")
	}

	// verify StagingMember fsm same as leader fsm.
	for i := 0; i < 100; i++ {
		v := staging.fsm.Read(i)
		require.Equal(t, i, v)
	}
}
