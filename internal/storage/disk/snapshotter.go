package disk

import (
	"io"
	"os"
	"path/filepath"

	"github.com/shaj13/raftkit/internal/storage"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

var _ storage.Snapshotter = &snapshotter{}

type snapshotter struct {
	snapdir string
}

func (s snapshotter) Reader(snap raftpb.Snapshot) (string, io.ReadCloser, error) {
	if raft.IsEmptySnap(snap) {
		return "", nil, ErrEmptySnapshot
	}

	f, err := os.Open(s.path(snap))
	if err != nil {
		return "", nil, err
	}

	r := readerPool.Get().(*fileReader)
	r.Reset(f)

	return snapshotName(snap.Metadata.Term, snap.Metadata.Index), r, nil
}

func (s snapshotter) Writer(name string) (io.WriteCloser, func() (raftpb.Snapshot, error), error) {
	path := filepath.Join(s.snapdir, name)
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}

	w := writerPool.Get().(*fileWriter)
	w.Reset(f, nil)

	peek := func() (raftpb.Snapshot, error) {
		w.FlushAndSync()
		s, err := peekSnapshot(path)
		if err != nil {
			_ = os.Remove(path)
			return raftpb.Snapshot{}, err
		}
		return *s, nil
	}

	return w, peek, nil
}

func (s snapshotter) Write(sf *storage.SnapshotFile) error {
	return encodeSnapshot(s.path(*sf.Snap), sf)
}

func (s snapshotter) Read(snap raftpb.Snapshot) (*storage.SnapshotFile, error) {
	return decodeSnapshot(s.path(snap))
}

func (s snapshotter) ReadFromPath(path string) (*storage.SnapshotFile, error) {
	return decodeSnapshot(path)
}

func (s snapshotter) path(snap raftpb.Snapshot) string {
	name := snapshotName(snap.Metadata.Term, snap.Metadata.Index)
	return filepath.Join(s.snapdir, name)
}
