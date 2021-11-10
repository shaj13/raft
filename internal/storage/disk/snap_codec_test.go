package disk

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/storage"
	"github.com/stretchr/testify/require"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/wal/walpb"
)

func TestSnapshotCodec(t *testing.T) {
	dir := createTestDir("read-write", t)
	path := filepath.Join(dir, t.Name())
	defer os.RemoveAll(dir)

	expected, expectedData := snapshotTestFile()
	err := encodeSnapshot(path, &expected)
	require.NoError(t, err)

	got, err := decodeSnapshot(path)
	require.NoError(t, err)
	require.Equal(t, expected.Raw, got.Raw)
	require.Equal(t, expected.Members, got.Members)

	gotData, err := ioutil.ReadAll(got.Data)
	require.NoError(t, err)
	require.Equal(t, expectedData, string(gotData))
}

func TestPeekSnapshot(t *testing.T) {
	expected, _ := snapshotTestFile()

	// Round #1 it return error when file invalid
	snap, err := peekSnapshot("")
	require.Error(t, err)

	// Round #2 it return snap object
	snap, err = peekSnapshot("./testdata/valid.snap")
	require.NoError(t, err)
	require.Equal(t, expected.Raw, snap)
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
			name:     "it return error when snapshot empty",
			file:     "./testdata/empty.snap",
			contains: "negative offset",
		},
		// {
		// 	name:     "it return error when snapshot have invalid format",
		// 	file:     "./testdata/format.snap",
		// 	contains: ErrSnapshotFormat.Error(),
		// },
		{
			name:     "it return error when snapshot have invalid crc",
			file:     "./testdata/crc.snap",
			contains: ErrCRCMismatch.Error(),
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeSnapshot(tt.file)
			require.Contains(t, err.Error(), tt.contains)
		})
	}
}

func TestDecodeNewestAvailableSnapshot(t *testing.T) {
	// Round #1 it return error when snapshots dir does not exist
	sf, err := decodeNewestAvailableSnapshot("", []walpb.Snapshot{})
	require.Nil(t, sf)
	require.Contains(t, err.Error(), "no such file or directory")

	// Round #2 it return error when no snapshots
	sf, err = decodeNewestAvailableSnapshot("./testdata/", []walpb.Snapshot{})
	require.Nil(t, sf)
	require.Equal(t, ErrNoSnapshot, err)

	// Round #3 it return latest snapshots
	expected, _ := snapshotTestFile()
	sf, err = decodeNewestAvailableSnapshot("./testdata/", []walpb.Snapshot{{Index: 3, Term: 3}})
	require.NoError(t, err)
	require.Equal(t, expected.Raw, sf.Raw)
}

func snapshotTestFile() (storage.Snapshot, string) {
	const data = "some app data"
	return storage.Snapshot{
		SnapshotState: raftpb.SnapshotState{
			Raw: etcdraftpb.Snapshot{
				Metadata: etcdraftpb.SnapshotMetadata{
					ConfState: etcdraftpb.ConfState{
						Voters: []uint64{1, 2, 3},
					},
					Index: 1,
					Term:  1,
				},
			},
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
