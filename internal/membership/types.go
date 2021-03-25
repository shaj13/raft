package membership

import (
	"context"
	"time"

	"github.com/shaj13/raftkit/api"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

type constructor func(
	ctx context.Context,
	r reporter,
	cfg config,
	id uint64,
	addr string,
) (Member, error)

type Dial func(ctx context.Context, addr string) (Transport, error)

type Transport interface {
	RoundTrip(ctx context.Context, msg raftpb.Message) error
	Close() error
}

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

type reporter interface {
	ReportUnreachable(id uint64)
	ReportShutdown(id uint64)
	ReportSnapshot(id uint64, status raft.SnapshotStatus)
}

type config interface {
	StreamTimeout() time.Duration
	DrainTimeout() time.Duration
}
