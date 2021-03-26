package grpc

import (
	"bytes"
	"context"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/net"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	snapshotHeader = "x-raft-snapshot-name"
)

type (
	dialKey struct{}
	callKey struct{}
)

// NewDialContext creates a new context with grpc.DialOption's.
func NewDialContext(ctx context.Context, opts ...grpc.DialOption) context.Context {
	return context.WithValue(ctx, dialKey{}, opts)
}

// NewCallContext creates a new context with grpc.CallOption's.
func NewCallContext(ctx context.Context, opts ...grpc.CallOption) context.Context {
	return context.WithValue(ctx, callKey{}, opts)
}

func dialFromContext(ctx context.Context) []grpc.DialOption {
	opts, ok := ctx.Value(dialKey{}).([]grpc.DialOption)
	if !ok {
		opts = []grpc.DialOption{}
	}
	return opts
}

func callFromContext(ctx context.Context) []grpc.CallOption {
	opts, ok := ctx.Value(callKey{}).([]grpc.CallOption)
	if !ok {
		opts = []grpc.CallOption{}
	}
	return opts
}

// Dial creates a RPC connection to the given address.
func Dial(ctx context.Context, addr string) (net.RPC, error) {
	opts := dialFromContext(ctx)
	conn, err := grpc.DialContext(ctx, addr, opts...)
	if err != nil {
		return nil, err
	}
	return &rpc{conn: conn}, nil
}

type rpc struct {
	conn *grpc.ClientConn
}

func (r *rpc) Message(ctx context.Context, m raftpb.Message) error {
	fn := r.message
	if m.Type == raftpb.MsgSnap {
		fn = r.snapshot
	}
	return fn(ctx, m)
}

func (r *rpc) Join(ctx context.Context, m api.Member) (uint64, api.Pool, error) {
	return 0, api.Pool{}, nil
}

func (r *rpc) Close() error {
	return r.conn.Close()
}

func (r *rpc) message(ctx context.Context, m raftpb.Message) error {
	opts := callFromContext(ctx)
	data, err := m.Marshal()
	if err != nil {
		return err
	}

	stream, err := api.NewRaftClient(r.conn).Message(ctx, opts...)
	if err != nil {
		return err
	}

	defer stream.CloseAndRecv()

	buf := bytes.NewBuffer(data)
	enc := newEncoder(buf)
	return enc.Encode(func(c *api.Chunk) error {
		return stream.Send(c)
	})
}

func (r *rpc) snapshot(ctx context.Context, m raftpb.Message) error {
	opts := callFromContext(ctx)
	snapshoter := net.SnapshoterFromContextt(ctx)
	name, rc, err := snapshoter.Reader(m)
	if err != nil {
		return err
	}

	md := metadata.Pairs(snapshotHeader, name)
	ctx = metadata.NewOutgoingContext(ctx, md)

	stream, err := api.NewRaftClient(r.conn).Message(ctx, opts...)
	if err != nil {
		return err
	}

	defer stream.CloseAndRecv()

	enc := newEncoder(rc)
	return enc.Encode(func(c *api.Chunk) error {
		return stream.Send(c)
	})
}
