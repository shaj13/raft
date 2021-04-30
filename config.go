package raft

import (
	"time"

	"github.com/shaj13/raftkit/internal/daemon"
	"github.com/shaj13/raftkit/internal/log"
	"github.com/shaj13/raftkit/internal/membership"
	"github.com/shaj13/raftkit/internal/net"
	"github.com/shaj13/raftkit/internal/storage"
	"go.etcd.io/etcd/raft/v3"
	"google.golang.org/grpc"
)

// Option configures raft library using the functional options paradigm popularized by Rob Pike and Dave Cheney.
// If you're unfamiliar with this style,
// see https://commandcenter.blogspot.com/2014/01/self-referential-functions-and-design.html and
// https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis.
type Option interface {
	apply(c *config)
}

// OptionFunc implements Option interface.
type optionFunc func(c *config)

// Apply the configuration to the provided strategy.
func (fn optionFunc) apply(c *config) {
	fn(c)
}

// WithTickInterval is the time interval to,
// increments the internal logical clock for,
// the current raft member by a single tick.
//
// Default Value: 100'ms.
func WithTickInterval(d time.Duration) Option {
	return optionFunc(func(c *config) {
		c.streamTimeOut = d
	})
}

// WithStreamTimeOut is the timeout on the streaming messages to other raft members.
//
// Default Value: 10's.
func WithStreamTimeOut(d time.Duration) Option {
	return optionFunc(func(c *config) {
		c.streamTimeOut = d
	})
}

// WithDrainTimeOut is the timeout on the streaming pending messages to other raft members.
// Drain can be very useful for graceful shutdown.
//
// Default Value: 10's.
func WithDrainTimeOut(d time.Duration) Option {
	return optionFunc(func(c *config) {
		c.drainTimeOut = d
	})
}

// WithGRPCDialOption configures how we set up the grpc connection.
//
// Default Value: no defaults.
func WithGRPCDialOption(opts ...grpc.DialOption) Option {
	return optionFunc(func(c *config) {
		c.dialOptions = opts
	})
}

// WithStateDIR is the directory to store durable state (WAL logs and Snapshots).
//
// Default Value: /tmp.
func WithStateDIR(dir string) Option {
	return optionFunc(func(c *config) {
		c.statedir = dir
	})
}

// WithMaxSnapshotFiles is the number of snapshots to keep beyond the
// current snapshot.
//
// Default Value: 5.
func WithMaxSnapshotFiles(max int) Option {
	return optionFunc(func(c *config) {
		c.maxSnapshotFiles = max
	})
}

// WithSnapshotInterval is the number of log entries between snapshots.
//
// Default Value: 1000.
func WithSnapshotInterval(i uint64) Option {
	return optionFunc(func(c *config) {
		c.snapInterval = i
	})
}

// WithElectionTick is the number of Node.Tick invocations that must pass between
// elections. That is, if a follower does not receive any message from the
// leader of current term before ElectionTick has elapsed, it will become
// candidate and start an election. ElectionTick must be greater than
// HeartbeatTick. We suggest ElectionTick = 10 * HeartbeatTick to avoid
// unnecessary leader switching.
//
// Default Value: 10.
func WithElectionTick(tick int) Option {
	return optionFunc(func(c *config) {
		c.rcfg.ElectionTick = tick
	})
}

// WithHeartbeatTick is the number of Node.Tick invocations that must pass between
// heartbeats. That is, a leader sends heartbeat messages to maintain its
// leadership every HeartbeatTick ticks.
//
// Default Value: 1.
func WithHeartbeatTick(tick int) Option {
	return optionFunc(func(c *config) {
		c.rcfg.HeartbeatTick = tick
	})
}

// WithMaxSizePerMsg limits the max byte size of each append message. Smaller
// value lowers the raft recovery cost(initial probing and message lost
// during normal operation). On the other side, it might affect the
// throughput during normal replication. Note: math.MaxUint64 for unlimited,
// 0 for at most one entry per message.
//
// Default Value: 1024 * 1024.
func WithMaxSizePerMsg(max uint64) Option {
	return optionFunc(func(c *config) {
		c.rcfg.MaxSizePerMsg = max
	})
}

// WithMaxCommittedSizePerReady limits the size of the committed entries which
// can be applied.
//
// Default Value: 0.
func WithMaxCommittedSizePerReady(max uint64) Option {
	return optionFunc(func(c *config) {
		c.rcfg.MaxCommittedSizePerReady = max
	})
}

// WithMaxUncommittedEntriesSize limits the aggregate byte size of the
// uncommitted entries that may be appended to a leader's log. Once this
// limit is exceeded, proposals will begin to return ErrProposalDropped
// errors. Note: 0 for no limit.
//
// Default Value: 1 << 30.
func WithMaxUncommittedEntriesSize(max uint64) Option {
	return optionFunc(func(c *config) {
		c.rcfg.MaxUncommittedEntriesSize = max
	})
}

// WithMaxInflightMsgs limits the max number of in-flight append messages during
// optimistic replication phase. The application transportation layer usually
// has its own sending buffer over TCP/UDP. Setting MaxInflightMsgs to avoid
// overflowing that sending buffer.
//
// Default Value: 256.
func WithMaxInflightMsgs(max int) Option {
	return optionFunc(func(c *config) {
		c.rcfg.MaxInflightMsgs = max
	})
}

// WithCheckQuorum specifies if the leader should check quorum activity. Leader
// steps down when quorum is not active for an electionTimeout.
//
// Default Value: false.
func WithCheckQuorum(check bool) Option {
	return optionFunc(func(c *config) {
		c.rcfg.CheckQuorum = check
	})
}

// WithPreVote enables the Pre-Vote algorithm described in raft thesis section
// 9.6. This prevents disruption when a node that has been partitioned away
// rejoins the cluster.
//
// Default Value: false.
func WithPreVote(preVote bool) Option {
	return optionFunc(func(c *config) {
		c.rcfg.PreVote = preVote
	})
}

// WithDisableProposalForwarding set to true means that followers will drop
// proposals, rather than forwarding them to the leader. One use case for
// this feature would be in a situation where the Raft leader is used to
// compute the data of a proposal, for example, adding a timestamp from a
// hybrid logical clock to data in a monotonically increasing way. Forwarding
// should be disabled to prevent a follower with an inaccurate hybrid
// logical clock from assigning the timestamp and then forwarding the data
// to the leader.
//
// Default Value: false.
func WithDisableProposalForwarding(disable bool) Option {
	return optionFunc(func(c *config) {
		c.rcfg.DisableProposalForwarding = disable
	})
}

type config struct {
	rcfg             *raft.Config
	tickInterval     time.Duration
	streamTimeOut    time.Duration
	drainTimeOut     time.Duration
	dialOptions      []grpc.DialOption
	statedir         string
	maxSnapshotFiles int
	snapInterval     uint64
	controller       net.Controller
	storage          storage.Storage
	pool             membership.Pool
	dial             net.Dial
	daemon           daemon.Daemon
}

func (c *config) TickInterval() time.Duration {
	return c.TickInterval()
}

func (c *config) StreamTimeout() time.Duration {
	return c.streamTimeOut
}

func (c *config) DrainTimeout() time.Duration {
	return c.drainTimeOut
}

func (c *config) CallOption() []grpc.CallOption {
	return []grpc.CallOption{}
}

func (c *config) DialOption() []grpc.DialOption {
	return []grpc.DialOption{grpc.WithInsecure()}
}

func (c *config) Snapshoter() storage.Snapshoter {
	return c.storage.Snapshoter()
}

func (c *config) StateDir() string {
	return c.statedir
}

func (c *config) MaxSnapshotFiles() int {
	return c.maxSnapshotFiles
}

func (c *config) Controller() net.Controller {
	return c.controller
}

func (c *config) Storage() storage.Storage {
	return c.storage
}

func (c *config) SnapInterval() uint64 {
	return c.snapInterval
}

func (c *config) RaftConfig() *raft.Config {
	return c.rcfg
}

func (c *config) Pool() membership.Pool {
	return c.pool
}

func (c *config) Dial() net.Dial {
	return c.dial
}

func (c *config) Reporter() membership.Reporter {
	return c.daemon
}

func newConfig(opts ...Option) *config {
	c := &config{
		rcfg: &raft.Config{
			Logger:                    log.Get(),
			ElectionTick:              10,
			HeartbeatTick:             1,
			MaxSizePerMsg:             1024 * 1024,
			MaxInflightMsgs:           256,
			MaxUncommittedEntriesSize: 1 << 30,
		},
		tickInterval:     time.Millisecond * 100,
		streamTimeOut:    time.Second * 10,
		drainTimeOut:     time.Second * 10,
		maxSnapshotFiles: 5,
		snapInterval:     1000,
		statedir:         "/tmp",
		dialOptions: []grpc.DialOption{
			grpc.WithInsecure(),
		},
	}

	for _, opt := range opts {
		opt.apply(c)
	}

	return c
}
