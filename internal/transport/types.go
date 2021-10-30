package transport

import (
	"context"

	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/storage"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

//go:generate mockgen -package transportmock  -source internal/transport/types.go -destination internal/mocks/transport/transport.go

// HandlerConfig define common configuration used by the NewHandler function.
type HandlerConfig interface {
	DialerConfig
	Controller() Controller
}

// DialerConfig define common configuration used by the dial function.
type DialerConfig interface {
	Snapshotter() storage.Snapshotter
}

// Handler responds to an RPC request.
type Handler interface{}

// Dialer return's Dial from the given config.
type Dialer func(context.Context, DialerConfig) Dial

// Dial connects to an RPC server at the specified network address.
type Dial func(context.Context, string) (Client, error)

// NewHandler returns a new Handler.
type NewHandler func(context.Context, HandlerConfig) Handler

// Client provides access to the exported methods of an object across a network.
//
//go:generate mockgen -package mocks  -source internal/rpc/types.go -destination internal/mocks/rpc.go
type Client interface {
	Message(context.Context, etcdraftpb.Message) error
	Join(context.Context, raftpb.Member) (uint64, []raftpb.Member, error)
	PromoteMember(ctx context.Context, m raftpb.Member) error
	Close() error
}

// Controller implements operations defined by raft raftpb.
// and acts as a bridge between the RPC and raft daemon.
type Controller interface {
	Push(context.Context, etcdraftpb.Message) error
	Join(context.Context, *raftpb.Member) (uint64, []raftpb.Member, error)
	PromoteMember(ctx context.Context, m raftpb.Member) error
}
