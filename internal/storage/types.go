package storage

import (
	"context"
	"io"

	"go.etcd.io/etcd/raft/v3/raftpb"
)

type Snapshoter interface {
	Reader(context.Context, raftpb.Message) (string, io.ReadCloser, error)
	Writer(context.Context, string) (io.WriteCloser, func() (raftpb.Snapshot, error), error)
}
