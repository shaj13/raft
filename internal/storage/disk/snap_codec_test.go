package disk

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/storage"
	"github.com/stretchr/testify/assert"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/wal/walpb"
)

func TestSnapshotCodec(t *testing.T) {
	dir := createTestDir("read-write", t)
	path := filepath.Join(dir, t.Name())
	defer os.RemoveAll(dir)

	expected, expectedData := snapshotTestFile()
	err := encodeSnapshot(path, &expected)
	assert.NoError(t, err)

	got, err := decodeSnapshot(path)
	assert.NoError(t, err)
	assert.Equal(t, expected.Snap, got.Snap)
	assert.Equal(t, expected.Pool, got.Pool)

	gotData, err := ioutil.ReadAll(got.Data)
	assert.NoError(t, err)
	assert.Equal(t, expectedData, string(gotData))
}

func TestPeekSnapshot(t *testing.T) {
	expected, _ := snapshotTestFile()

	// Round #1 it return error when file invalid
	snap, err := peekSnapshot("")
	assert.Error(t, err)
	assert.Nil(t, snap)

	// Round #2 it return snap object
	snap, err = peekSnapshot("./testdata/valid.snap")
	assert.NoError(t, err)
	assert.Equal(t, expected.Snap, snap)
}

func TestDecodeSnapErr(t *testing.T) {
	table := []struct {
		name     string
		file     string
		contains string
	}{
		{
			name:     "it return error when file does not exist",
			file:     "test",
			contains: "no such file or directory",
		},
		{
			name:     "it return error when file have unexpected EOF",
			file:     "./testdata/ueof.snap",
			contains: io.ErrUnexpectedEOF.Error(),
		},
		{
			name:     "it return error when snapshot empty",
			file:     "./testdata/empty.snap",
			contains: ErrEmptySnapshot.Error(),
		},
		{
			name:     "it return error when snapshot have invalid format",
			file:     "./testdata/format.snap",
			contains: ErrSnapshotFormat.Error(),
		},
		{
			name:     "it return error when snapshot have invalid crc",
			file:     "./testdata/crc.snap",
			contains: ErrCRCMismatch.Error(),
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeSnapshot(tt.file)
			assert.Contains(t, err.Error(), tt.contains)
		})
	}
}

func TestDecodeNewestAvailableSnapshot(t *testing.T) {
	// Round #1 it return error when snapshots dir does not exist
	sf, err := decodeNewestAvailableSnapshot("", []walpb.Snapshot{})
	assert.Nil(t, sf)
	assert.Contains(t, err.Error(), "no such file or directory")

	// Round #2 it return error when no snapshots
	sf, err = decodeNewestAvailableSnapshot("./testdata/", []walpb.Snapshot{})
	assert.Nil(t, sf)
	assert.Equal(t, ErrNoSnapshot, err)

	// Round #3 it return latest snapshots
	expected, _ := snapshotTestFile()
	sf, err = decodeNewestAvailableSnapshot("./testdata/", []walpb.Snapshot{{Index: 3, Term: 3}})
	assert.NoError(t, err)
	assert.Equal(t, expected.Snap, sf.Snap)
}

func TestSnapshotFileReader(t *testing.T) {
	f, err := os.Open("./testdata/empty.snap")
	if err != nil {
		t.Fatal(err)
	}

	r := readerPool.Get().(*fileReader)
	r.Reset(f)

	err = r.Close()
	assert.NoError(t, err)

	_, err = r.Read([]byte{})
	assert.Equal(t, ErrClosedSnapshot, err)

	err = r.Close()
	assert.Equal(t, ErrClosedSnapshot, err)
}

func TestSnapshotFileWriter(t *testing.T) {
	data := []byte("file data")
	path := filepath.Join(os.TempDir(), "snapfilewriter")
	defer os.Remove(path)

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	w := writerPool.Get().(*fileWriter)
	w.Reset(f, nil)

	_, err = w.Write(data)
	assert.NoError(t, err)

	err = w.Close()
	assert.NoError(t, err)

	_, err = w.Write(data)
	assert.Equal(t, ErrClosedSnapshot, err)

	err = w.Close()
	assert.Equal(t, ErrClosedSnapshot, err)

	got, _ := ioutil.ReadFile(path)
	assert.Equal(t, data, got)
}

func snapshotTestFile() (storage.SnapshotFile, string) {
	const data = "some app data"
	return storage.SnapshotFile{
		Snap: &etcdraftpb.Snapshot{
			Metadata: etcdraftpb.SnapshotMetadata{
				ConfState: etcdraftpb.ConfState{
					Voters: []uint64{1, 2, 3},
				},
				Index: 1,
				Term:  1,
			},
		},
		Pool: &raftpb.Pool{
			Members: []raftpb.Member{
				{
					Address: ":50052",
					ID:      11,
				},
			},
		},
		Data: ioutil.NopCloser(strings.NewReader(data)),
	}, data
}
