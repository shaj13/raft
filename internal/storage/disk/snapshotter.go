package disk

import (
	"bufio"
	"io"
	"os"
	"path/filepath"

	"github.com/shaj13/raftkit/internal/storage"
	"go.etcd.io/etcd/pkg/v3/fileutil"
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

	r := struct {
		io.Reader
		io.Closer
	}{
		bufio.NewReader(f),
		f,
	}

	return snapshotName(snap.Metadata.Term, snap.Metadata.Index), r, nil
}

func (s snapshotter) Writer(name string) (io.WriteCloser, func() (raftpb.Snapshot, error), error) {
	path := filepath.Join(s.snapdir, name)
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}

	bw := bufio.NewWriter(f)

	w := struct {
		io.Writer
		io.Closer
	}{
		bw,
		f,
	}

	peek := func() (snap raftpb.Snapshot, err error) {
		defer func() {
			if err != nil {
				os.Remove(path)
			}
		}()

		err = bw.Flush()
		if err != nil {
			return
		}

		err = fileutil.Fsync(f)
		if err != nil {
			return
		}

		s, err := peekSnapshot(path)
		if err != nil {
			return
		}

		return s, nil
	}

	return w, peek, nil
}

func (s snapshotter) Write(sf *storage.Snapshot) error {
	return encodeSnapshot(s.path(sf.Raw), sf)
}

func (s snapshotter) Read(snap raftpb.Snapshot) (*storage.Snapshot, error) {
	return decodeSnapshot(s.path(snap))
}

func (s snapshotter) ReadFrom(path string) (*storage.Snapshot, error) {
	return decodeSnapshot(path)
}

func (s snapshotter) path(snap raftpb.Snapshot) string {
	name := snapshotName(snap.Metadata.Term, snap.Metadata.Index)
	return filepath.Join(s.snapdir, name)
}
