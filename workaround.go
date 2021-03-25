package raft

import (
	"bufio"
	"bytes"
	"context"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/membership"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc"
)

type reporter struct{}

func (reporter) ReportUnreachable(id uint64)                          {}
func (reporter) ReportShutdown(id uint64)                             {}
func (reporter) ReportSnapshot(id uint64, status raft.SnapshotStatus) {}

func dial(ctx context.Context, addr string) (membership.Transport, error) {
	cc, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return tr{cc}, nil
}

type tr struct {
	cc *grpc.ClientConn
}

func (t tr) RoundTrip(ctx context.Context, msg raftpb.Message) error {
	cc := t.cc
	buf := mustMarshal(&msg)
	st := new(streamer)
	st.scanner = bufio.NewScanner(bytes.NewBuffer(buf))
	st.scanner.Split(st.scan)

	stream, err := api.NewRaftClient(cc).Message(ctx)
	if err != nil {
		return err
	}

	defer stream.CloseAndRecv()

	for st.Next() {
		c := st.Chunck()
		if err := stream.Send(c); err != nil {
			return err
		}
	}

	return nil
}

func (t tr) Close() error {
	return t.cc.Close()
}
