package raftengine

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/shaj13/raft/internal/membership"
	"github.com/shaj13/raft/internal/raftpb"
	"github.com/shaj13/raft/internal/storage"
	"github.com/shaj13/raft/internal/transport"
	"github.com/shaj13/raft/raftlog"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

//go:generate mockgen -package raftengine  -source internal/raftengine/types.go -destination internal/raftengine/types_test.go

// Operator is a bootstrapper func that determine the action that is to be performed or considered.
type Operator interface {
	fmt.Stringer
	before(ost *operatorsState) error
	after(ost *operatorsState) error
}

// Config define common configuration used by the daemon.
type Config interface {
	Mux() Mux
	RaftConfig() *raft.Config
	SnapInterval() uint64
	Pool() membership.Pool
	Storage() storage.Storage
	Dial() transport.Dial
	TickInterval() time.Duration
	StateMachine() StateMachine
	Context() context.Context
	DrainTimeout() time.Duration
	GroupID() uint64
	Logger() raftlog.Logger
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

// Mux represents a multi node state that is participating in multiple consensus groups,
// a mux is more efficient than a collection of nodes.
// the name mux stands for "multiplexer". Like the standard "http.ServeMux".
type Mux interface {
	Start()
	Stop()
	add(gid uint64, rn *raft.RawNode, cfg *raft.Config) raft.Node
}

type operatorsState struct {
	hasExistingState bool
	local            *raftpb.Member
	membs            []raftpb.Member
	cfg              *raft.Config
	hst              etcdraftpb.HardState
	ents             []etcdraftpb.Entry
	sf               *storage.Snapshot
	eng              *engine
}

type nodeLogger struct {
	raftlog.Logger
}

func (nl nodeLogger) Debug(v ...interface{}) {
	nl.V(2).Info(v...)
}

func (nl nodeLogger) Debugf(format string, v ...interface{}) {
	nl.V(2).Infof(format, v...)
}
