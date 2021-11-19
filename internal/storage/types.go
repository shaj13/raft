package storage

import (
	"io"

	"github.com/shaj13/raft/internal/raftpb"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

//go:generate mockgen -package storagemock -source internal/storage/types.go -destination internal/mocks/storage/storage.go

type Snapshot struct {
	raftpb.SnapshotState
	Data io.ReadCloser
}

type Snapshotter interface {
	Writer(uint64, uint64) (io.WriteCloser, error)
	Reader(uint64, uint64) (io.ReadCloser, error)
	Write(*Snapshot) error
	Read(uint64, uint64) (*Snapshot, error)
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
