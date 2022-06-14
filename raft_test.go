package raft

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/franklee0817/raft/raftlog"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/raft/v3"
)

func TestConfig(t *testing.T) {
	table := []struct {
		defaults interface{}
		expected interface{}
		opt      Option
		value    func(c *config) interface{}
	}{
		{
			defaults: false,
			expected: true,
			opt:      WithPipelining(),
			value:    func(c *config) interface{} { return c.pipelining },
		},
		{
			defaults: raft.ReadOnlySafe,
			expected: raft.ReadOnlySafe,
			opt:      WithLinearizableReadSafe(),
			value:    func(c *config) interface{} { return c.rcfg.ReadOnlyOption },
		},
		{
			defaults: raftlog.DefaultLogger,
			expected: nil,
			opt:      WithLogger(nil),
			value:    func(c *config) interface{} { return c.logger },
		},
		{
			defaults: raft.ReadOnlySafe,
			expected: raft.ReadOnlyLeaseBased,
			opt:      WithLinearizableReadLeaseBased(),
			value:    func(c *config) interface{} { return c.rcfg.ReadOnlyOption },
		},
		{
			defaults: context.Background(),
			expected: context.TODO(),
			opt:      WithContext(context.TODO()),
			value:    func(c *config) interface{} { return c.ctx },
		},
		{
			defaults: time.Millisecond * 100,
			expected: time.Nanosecond * 500,
			opt:      WithTickInterval(time.Nanosecond * 500),
			value:    func(c *config) interface{} { return c.TickInterval() },
		},
		{
			defaults: time.Second * 10,
			expected: time.Nanosecond * 500,
			opt:      WithStreamTimeOut(time.Nanosecond * 500),
			value:    func(c *config) interface{} { return c.StreamTimeout() },
		},
		{
			defaults: time.Second * 10,
			expected: time.Nanosecond * 500,
			opt:      WithDrainTimeOut(time.Nanosecond * 500),
			value:    func(c *config) interface{} { return c.DrainTimeout() },
		},
		{
			defaults: os.TempDir(),
			expected: "/var/lib",
			opt:      WithStateDIR("/var/lib"),
			value:    func(c *config) interface{} { return c.StateDir() },
		},
		{
			defaults: 5,
			expected: 10,
			opt:      WithMaxSnapshotFiles(10),
			value:    func(c *config) interface{} { return c.MaxSnapshotFiles() },
		},
		{
			defaults: uint64(1000),
			expected: uint64(2000),
			opt:      WithSnapshotInterval(2000),
			value:    func(c *config) interface{} { return c.SnapInterval() },
		},
		{
			defaults: 10,
			expected: 100,
			opt:      WithElectionTick(100),
			value:    func(c *config) interface{} { return c.rcfg.ElectionTick },
		},
		{
			defaults: 1,
			expected: 100,
			opt:      WithHeartbeatTick(100),
			value:    func(c *config) interface{} { return c.rcfg.HeartbeatTick },
		},
		{
			defaults: uint64(1024 * 1024),
			expected: uint64(2024 * 2024),
			opt:      WithMaxSizePerMsg(2024 * 2024),
			value:    func(c *config) interface{} { return c.rcfg.MaxSizePerMsg },
		},
		{
			defaults: uint64(0),
			expected: uint64(5),
			opt:      WithMaxCommittedSizePerReady(5),
			value:    func(c *config) interface{} { return c.rcfg.MaxCommittedSizePerReady },
		},
		{
			defaults: uint64(1 << 30),
			expected: uint64(9),
			opt:      WithMaxUncommittedEntriesSize(9),
			value:    func(c *config) interface{} { return c.rcfg.MaxUncommittedEntriesSize },
		},
		{
			defaults: 256,
			expected: 20,
			opt:      WithMaxInflightMsgs(20),
			value:    func(c *config) interface{} { return c.rcfg.MaxInflightMsgs },
		},
		{
			defaults: false,
			expected: true,
			opt:      WithCheckQuorum(),
			value:    func(c *config) interface{} { return c.rcfg.CheckQuorum },
		},
		{
			defaults: false,
			expected: true,
			opt:      WithPreVote(),
			value:    func(c *config) interface{} { return c.rcfg.PreVote },
		},
		{
			defaults: false,
			expected: true,
			opt:      WithDisableProposalForwarding(),
			value:    func(c *config) interface{} { return c.rcfg.DisableProposalForwarding },
		},
	}

	for _, tt := range table {
		c1 := newConfig()
		require.Equal(t, tt.defaults, tt.value(c1))

		c2 := newConfig(tt.opt)
		require.Equal(t, tt.expected, tt.value(c2))
	}
}

func TestStartConfig(t *testing.T) {
	table := []struct {
		expected string
		opt      StartOption
	}{
		{expected: "raftengine.join", opt: WithJoin("", 0)},
		{expected: "raftengine.forceJoin", opt: WithForceJoin("", 0)},
		{expected: "raftengine.initCluster", opt: WithInitCluster()},
		{expected: "raftengine.forceNewCluster", opt: WithForceNewCluster()},
		{expected: "raftengine.restart", opt: WithRestart()},
		{expected: "*raftengine.fallback", opt: WithFallback()},
		{expected: "raftengine.restore", opt: WithRestore("")},
		{expected: "raftengine.members", opt: WithMembers()},
	}

	for _, tt := range table {
		c := new(startConfig)
		c.apply(tt.opt)
		require.GreaterOrEqual(t, 1, len(c.operators))
		require.Equal(t, tt.expected, fmt.Sprintf("%T", c.operators[0]))
	}
}

func TestWithAddress(t *testing.T) {
	addr := ":TestWithAddress:"
	opt := WithAddress(addr)
	c := new(startConfig)
	opt.apply(c)
	require.Equal(t, addr, c.addr)
}
