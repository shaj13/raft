package daemon

import (
	"fmt"
	"time"

	"github.com/shaj13/raftkit/internal/membership"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/rpc"
	"github.com/shaj13/raftkit/internal/storage"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

// go:generate mockgen -package daemon  -source internal/daemon/types.go -destination internal/daemon/types_test.go

// Operator is a bootstrapper func that determine the action that is to be performed or considered.
type Operator interface {
	fmt.Stringer
	before(d *daemon) error
	after(d *daemon) error
}

type Config interface {
	RaftConfig() *raft.Config
	SnapInterval() uint64
	Pool() membership.Pool
	Storage() storage.Storage
	Dial() rpc.Dial
	TickInterval() time.Duration
}

type operatorsState struct {
	wasExisted bool
	local      *raftpb.Member
	membs      []raftpb.Member
	cfg        *raft.Config
	hst        etcdraftpb.HardState
	ents       []etcdraftpb.Entry
	sf         *storage.SnapshotFile
}
