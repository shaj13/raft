package grpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"sync"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/net"
	"github.com/shaj13/raftkit/internal/storage"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var _ net.Client = &client{}

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
	Snapshoter() storage.Snapshoter
}

// Dialer return's grpc dialer.
func Dialer(dopts []grpc.DialOption, copts []grpc.CallOption) net.Dialer {
	return func(c context.Context, dc net.DialerConfig) net.Dial {
		return func(ctx context.Context, addr string) (net.Client, error) {
			conn, err := grpc.DialContext(ctx, addr, dopts...)
			if err != nil {
				return nil, err
			}

			return &client{
				conn:       conn,
				callOption: copts,
				snapshoter: dc.(DialConfig).Snapshoter(),
			}, nil
		}
	}
}

// Client implements net.Client.
type client struct {
	conn       *grpc.ClientConn
	callOption []grpc.CallOption
	snapshoter storage.Snapshoter
}

func (c *client) Message(ctx context.Context, m raftpb.Message) error {
	fn := c.message
	if m.Type == raftpb.MsgSnap {
		fn = c.snapshot
	}

	err := fn(ctx, m)
	if err == io.EOF {
		return nil
	}

	return err
}

func (c *client) Join(ctx context.Context, m api.Member) (uint64, []api.Member, error) {
	fail := func(err error) (uint64, []api.Member, error) {
		return 0, nil, err
	}

	stream, err := api.NewRaftClient(c.conn).Join(ctx, &m, c.callOption...)
	if err != nil {
		return fail(err)
	}

	membs := []api.Member{}
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

func (c *client) message(ctx context.Context, m raftpb.Message) (err error) {
	data, err := m.Marshal()
	if err != nil {
		return err
	}

	stream, err := api.NewRaftClient(c.conn).Message(ctx, c.callOption...)
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
	return enc.Encode(func(c *api.Chunk) error {
		return stream.Send(c)
	})
}

func (c *client) snapshot(ctx context.Context, m raftpb.Message) (err error) {
	name, r, err := c.snapshoter.Reader(ctx, m.Snapshot)
	if err != nil {
		return err
	}

	md := metadata.Pairs(
		snapshotHeader, name,
		snapshotHeader, strconv.FormatUint(m.To, 10),
		snapshotHeader, strconv.FormatUint(m.From, 10),
	)
	ctx = metadata.NewOutgoingContext(ctx, md)

	stream, err := api.NewRaftClient(c.conn).Snapshot(ctx, c.callOption...)
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
	return enc.Encode(func(c *api.Chunk) error {
		return stream.Send(c)
	})
}
