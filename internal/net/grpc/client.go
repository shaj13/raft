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

var _ net.RPC = &client{}

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
	CallOption() []grpc.CallOption
	DialOption() []grpc.DialOption
	Snapshoter() storage.Snapshoter
}

// Dial connects to an GRPC server at the specified network address.
//
// Dial compatible with net.Dial.
func Dial(ctx context.Context, v interface{}, addr string) (net.RPC, error) {
	c := v.(DialConfig)
	conn, err := grpc.DialContext(ctx, addr, c.DialOption()...)
	if err != nil {
		return nil, err
	}

	return &client{
		conn:       conn,
		callOption: c.CallOption(),
		snapshoter: c.Snapshoter(),
	}, nil
}

// Client implements net.RPC.
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
	return fn(ctx, m)
}

func (c *client) Join(ctx context.Context, m api.Member) (uint64, api.Pool, error) {
	fail := func(err error) (uint64, api.Pool, error) {
		return 0, api.Pool{}, err
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

	return id, api.Pool{Members: membs}, nil
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
	name, r, err := c.snapshoter.Reader(ctx, m)
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
