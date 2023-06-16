package disk

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc64"
	"io"
	"os"
	"path/filepath"

	"github.com/rakoo/raft/internal/raftpb"
	"github.com/rakoo/raft/internal/storage"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/wal/walpb"
)

var crcTable = crc64.MakeTable(crc64.ECMA)

var (
	errSnapshotFormat = errors.New("raft/storage: invalid snapshot file format")
	errCRCMismatch    = errors.New("raft/storage: snapshot file corrupted, crc mismatch")
	errNoSnapshot     = errors.New("raft/storage: no available snapshot")
)

func snapshotName(term, index uint64) string {
	return fmt.Sprintf(format, term, index) + snapExt
}

func decodeNewestAvailableSnapshot(dir string, snaps []walpb.Snapshot) (*storage.Snapshot, error) {
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
		name := snapshotName(snaps[i].Term, snaps[i].Index)
		if _, ok := files[name]; ok {
			target = name
			break
		}
	}

	if len(target) == 0 {
		return nil, errNoSnapshot
	}

	return decodeSnapshot(filepath.Join(dir, target))
}

func peekSnapshot(path string) (etcdraftpb.Snapshot, error) {
	sf, err := decodeSnapshot(path)
	if err != nil {
		return etcdraftpb.Snapshot{}, err
	}

	defer sf.Data.Close()

	return sf.Raw, nil
}

func encodeSnapshot(path string, s *storage.Snapshot) error {
	pathtmp := path + ".tmp"

	f, err := os.Create(pathtmp)
	if err != nil {
		return err
	}

	fw := writer{
		bufio.NewWriter(f),
		f,
	}
	crc := crc64.New(crcTable)
	w := io.MultiWriter(crc, fw)

	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(pathtmp)
			return
		}

		err = fw.Close()
		if err != nil {
			return
		}

		err = os.Rename(pathtmp, path)
	}()

	_, err = io.Copy(w, s.Data)
	if err != nil {
		return err
	}

	s.CRC = crc.Sum(nil)
	s.Version = raftpb.V0

	buf, err := s.Marshal()
	if err != nil {
		return err
	}

	_, err = fw.Write(buf)
	if err != nil {
		return err
	}

	tsize := uint64(len(buf))
	bsize := make([]byte, 8)
	binary.BigEndian.PutUint64(bsize, tsize)

	_, err = fw.Write(bsize)
	return err
}

func decodeSnapshot(path string) (*storage.Snapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	bsize := make([]byte, 8)
	_, err = f.ReadAt(bsize, stat.Size()-8)
	if err == io.EOF {
		return nil, errSnapshotFormat
	}

	if err != nil {
		return nil, err
	}

	size := binary.BigEndian.Uint64(bsize)
	eod := stat.Size() - int64(size+8)
	buf := make([]byte, size)
	_, err = f.ReadAt(buf, eod)
	if err != nil {
		return nil, err
	}

	state := new(raftpb.SnapshotState)
	if err := state.Unmarshal(buf); err != nil {
		return nil, err
	}

	crc := crc64.New(crcTable)
	br := bufio.NewReader(f)
	lr := &io.LimitedReader{
		R: br,
		N: eod,
	}

	_, err = io.Copy(crc, lr)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(state.CRC, crc.Sum(nil)) {
		return nil, errCRCMismatch
	}

	// Reset file offset to read snap data again.
	_, _ = f.Seek(0, 0)
	br.Reset(f)
	lr.N = eod

	data := struct {
		io.Reader
		io.Closer
	}{
		lr,
		f,
	}

	s := new(storage.Snapshot)
	s.SnapshotState = *state
	s.Data = data

	return s, nil
}

type writer struct {
	*bufio.Writer
	*os.File
}

func (w writer) Write(p []byte) (n int, err error) {
	return w.Writer.Write(p)
}

func (w writer) Close() error {
	if err := w.Flush(); err != nil {
		return err
	}

	if err := fileutil.Fsync(w.File); err != nil {
		return err
	}

	return w.File.Close()
}
