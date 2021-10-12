package grpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"sync"

	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/rpc"
	"github.com/shaj13/raftkit/internal/storage"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var _ rpc.Client = &client{}

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

const (
	snapshotHeader = "X-Raft-Snapshot"
	memberIDHeader = "X-Raft-Member-ID"
)

// DialConfig define common configuration used by the dial function.
type DialConfig interface {
	Snapshotter() storage.Snapshotter
}

// Dialer return's grpc dialer.
func Dialer(dopts func(context.Context) []grpc.DialOption, copts func(context.Context) []grpc.CallOption) rpc.Dialer {
	return func(c context.Context, dc rpc.DialerConfig) rpc.Dial {
		return func(ctx context.Context, addr string) (rpc.Client, error) {
			conn, err := grpc.DialContext(ctx, addr, dopts(ctx)...)
			if err != nil {
				return nil, err
			}

			return &client{
				conn:    conn,
				copts:   copts,
				shotter: dc.(DialConfig).Snapshotter(),
			}, nil
		}
	}
}

// Client implements rpc.Client.
type client struct {
	conn    *grpc.ClientConn
	copts   func(context.Context) []grpc.CallOption
	shotter storage.Snapshotter
}

func (c *client) Message(ctx context.Context, m etcdraftpb.Message) error {
	fn := c.message
	if m.Type == etcdraftpb.MsgSnap {
		fn = c.snapshot
	}

	err := fn(ctx, m)
	if err == io.EOF {
		return nil
	}

	return err
}

func (c *client) Join(ctx context.Context, m raftpb.Member) (uint64, []raftpb.Member, error) {
	fail := func(err error) (uint64, []raftpb.Member, error) {
		return 0, nil, err
	}

	stream, err := raftpb.NewRaftClient(c.conn).Join(ctx, &m, c.copts(ctx)...)
	if err != nil {
		return fail(err)
	}

	membs := []raftpb.Member{}
	for {
		m, err := stream.Recv()

		if err == io.EOF {
			break
		}

		if err != nil {
			return fail(err)
		}

		membs = append(membs, *m)
	}

	md, err := stream.Header()
	if err != nil {
		return fail(err)
	}

	vals := md.Get(memberIDHeader)
	if len(vals) != 1 {
		return fail(
			fmt.Errorf("raft/net/grpc: member id missing from grpc metadata"),
		)
	}

	id, err := strconv.ParseUint(vals[0], 0, 64)
	if err != nil {
		return fail(
			fmt.Errorf("raft/net/grpc: unable to parse member id from grpc metadata, Err %s", err),
		)
	}

	return id, membs, nil
}

func (c *client) Close() error {
	return c.conn.Close()
}

func (c *client) message(ctx context.Context, m etcdraftpb.Message) (err error) {
	data, err := m.Marshal()
	if err != nil {
		return err
	}

	stream, err := raftpb.NewRaftClient(c.conn).Message(ctx, c.copts(ctx)...)
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
	return enc.Encode(func(c *raftpb.Chunk) error {
		return stream.Send(c)
	})
}

func (c *client) snapshot(ctx context.Context, m etcdraftpb.Message) (err error) {
	name, r, err := c.shotter.Reader(ctx, m.Snapshot)
	if err != nil {
		return err
	}

	md := metadata.Pairs(
		snapshotHeader, name,
		snapshotHeader, strconv.FormatUint(m.To, 10),
		snapshotHeader, strconv.FormatUint(m.From, 10),
	)
	ctx = metadata.NewOutgoingContext(ctx, md)

	stream, err := raftpb.NewRaftClient(c.conn).Snapshot(ctx, c.copts(ctx)...)
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
	return enc.Encode(func(c *raftpb.Chunk) error {
		return stream.Send(c)
	})
}
