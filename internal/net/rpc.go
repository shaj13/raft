package net

import (
	"context"
	"io"

	"github.com/shaj13/raftkit/api"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

type snapshoterKey struct{}

// NewSnapshoterContext creates a new context with Snapshoter.
func NewSnapshoterContext(ctx context.Context, sn Snapshoter) context.Context {
	return context.WithValue(ctx, snapshoterKey{}, sn)
}

// SnapshoterFromContextt return's Snapshoter from given ctx.
func SnapshoterFromContextt(ctx context.Context) Snapshoter {
	return ctx.Value(snapshoterKey{}).(Snapshoter)
}

// Dial connects to the RPC address.
type Dial func(ctx context.Context, addr string) (RPC, error)

// RPC provides access to the exported methods of an object across a network.
type RPC interface {
	Message(context.Context, raftpb.Message) error
	Join(context.Context, api.Member) (uint64, api.Pool, error)
	Close() error
}

type Snapshoter interface {
	Reader(raftpb.Message) (string, io.ReadCloser, error)
	Writer(string) (io.Writer, func() (raftpb.Snapshot, error), error)
}
