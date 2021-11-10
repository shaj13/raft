package storage

import (
	"io"

	"github.com/shaj13/raftkit/internal/raftpb"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

//go:generate mockgen -package storagemock -source internal/storage/types.go -destination internal/mocks/storage/storage.go

type Snapshot struct {
	raftpb.SnapshotState
	Data io.ReadCloser
}

type Snapshotter interface {
	Reader(etcdraftpb.Snapshot) (string, io.ReadCloser, error)
	Writer(string) (io.WriteCloser, func() (etcdraftpb.Snapshot, error), error)
	Write(*Snapshot) error
	Read(etcdraftpb.Snapshot) (*Snapshot, error)
	ReadFrom(string) (*Snapshot, error)
}

type Storage interface {
	SaveSnapshot(etcdraftpb.Snapshot) error
	SaveEntries(etcdraftpb.HardState, []etcdraftpb.Entry) error
	Snapshotter() Snapshotter
	Boot([]byte) ([]byte, etcdraftpb.HardState, []etcdraftpb.Entry, *Snapshot, error)
	Exist() bool
	Close() error
}
