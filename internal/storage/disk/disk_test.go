package disk

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/pkg/v3/fileutil"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/wal"
	"go.etcd.io/etcd/server/v3/wal/walpb"
)

func TestDiskWalInteraction(t *testing.T) {

	dir := createTestDir("wal", t)
	defer os.RemoveAll(dir)

	sf, _ := snapshotTestFile()
	hs := raftpb.HardState{
		Term:   sf.Raw.Metadata.Term,
		Commit: sf.Raw.Metadata.Index,
	}

	// create wal and append data using disk objec.
	w, _ := wal.Create(nil, dir, nil)
	disk := newTestDisk("")
	disk.wal = w

	err := disk.SaveSnapshot(sf.Raw)
	require.NoError(t, err)

	err = disk.SaveEntries(hs, []raftpb.Entry{})
	require.NoError(t, err)

	// clsoe disk
	close(disk.gc.done)
	disk.Close()

	// open wal for read and check data against waht saved.
	snaps, _ := wal.ValidSnapshotEntries(nil, dir)
	w, _ = wal.OpenForRead(nil, dir, walpb.Snapshot{})
	_, gotHs, _, _ := w.ReadAll()

	require.Equal(t, sf.Raw.Metadata.Index, snaps[1].Index)
	require.Equal(t, sf.Raw.Metadata.Term, snaps[1].Term)
	require.Equal(t, gotHs, hs)

}

func TestDiskBootMkdir(t *testing.T) {
	temp := filepath.Join(os.TempDir(), "/test_disk_boot")
	defer os.RemoveAll(temp)
	d := new(disk)
	d.cfg = new(config)
	d.snapdir = ""
	d.waldir = ""

	_, _, _, _, err := d.Boot(nil)
	require.Contains(t, err.Error(), "create snapshot dir")

	d.snapdir = os.TempDir()
	_, _, _, _, err = d.Boot(nil)
	require.Contains(t, err.Error(), "create WAL dir")

	// it now should create dir
	d.snapdir = temp
	d.waldir = temp
	_, _, _, _, err = d.Boot(nil)
	require.NoError(t, err)
	require.True(t, fileutil.Exist(temp))
}

func TestDiskBoot(t *testing.T) {
	temp := filepath.Join(os.TempDir(), "/test_disk_boot")
	defer os.RemoveAll(temp)

	d := newTestDisk(temp)

	t.Run("it return error when wal locked", func(t *testing.T) {
		defer d.Close()
		os.RemoveAll(temp)

		_, _, _, _, err := d.Boot(nil)
		require.NoError(t, err)

		_, _, _, _, err = d.Boot(nil)
		require.Contains(t, err.Error(), "file already locked")
	})

	t.Run("it open the existing wal", func(t *testing.T) {
		defer d.Close()
		os.RemoveAll(temp)
		meta := []byte("wal metadata")

		_, _, _, _, err := d.Boot(meta)
		require.NoError(t, err)
		d.Close()

		got, _, _, _, err := d.Boot(nil)
		require.NoError(t, err)
		require.Equal(t, meta, got)
	})
}

func TestDiskExist(t *testing.T) {
	d := new(disk)
	require.False(t, d.Exist())
}

func newTestDisk(dir string) *disk {
	gc := newGC(context.TODO(), dir, dir, 100)
	d := new(disk)
	d.cfg = new(config)
	d.snapdir = dir
	d.waldir = dir
	d.gc = gc
	return d
}

type config struct{}

func (config) StateDir() (str string)    { return }
func (config) MaxSnapshotFiles() (i int) { return }
func (config) Context() context.Context  { return context.TODO() }
