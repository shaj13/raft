package grpc

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"sync"

	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/storage"
	"github.com/shaj13/raftkit/internal/transport"
	"github.com/shaj13/raftkit/internal/transport/grpc/pb"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var _ transport.Client = &client{}

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

const (
	snapshotHeader = "X-Raft-Snapshot"
	memberIDHeader = "X-Raft-Member-ID"
)

// Dialer return's grpc dialer.
func Dialer(dopts func(context.Context) []grpc.DialOption, copts func(context.Context) []grpc.CallOption) transport.Dialer {
	return func(dc transport.DialerConfig) transport.Dial {
		return func(ctx context.Context, addr string) (transport.Client, error) {
			conn, err := grpc.DialContext(ctx, addr, dopts(ctx)...)
			if err != nil {
				return nil, err
			}

			return &client{
				conn:    conn,
				copts:   copts,
				shotter: dc.Snapshotter(),
			}, nil
		}
	}
}

// Client implements transport.Client.
type client struct {
	conn    *grpc.ClientConn
	copts   func(context.Context) []grpc.CallOption
	shotter storage.Snapshotter
}

func (c *client) PromoteMember(ctx context.Context, m raftpb.Member) error {
	_, err := pb.NewRaftClient(c.conn).PromoteMember(ctx, &m, c.copts(ctx)...)
	return err
}

func (c *client) Message(ctx context.Context, msg etcdraftpb.Message) error {
	fn := c.message
	if msg.Type == etcdraftpb.MsgSnap {
		fn = c.snapshot
	}

	err := fn(ctx, msg)
	if err == io.EOF {
		return nil
	}

	return err
}

func (c *client) Join(ctx context.Context, m raftpb.Member) (*raftpb.JoinResponse, error) {
	return pb.NewRaftClient(c.conn).Join(ctx, &m, c.copts(ctx)...)
}

func (c *client) Close() error {
	return c.conn.Close()
}

func (c *client) message(ctx context.Context, msg etcdraftpb.Message) (err error) {
	data, err := msg.Marshal()
	if err != nil {
		return err
	}

	stream, err := pb.NewRaftClient(c.conn).Message(ctx, c.copts(ctx)...)
	if err != nil {
		return err
	}

	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	buf.Write(data)

	defer func() {
		bufferPool.Put(buf)
		_, rerr := stream.CloseAndRecv()
		if err == nil {
			err = rerr
		}
	}()

	enc := newEncoder(buf)
	return enc.Encode(func(c *pb.Chunk) error {
		return stream.Send(c)
	})
}

func (c *client) snapshot(ctx context.Context, msg etcdraftpb.Message) (err error) {
	meta := msg.Snapshot.Metadata
	r, err := c.shotter.Reader(meta.Term, meta.Index)
	if err != nil {
		return err
	}

	md := metadata.Pairs(
		snapshotHeader, strconv.FormatUint(meta.Term, 10),
		snapshotHeader, strconv.FormatUint(meta.Index, 10),
	)
	ctx = metadata.NewOutgoingContext(ctx, md)

	stream, err := pb.NewRaftClient(c.conn).Snapshot(ctx, c.copts(ctx)...)
	if err != nil {
		return err
	}

	defer func() {
		_, rerr := stream.CloseAndRecv()
		if err == nil {
			err = rerr
		}
	}()

	enc := newEncoder(r)
	err = enc.Encode(func(c *pb.Chunk) error {
		return stream.Send(c)
	})

	if err != nil {
		return
	}

	return c.message(ctx, msg)
}
