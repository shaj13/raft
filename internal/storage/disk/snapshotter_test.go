package disk

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/pkg/v3/fileutil"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func TestSnapshoterReaderErr(t *testing.T) {
	table := []struct {
		name     string
		contains string
		snap     raftpb.Snapshot
	}{
		{
			name:     "it return error when snap msg empty",
			contains: ErrEmptySnapshot.Error(),
			snap:     raftpb.Snapshot{},
		},
		{
			name:     "it return error when snap file does not exist",
			contains: "no such file or directory",
			snap:     testSnap(10000, 10000),
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			s := &snapshotter{snapdir: "./testdata/"}
			_, _, err := s.Reader(tt.snap)
			if err == nil {
				t.Fatal("expected non nil error")
			}
			require.Contains(t, err.Error(), tt.contains)
		})
	}
}

func TestSnapshoterReader(t *testing.T) {
	s := &snapshotter{snapdir: "./testdata/"}
	name, r, err := s.Reader(testSnap(1, 1))
	require.NoError(t, err)
	require.Equal(t, snapshotName(1, 1), name)
	require.NotNil(t, r)
	r.Close()
}

func TestSnapshoterWriter(t *testing.T) {
	file := "testsnapshoterwriter_invalid"
	s := &snapshotter{snapdir: "/does_not_exist"}

	// Round #1 check file error
	_, _, err := s.Writer(file)
	require.Contains(t, err.Error(), "no such file or directory")

	// Round #2 check write and peek
	s.snapdir = "./testdata"
	w, peek, err := s.Writer(file)
	require.NoError(t, err)

	w.Write([]byte(""))
	w.Close()

	_, err = peek()
	require.Error(t, err)

	exist := fileutil.Exist(filepath.Join(s.snapdir, file))
	require.False(t, exist, "expect peek to remove file if there an error")
}

func testSnap(index, term uint64) raftpb.Snapshot {
	st, _ := snapshotTestFile()
	st.Raw.Metadata.Index = index
	st.Raw.Metadata.Term = term
	return st.Raw
}
