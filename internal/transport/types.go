package transport

import (
	"context"
	"io"

	"github.com/franklee0817/raft/internal/raftpb"
	"github.com/franklee0817/raft/raftlog"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

//go:generate mockgen -package transportmock  -source internal/transport/types.go -destination internal/mocks/transport/transport.go

// Config define common configuration used by the dial and transport handler.
type Config interface {
	Controller() Controller
	Logger() raftlog.Logger
	GroupID() uint64
}

// Handler responds to an RPC request.
type Handler interface{}

// Dialer return's Dial from the given config.
type Dialer func(Config) Dial

// Dial connects to an RPC server at the specified network address.
type Dial func(context.Context, string) (Client, error)

// NewHandler returns a new Handler.
type NewHandler func(Config) Handler

// Client provides access to the exported methods of an object across a network.
type Client interface {
	Message(context.Context, etcdraftpb.Message) error
	Join(context.Context, raftpb.Member) (*raftpb.JoinResponse, error)
	PromoteMember(ctx context.Context, m raftpb.Member) error
	Close() error
}

// Controller implements operations defined by raft raftpb.
// and acts as a bridge between the RPC and raft daemon.
type Controller interface {
	Push(context.Context, uint64, etcdraftpb.Message) error
	Join(context.Context, uint64, *raftpb.Member) (*raftpb.JoinResponse, error)
	PromoteMember(context.Context, uint64, raftpb.Member) error
	SnapshotWriter(uint64, uint64, uint64) (io.WriteCloser, error)
	SnapshotReader(uint64, uint64, uint64) (io.ReadCloser, error)
}
