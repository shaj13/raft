package disk

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSnapshotterReaderWriter(t *testing.T) {
	nils := "<nil>"
	notExist := "/does-not-exist-1-2-3"
	noFileDir := "no such file or directory"

	callReader := func(s *snapshotter) error {
		_, err := s.Reader(1, 1)
		return err
	}
	callWriter := func(s *snapshotter) error {
		_, err := s.Writer(0, 0)
		return err
	}

	table := []struct {
		path     string
		contains string
		call     func(*snapshotter) error
	}{
		{
			path:     notExist,
			contains: noFileDir,
			call:     callReader,
		},
		{
			path:     "./testdata",
			call:     callReader,
			contains: nils,
		},
		{
			path:     notExist,
			contains: noFileDir,
			call:     callWriter,
		},
		{
			path:     t.TempDir(),
			contains: nils,
			call:     callWriter,
		},
	}

	for i, tt := range table {
		s := new(snapshotter)
		s.snapdir = tt.path
		err := tt.call(s)
		got := nils
		if err != nil {
			got = err.Error()
		}
		require.Contains(t, got, tt.contains, i)
	}
}

func TestSnapshotterReadWrite(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/" + snapshotName(1, 1)
	buf, err := ioutil.ReadFile("./testdata/valid.snap")
	require.NoError(t, err)

	err = ioutil.WriteFile(path, buf, 0600)
	require.NoError(t, err)

	shotter := new(snapshotter)
	shotter.snapdir = dir

	snap, err := shotter.Read(1, 1)
	require.NoError(t, err)

	err = shotter.Write(snap)
	require.NoError(t, err)

	snap, err = shotter.ReadFrom(path)
	require.NoError(t, err)
	buf, _ = ioutil.ReadAll(snap.Data)
	require.Equal(t, "some app data", string(buf))
}
