package net

import (
	"context"

	"github.com/shaj13/raftkit/api"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

// Server represents an RPC Server.
type Server interface{}

// Dial connects to an RPC server at the specified network address.
type Dial func(ctx context.Context, addr string) (RPC, error)

// New returns a new Server to handle requests
// to the set of services at the other end of the connection.
type New func(ctx context.Context, cfg interface{}) (Server, error)

// RPC provides access to the exported methods of an object across a network.
type RPC interface {
	Message(context.Context, raftpb.Message) error
	Join(context.Context, api.Member) (uint64, api.Pool, error)
	Close() error
}

// Controller implements operations defined by raft API.
// and acts as a bridge between the RPC and raft daemon.
type Controller interface {
	Push(context.Context, raftpb.Message) error
	Join(context.Context, *api.Member) (uint64, []api.Member, error)
}
