package rpc

import (
	"context"

	"github.com/shaj13/raftkit/internal/raftpb"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

//go:generate mockgen -package mocks  -source internal/rpc/types.go -destination internal/mocks/rpc.go

// ServerConfig define common configuration used by the NewServer function.
type ServerConfig interface{}

// Dialer define common configuration used by the Dial function.
type DialerConfig interface{}

// Server represents an RPC Server.
type Server interface{}

// Dialer return's Dial from the given config.
type Dialer func(context.Context, DialerConfig) Dial

// Dial connects to an RPC server at the specified network address.
type Dial func(context.Context, string) (Client, error)

// NewServer returns a new Server to handle requests
// to the set of services at the other end of the connection.
type NewServer func(context.Context, ServerConfig) (Server, error)

// Client provides access to the exported methods of an object across a network.
//
//go:generate mockgen -package mocks  -source internal/rpc/types.go -destination internal/mocks/rpc.go
type Client interface {
	Message(context.Context, etcdraftpb.Message) error
	Join(context.Context, raftpb.Member) (uint64, []raftpb.Member, error)
	Close() error
}

// Controller implements operations defined by raft raftpb.
// and acts as a bridge between the RPC and raft daemon.
type Controller interface {
	Push(context.Context, etcdraftpb.Message) error
	Join(context.Context, *raftpb.Member) (uint64, []raftpb.Member, error)
}
