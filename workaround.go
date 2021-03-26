package raft

import (
	"context"

	"github.com/shaj13/raftkit/internal/membership"
	raftrpc "github.com/shaj13/raftkit/internal/net/grpc"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc"
)

type reporter struct {
	reprotc chan report
}

func (r reporter) ReportUnreachable(id uint64) {
	r.reprotc <- report{
		signal: unreachable,
		id:     id,
	}
}
func (reporter) ReportShutdown(id uint64)                             {}
func (reporter) ReportSnapshot(id uint64, status raft.SnapshotStatus) {}

func dial(ctx context.Context, addr string) (membership.Transport, error) {
	cc, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return tr{cc: cc, add: addr}, nil
}

type tr struct {
	cc  *grpc.ClientConn
	add string
}

func (t tr) RoundTrip(ctx context.Context, msg raftpb.Message) error {
	ctx = raftrpc.NewDialContext(ctx, grpc.WithInsecure())
	rpc, err := raftrpc.Dial(ctx, t.add)
	if err != nil {
		return err
	}
	return rpc.Message(ctx, msg)
}

func (t tr) Close() error {
	return t.cc.Close()
}
