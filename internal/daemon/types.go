package daemon

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/shaj13/raftkit/internal/membership"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/storage"
	"github.com/shaj13/raftkit/internal/transport"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

//go:generate mockgen -package daemon  -source internal/daemon/types.go -destination internal/daemon/types_test.go

// Operator is a bootstrapper func that determine the action that is to be performed or considered.
type Operator interface {
	fmt.Stringer
	before(ost *operatorsState) error
	after(ost *operatorsState) error
}

// Config define common configuration used by the daemon.
type Config interface {
	RaftConfig() *raft.Config
	SnapInterval() uint64
	Pool() membership.Pool
	Storage() storage.Storage
	Dial() transport.Dial
	TickInterval() time.Duration
	StateMachine() StateMachine
	Context() context.Context
	DrainTimeout() time.Duration
}

// StateMachine define an interface that must be implemented by
// application to make use of the raft replicated log.
type StateMachine interface {
	// Apply committed raft log entry.
	Apply([]byte)

	// Snapshot is used to write the current state to a snapshot file,
	// on stable storage and compacting the raft logs.
	Snapshot() (io.ReadCloser, error)

	// Restore is used to restore state machine from a snapshot.
	Restore(io.ReadCloser) error
}

type operatorsState struct {
	hasExistingState bool
	local            *raftpb.Member
	membs            []raftpb.Member
	cfg              *raft.Config
	hst              etcdraftpb.HardState
	ents             []etcdraftpb.Entry
	sf               *storage.SnapshotFile
	daemon           *daemon
}
