package daemon

import (
	"fmt"
	"time"

	"github.com/shaj13/raftkit/internal/membership"
	"github.com/shaj13/raftkit/internal/rpc"
	"github.com/shaj13/raftkit/internal/storage"
	"go.etcd.io/etcd/raft/v3"
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
