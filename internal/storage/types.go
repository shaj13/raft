package storage

import (
	"context"
	"io"

	"github.com/shaj13/raftkit/internal/raftpb"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

type SnapshotFile struct {
	Snap *etcdraftpb.Snapshot
	Pool *raftpb.Pool
	Data io.ReadCloser
}

type Snapshotter interface {
	Reader(context.Context, etcdraftpb.Snapshot) (string, io.ReadCloser, error)
	Writer(context.Context, string) (io.WriteCloser, func() (etcdraftpb.Snapshot, error), error)
	Write(sf *SnapshotFile) error
	Read(snap etcdraftpb.Snapshot) (*SnapshotFile, error)
}

type Storage interface {
	SaveSnapshot(snap etcdraftpb.Snapshot) error
	SaveEntries(st etcdraftpb.HardState, entries []etcdraftpb.Entry) error
	Snapshotter() Snapshotter
	Boot(meta []byte) ([]byte, etcdraftpb.HardState, []etcdraftpb.Entry, *SnapshotFile, error)
	Exist() bool
	Close() error
}
