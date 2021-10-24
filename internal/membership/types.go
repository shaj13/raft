package membership

import (
	"context"
	"time"

	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/transport"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

//go:generate mockgen -package membershipmock  -source internal/membership/types.go -destination internal/mocks/membership/membership.go
//go:generate mockgen -package membership  -source internal/membership/types.go -destination internal/membership/types_test.go

type constructor func(
	ctx context.Context,
	cfg Config,
	m raftpb.Member,
) (Member, error)

type Member interface {
	ID() uint64
	Address() string
	ActiveSince() time.Time
	IsActive() bool
	Update(string) error
	Send(etcdraftpb.Message) error
	Type() raftpb.MemberType
	Close() error
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
	Dial() transport.Dial
}

type Pool interface {
	NextID() uint64
	Members() []Member
	Get(id uint64) (Member, bool)
	Add(m raftpb.Member) error
	Update(m raftpb.Member) error
	Remove(m raftpb.Member) error
	Snapshot() []raftpb.Member
	Restore(pool raftpb.Pool)
}
