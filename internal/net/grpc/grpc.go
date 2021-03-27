package grpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/net"
	"github.com/shaj13/raftkit/internal/storage"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	snapshotHeader = "x-raft-snapshot-name"
	memberIDHeader = "x-raft-member-id"
)

type Config interface {
	CallOption() []grpc.CallOption
	DialOption() []grpc.DialOption
	Snapshoter() storage.Snapshoter
}

func dial(ctx context.Context, cfg interface{}, addr string) (net.RPC, error) {
	c, ok := cfg.(Config)
	if !ok {
		panic(
			"raft/net/grpc: The given config to dial does not implements grpc config interface",
		)
	}

	conn, err := grpc.DialContext(ctx, addr, c.DialOption()...)
	if err != nil {
		return nil, err
	}

	return &rpc{
		conn:       conn,
		callOption: c.CallOption(),
		snapshoter: c.Snapshoter(),
	}, nil
}

type rpc struct {
	conn       *grpc.ClientConn
	callOption []grpc.CallOption
	snapshoter storage.Snapshoter
}

func (r *rpc) Message(ctx context.Context, m raftpb.Message) error {
	fn := r.message
	if m.Type == raftpb.MsgSnap {
		fn = r.snapshot
	}
	return fn(ctx, m)
}

func (r *rpc) Join(ctx context.Context, m api.Member) (uint64, api.Pool, error) {
	fail := func(err error) (uint64, api.Pool, error) {
		return 0, api.Pool{}, nil
	}

	stream, err := api.NewRaftClient(r.conn).Join(ctx, &m, r.callOption...)
	if err != nil {
		return fail(err)
	}

	md, err := stream.Header()
	if err != nil {
		return fail(err)
	}

	str := md.Get(memberIDHeader)[0]
	id, err := strconv.ParseUint(str, 0, 64)
	if err != nil {
		return fail(
			fmt.Errorf("raft/net/grpc: unable to parse member id from grpc metadata, Err %s", err),
		)
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

	return id, api.Pool{Members: membs}, nil
}

func (r *rpc) Close() error {
	return r.conn.Close()
}

func (r *rpc) message(ctx context.Context, m raftpb.Message) error {
	data, err := m.Marshal()
	if err != nil {
		return err
	}

	stream, err := api.NewRaftClient(r.conn).Message(ctx, r.callOption...)
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
	name, rc, err := r.snapshoter.Reader(ctx, m)
	if err != nil {
		return err
	}

	md := metadata.Pairs(snapshotHeader, name)
	ctx = metadata.NewOutgoingContext(ctx, md)

	stream, err := api.NewRaftClient(r.conn).Message(ctx, r.callOption...)
	if err != nil {
		return err
	}

	defer stream.CloseAndRecv()

	enc := newEncoder(rc)
	return enc.Encode(func(c *api.Chunk) error {
		return stream.Send(c)
	})
}
