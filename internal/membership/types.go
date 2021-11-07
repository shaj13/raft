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

type Member interface {
	ID() uint64
	Address() string
	ActiveSince() time.Time
	IsActive() bool
	Update(m raftpb.Member) error
	Send(etcdraftpb.Message) error
	Type() raftpb.MemberType
	Raw() raftpb.Member
	Close() error
	TearDown(ctx context.Context) error
}

type Reporter interface {
	ReportUnreachable(id uint64)
	ReportShutdown(id uint64)
	ReportSnapshot(id uint64, status raft.SnapshotStatus)
}

type Config interface {
	Context() context.Context
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
	RegisterTypeMatcher(fn func(m raftpb.Member) raftpb.MemberType)
	TearDown(ctx context.Context) error
}
