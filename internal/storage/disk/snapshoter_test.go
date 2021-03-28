package disk

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/pkg/v3/fileutil"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func TestSnapshoterReaderErr(t *testing.T) {
	table := []struct {
		name     string
		contains string
		msg      raftpb.Message
	}{
		{
			name:     "it return error when snap msg empty",
			contains: ErrEmptySnapshot.Error(),
			msg:      raftpb.Message{},
		},
		{
			name:     "it return error when snap file does not exist",
			contains: "no such file or directory",
			msg:      testMsg(10000, 10000),
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			s := &snapshoter{snapdir: "./testdata/"}
			_, _, err := s.Reader(context.TODO(), tt.msg)
			if err == nil {
				t.Fatal("expected non nil error")
			}
			assert.Contains(t, err.Error(), tt.contains)
		})
	}
}

func TestSnapshoterReader(t *testing.T) {
	s := &snapshoter{snapdir: "./testdata/"}
	name, r, err := s.Reader(context.TODO(), testMsg(1, 1))
	assert.NoError(t, err)
	assert.Equal(t, snapshotName(1, 1), name)
	if assert.NotNil(t, r) {
		r.Close()
	}
}

func TestSnapshoterWriter(t *testing.T) {
	file := "testsnapshoterwriter_invalid"
	ctx := context.TODO()
	s := &snapshoter{snapdir: "/does_not_exist"}

	// Round #1 check file error
	_, _, err := s.Writer(ctx, file)
	assert.Contains(t, err.Error(), "no such file or directory")

	// Round #2 check write and peek
	s.snapdir = "./testdata"
	w, peek, err := s.Writer(ctx, file)

	if !assert.NoError(t, err) {
		return
	}

	w.Write([]byte(""))
	w.Close()

	_, err = peek()
	assert.Error(t, err)

	exist := fileutil.Exist(filepath.Join(s.snapdir, file))
	assert.False(t, exist, "expect peek to remove file if there an error")
}

func testMsg(index, term uint64) raftpb.Message {
	st, _ := snapshotTestFile()
	st.Snap.Metadata.Index = index
	st.Snap.Metadata.Term = term
	return raftpb.Message{
		Snapshot: *st.Snap,
	}
}
