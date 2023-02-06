package inmemory

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/shaj13/raft/internal/raftpb"
	"github.com/shaj13/raft/internal/storage"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

// New creates an in-memory storage backed by an bytes.Buffer.
func New() storage.Storage {
	return &inmemory{
		buf: bytes.NewBuffer(nil),
	}
}

type inmemory struct {
	mu    sync.Mutex
	state raftpb.SnapshotState
	buf   *bytes.Buffer
}

func (mem *inmemory) Writer(uint64, uint64) (io.WriteCloser, error) {
	mem.mu.Lock()
	mem.buf.Reset()

	return &inmemoryWriter{
		Buffer: mem.buf,
		fn:     mem.mu.Unlock, // Unlock memory on close.
	}, nil
}

func (mem *inmemory) Write(s *storage.Snapshot) error {
	mem.mu.Lock()
	defer mem.mu.Unlock()

	mem.buf.Reset()
	_, err := io.Copy(mem.buf, s.Data)
	return err
}

func (mem *inmemory) Reader(term uint64, index uint64) (io.ReadCloser, error) {
	s, err := mem.Read(term, index)
	if err != nil {
		return nil, err
	}

	return s.Data, nil
}

func (mem *inmemory) Read(_ uint64, index uint64) (*storage.Snapshot, error) {
	mem.mu.Lock()
	defer mem.mu.Unlock()

	if mem.state.Raw.Metadata.Index != index {
		return nil, fmt.Errorf("raft: snapshot %d not found", index)
	}

	buf := bytes.NewBuffer(nil)
	buf.Write(mem.buf.Bytes())

	return &storage.Snapshot{
		SnapshotState: mem.state,
		Data:          io.NopCloser(buf),
	}, nil
}

func (mem *inmemory) SaveSnapshot(snap etcdraftpb.Snapshot) error {
	mem.mu.Lock()
	defer mem.mu.Unlock()

	mem.state.Raw = snap
	return nil
}

func (mem *inmemory) Boot(meta []byte) ([]byte, etcdraftpb.HardState, []etcdraftpb.Entry, *storage.Snapshot, error) {
	return meta, etcdraftpb.HardState{}, nil, nil, nil
}

func (mem *inmemory) ReadFrom(string) (*storage.Snapshot, error) {
	return nil, errors.New("raft: in-memory snapshotter operation not supported")
}

func (mem *inmemory) Snapshotter() storage.Snapshotter                                 { return mem }
func (mem *inmemory) SaveEntries(etcdraftpb.HardState, []etcdraftpb.Entry) (err error) { return }
func (mem *inmemory) Exist() (ok bool)                                                 { return }
func (mem *inmemory) Close() (err error)                                               { return }

type inmemoryWriter struct {
	*bytes.Buffer
	fn func()
}

func (w *inmemoryWriter) Close() error {
	w.fn()
	return nil
}
