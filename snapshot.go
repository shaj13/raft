package raft

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gogo/protobuf/proto"
	"github.com/shaj13/raftkit/api"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/wal/walpb"
)

const delim = '\r'

var (
	errNoSnapshot = errors.New("raft: no available snapshot")
)

type snapshotFile struct {
	Snap *raftpb.Snapshot
	Pool *api.Pool
	Data io.ReadCloser
}

type snapshotFileReader struct {
	*bufio.Reader
	f *os.File
}

func (s snapshotFileReader) Close() error {
	return s.f.Close()
}

func readNewestAvailableSnap(dir string, snaps []walpb.Snapshot) (*snapshotFile, error) {
	files := map[string]struct{}{}
	target := ""
	ls, err := list(dir, snapExt)
	if err != nil {
		return nil, err
	}

	for _, name := range ls {
		files[name] = struct{}{}
	}

	for i := len(snaps) - 1; i >= 0; i-- {
		name := fmt.Sprintf(format, snaps[i].Term, snaps[i].Index) + snapExt
		if _, ok := files[name]; ok {
			target = name
			break
		}
	}

	if len(target) == 0 {
		return nil, errNoSnapshot
	}

	return readSnap(filepath.Join(dir, target))
}

func readSnap(path string) (sf *snapshotFile, err error) {
	sf = new(snapshotFile)
	sf.Snap = new(raftpb.Snapshot)
	sf.Pool = new(api.Pool)
	sf.Data, err = readSnapByblocks(path, sf.Snap, sf.Pool)
	return
}

func peekSnap(path string) (*raftpb.Snapshot, error) {
	sf := new(snapshotFile)
	sf.Snap = new(raftpb.Snapshot)
	r, err := readSnapByblocks(path, sf.Snap)
	if err != nil {
		return nil, err
	}
	r.Close()
	return sf.Snap, nil
}

func readSnapByblocks(path string, blocks ...proto.Message) (rc io.ReadCloser, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	r := snapshotFileReader{
		f:      f,
		Reader: bufio.NewReader(f),
	}

	defer func() {
		if err != nil {
			r.Close()
		}
	}()

	for _, b := range blocks {
		data, err := r.ReadBytes(delim)
		if err != nil {
			return nil, err
		}
		if len(data) > 0 {
			data = data[:len(data)-1] // delim
		}
		if err := proto.Unmarshal(data, b); err != nil {
			return nil, err
		}
	}

	return r, nil
}

func writeSnap(path string, s *snapshotFile) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}

	w := bufio.NewWriter(f)
	defer func() {
		if err != nil {
			os.Remove(path)
			return
		}

		w.Flush()
		f.Sync()
		f.Close()
	}()

	blocks := []proto.Message{
		s.Snap,
		s.Pool,
	}

	for _, b := range blocks {
		data, err := proto.Marshal(b)
		if err != nil {
			return err
		}

		if _, err := w.Write(data); err != nil {
			return err
		}

		if err := w.WriteByte(delim); err != nil {
			return err
		}
	}

	_, err = io.Copy(w, s.Data)
	return err
}