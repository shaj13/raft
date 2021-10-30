package raft

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	daemonmock "github.com/shaj13/raftkit/internal/mocks/daemon"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/raft/v3"
)

func TestNodePreConditions(t *testing.T) {
	// the tests aims to verify that all node method calls the selected pre conditions and in order.
	ctrl := gomock.NewController(t)
	daemon := daemonmock.NewMockDaemon(ctrl)
	daemon.EXPECT().Status().Return(raft.Status{}, nil).AnyTimes()
	ctx := context.TODO()

	table := []struct {
		call     func(n *Node) error
		expected []func(c *Node) error
	}{
		{
			call: func(n *Node) error { return n.LinearizableRead(ctx, 0) },
			expected: []func(c *Node) error{
				joined(),
				noLeader(),
				notType(0, 0),
				available(),
			},
		},
		{
			call: func(n *Node) error { 
				 _, _, err := n.Snapshot()
				 return err  
			},
			expected: []func(c *Node) error{
				joined(),
			},
		},
		{
			call: func(n *Node) error { return n.TransferLeadership(ctx, 0) },
			expected: []func(c *Node) error{
				joined(),
				notMember(0),
				memberRemoved(0),
				noLeader(),
				notType(0, 0),
				disableForwarding(),
				available(),
			},
		},
		{
			call: func(n *Node) error { return n.StepDown(ctx) },
			expected: []func(c *Node) error{
				joined(),
				notLeader(),
				available(),
			},
		},
		{
			call: func(n *Node) error { return n.UpdateMember(ctx, new(RawMember)) },
			expected: []func(c *Node) error{
				joined(),
				notMember(0),
				memberRemoved(0),
				noLeader(),
				notType(0, 0),
				addressInUse(0, ""),
				disableForwarding(),
				available(),
			},
		},
		{
			call: func(n *Node) error { return n.RemoveMember(ctx, 0) },
			expected: []func(c *Node) error{
				joined(),
				notMember(0),
				memberRemoved(0),
				leader(0),
				noLeader(),
				notType(0, 0),
				disableForwarding(),
				available(),
			},
		},
		{
			call: func(n *Node) error { return n.AddMember(ctx, new(RawMember)) },
			expected: []func(c *Node) error{
				joined(),
				idInUse(0),
				addressInUse(0, ""),
				noLeader(),
				notType(0, 0),
				disableForwarding(),
				available(),
			},
		},
		{
			call: func(n *Node) error { return n.PromoteMember(ctx, 0) },
			expected: []func(c *Node) error{
				joined(),
				notMember(0),
				noLeader(),
				notType(0, 0),
				notType(0, 0),
				disableForwarding(),
				available(),
			},
		},
		{
			call: func(n *Node) error { return n.DemoteMember(ctx, 0) },
			expected: []func(c *Node) error{
				joined(),
				notMember(0),
				memberRemoved(0),
				noLeader(),
				leader(0),
				notType(0, 0),
				notType(0, 0),
				disableForwarding(),
				available(),
			},
		},
	}

	for _, tt := range table {
		terr := fmt.Errorf("TestNodePreConditions")
		got := []func(c *Node) error{}
		node := new(Node)
		node.daemon = daemon
		node.exec = func(fns ...func(c *Node) error) error {
			got = fns
			return terr
		}

		err := tt.call(node)
		require.Equal(t, terr, err)
		require.Equal(t, len(tt.expected), len(got))

		for i, fn := range got {
			require.Equal(t, fmt.Sprintf("%p", tt.expected[i]), fmt.Sprintf("%p", fn))
		}
	}
}
