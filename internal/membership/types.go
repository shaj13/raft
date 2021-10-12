package membership

import (
	"context"
	"time"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/rpc"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

type constructor func(
	ctx context.Context,
	cfg Config,
	id uint64,
	addr string,
) (Member, error)

type Member interface {
	ID() uint64
	Address() string
	Since() time.Time
	IsActive() bool
	Update(string) error
	Send(raftpb.Message) error
	Type() api.MemberType
	Close()
}

type Reporter interface {
	ReportUnreachable(id uint64)
	ReportShutdown(id uint64)
	ReportSnapshot(id uint64, status raft.SnapshotStatus)
}

type Config interface {
	StreamTimeout() time.Duration
	DrainTimeout() time.Duration
	Reporter() Reporter
	Dial() rpc.Dial
}

type Pool interface {
	NextID() uint64
	Members() []Member
	Get(id uint64) (Member, bool)
	Add(m api.Member) error
	Update(m api.Member) error
	Remove(m api.Member) error
	Snapshot() []api.Member
	Restore(pool api.Pool)
}
