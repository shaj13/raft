package storage

import (
	"io"

	"github.com/shaj13/raftkit/internal/raftpb"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

//go:generate mockgen -package mocks -source internal/storage/types.go -destination internal/mocks/storage.go

type SnapshotFile struct {
	Snap *etcdraftpb.Snapshot
	Pool *raftpb.Pool
	Data io.ReadCloser
}

type Snapshotter interface {
	Reader(etcdraftpb.Snapshot) (string, io.ReadCloser, error)
	Writer(string) (io.WriteCloser, func() (etcdraftpb.Snapshot, error), error)
	Write(sf *SnapshotFile) error
	Read(snap etcdraftpb.Snapshot) (*SnapshotFile, error)
	ReadFromPath(path string) (*SnapshotFile, error)
}

type Storage interface {
	SaveSnapshot(snap etcdraftpb.Snapshot) error
	SaveEntries(st etcdraftpb.HardState, entries []etcdraftpb.Entry) error
	Snapshotter() Snapshotter
	Boot(meta []byte) ([]byte, etcdraftpb.HardState, []etcdraftpb.Entry, *SnapshotFile, error)
	Exist() bool
	Close() error
}
