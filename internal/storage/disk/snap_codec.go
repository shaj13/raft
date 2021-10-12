package disk

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"hash"
	"hash/crc64"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/storage"
	"go.etcd.io/etcd/pkg/v3/fileutil"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/wal/walpb"
)

const delim = '\r'

var (
	crcPool = sync.Pool{
		New: func() interface{} {
			return crc64.New(
				crc64.MakeTable(crc64.ECMA),
			)
		},
	}

	writerPool = sync.Pool{
		New: func() interface{} {
			w := bufio.NewWriter(nil)
			return &fileWriter{Writer: w}
		},
	}

	readerPool = sync.Pool{
		New: func() interface{} {
			r := bufio.NewReader(nil)
			return &fileReader{Reader: r}
		},
	}
)

var (
	ErrEmptySnapshot  = errors.New("raft/disk: empty snapshot file")
	ErrSnapshotFormat = errors.New("raft/disk: invalid snapshot file format")
	ErrCRCMismatch    = errors.New("raft/disk: snapshot file corrupted, crc mismatch")
	ErrClosedSnapshot = errors.New("raft/disk: read/write on closed snapshot")
	ErrNoSnapshot     = errors.New("raft/disk: no available snapshot")
)

func snapshotName(term, index uint64) string {
	return fmt.Sprintf(format, term, index) + snapExt
}

func decodeNewestAvailableSnapshot(dir string, snaps []walpb.Snapshot) (*storage.SnapshotFile, error) {
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
		return nil, ErrNoSnapshot
	}

	return decodeSnapshot(filepath.Join(dir, target))
}

func decodeSnapshot(path string) (sf *storage.SnapshotFile, err error) {
	sf = new(storage.SnapshotFile)
	sf.Snap = new(etcdraftpb.Snapshot)
	sf.Pool = new(raftpb.Pool)
	sf.Data, err = decodeSnapshotByblocks(path, sf.Snap, sf.Pool)
	return
}

func peekSnapshot(path string) (*etcdraftpb.Snapshot, error) {
	sf := new(storage.SnapshotFile)
	sf.Snap = new(etcdraftpb.Snapshot)
	r, err := decodeSnapshotByblocks(path, sf.Snap)
	if err != nil {
		return nil, err
	}
	r.Close()
	return sf.Snap, nil
}

func encodeSnapshot(path string, s *storage.SnapshotFile) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}

	// header writer used to skip crc.
	hw := writerPool.Get().(*fileWriter)
	w := writerPool.Get().(*fileWriter)
	crc := crcPool.Get().(hash.Hash64)

	crc.Reset()
	hw.Reset(f, nil)
	w.Reset(
		f,
		io.MultiWriter(crc, f),
	)

	defer func() {
		w.Close()
		hw.Close()

		crcPool.Put(crc)

		if err != nil {
			os.Remove(path)
		}
	}()

	msgs := []proto.Message{
		// reserve header to rewrite it at the end.
		&raftpb.SnapshotHeader{
			CRC: make([]byte, crc64.Size),
		},
		s.Snap,
		s.Pool,
	}

	for i, m := range msgs {
		w := w
		if i == 0 {
			w = hw
		}

		block, err := proto.Marshal(m)
		if err != nil {
			return err
		}

		if _, err := w.Write(block); err != nil {
			return err
		}

		if err := w.WriteByte(delim); err != nil {
			return err
		}

		w.FlushAndSync()
	}

	_, err = io.Copy(w, s.Data)
	if err != nil {
		return err
	}

	w.FlushAndSync()

	if _, err := f.Seek(0, 0); err != nil {
		return err
	}

	h := &raftpb.SnapshotHeader{
		CRC: crc.Sum(nil),
	}

	block := pbutil.MustMarshal(h)
	_, err = w.Write(block)

	return err
}

func decodeSnapshotByblocks(path string, msgs ...proto.Message) (rc io.ReadCloser, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	crc := crcPool.Get().(hash.Hash64)
	r := readerPool.Get().(*fileReader)
	r.Reset(f)
	crc.Reset()

	defer func() {
		crcPool.Put(crc)
		if err != nil {
			r.Close()
		}
	}()

	n := 0
	header := new(raftpb.SnapshotHeader)
	msgs = append(msgs, nil)
	copy(msgs[1:], msgs[:])
	msgs[0] = header

	_, err = r.Peek(1)
	if err != nil {
		return nil, ErrEmptySnapshot
	}

	for i, m := range msgs {
		block, err := r.ReadBytes(delim)
		if err == io.EOF {
			return nil, ErrSnapshotFormat
		}

		if err != nil {
			return nil, err
		}

		if i != 0 {
			crc.Write(block)
		}

		if len(block) > 0 {
			// remove delim.
			block = block[:len(block)-1]
		}

		if err := proto.Unmarshal(block, m); err != nil {
			return nil, err
		}

		n += len(block) + 1
	}

	_, err = io.Copy(crc, r)
	if err != nil {
		return nil, err
	}

	if bytes.Compare(crc.Sum(nil), header.CRC) != 0 {
		return nil, ErrCRCMismatch
	}

	f.Seek(int64(n), 0)
	r.Reset(f)

	return r, nil
}

type fileReader struct {
	*bufio.Reader
	file *os.File
	err  error
}

func (f *fileReader) Read(p []byte) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.Reader.Read(p)
}

func (f *fileReader) Close() error {
	if f.err != nil {
		return f.err
	}
	file := f.file
	f.err = ErrClosedSnapshot
	f.file = nil
	f.Reader.Reset(nil)
	readerPool.Put(f)
	return file.Close()
}

func (f *fileReader) Reset(file *os.File) {
	f.file = file
	f.err = nil
	f.Reader.Reset(file)
}

type fileWriter struct {
	*bufio.Writer
	file *os.File
	name string
	err  error
}

func (f *fileWriter) FlushAndSync() {
	if f.Buffered() == 0 {
		return
	}
	f.Writer.Flush()
	fileutil.Fsync(f.file)
}

func (f *fileWriter) Write(p []byte) (int, error) {
	if f.err != nil {
		return 0, f.err
	}

	return f.Writer.Write(p)
}

func (f *fileWriter) Reset(file *os.File, w io.Writer) {
	var writer io.Writer = file
	f.file = file
	f.err = nil
	if w != nil {
		writer = w
	}
	f.Writer.Reset(writer)
}

func (f *fileWriter) Close() error {
	if f.err != nil {
		return f.err
	}
	f.FlushAndSync()
	file := f.file
	f.err = ErrClosedSnapshot
	f.file = nil
	f.Writer.Reset(nil)
	writerPool.Put(f)
	return file.Close()
}
