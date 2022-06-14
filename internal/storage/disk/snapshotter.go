package disk

import (
	"bufio"
	"io"
	"os"
	"path/filepath"

	"github.com/franklee0817/raft/internal/storage"
)

var _ storage.Snapshotter = &snapshotter{}

type snapshotter struct {
	snapdir string
}

func (s snapshotter) Reader(term uint64, index uint64) (io.ReadCloser, error) {
	path := s.path(term, index)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	r := struct {
		io.Reader
		io.Closer
	}{
		bufio.NewReader(f),
		f,
	}

	return r, nil
}

func (s snapshotter) Writer(term uint64, index uint64) (io.WriteCloser, error) {
	path := s.path(term, index)
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	w := writer{
		bufio.NewWriter(f),
		f,
	}

	return w, nil
}

func (s snapshotter) Write(sf *storage.Snapshot) error {
	path := s.path(sf.Raw.Metadata.Term, sf.Raw.Metadata.Index)
	return encodeSnapshot(path, sf)
}

func (s snapshotter) Read(term uint64, index uint64) (*storage.Snapshot, error) {
	path := s.path(term, index)
	return decodeSnapshot(path)
}

func (s snapshotter) ReadFrom(path string) (*storage.Snapshot, error) {
	return decodeSnapshot(path)
}

func (s snapshotter) path(term uint64, index uint64) string {
	name := snapshotName(term, index)
	return filepath.Join(s.snapdir, name)
}
