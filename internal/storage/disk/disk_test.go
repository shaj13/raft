package disk

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
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
		Term:   sf.Snap.Metadata.Term,
		Commit: sf.Snap.Metadata.Index,
	}

	// create wal and append data using disk objec.
	w, _ := wal.Create(nil, dir, nil)
	disk := newTestDisk("")
	disk.wal = w

	err := disk.SaveSnapshot(*sf.Snap)
	assert.NoError(t, err)

	err = disk.SaveEntries(hs, []raftpb.Entry{})
	assert.NoError(t, err)

	// clsoe disk
	disk.Close()

	// open wal for read and check data against waht saved.
	snaps, _ := wal.ValidSnapshotEntries(nil, dir)
	w, _ = wal.OpenForRead(nil, dir, walpb.Snapshot{})
	_, gotHs, _, _ := w.ReadAll()

	assert.Equal(t, sf.Snap.Metadata.Index, snaps[1].Index)
	assert.Equal(t, sf.Snap.Metadata.Term, snaps[1].Term)
	assert.Equal(t, gotHs, hs)

}

func TestDiskBootMkdir(t *testing.T) {
	temp := filepath.Join(os.TempDir(), "/test_disk_boot")
	defer os.RemoveAll(temp)
	d := new(disk)
	d.snapdir = ""
	d.waldir = ""

	_, _, _, _, err := d.Boot(nil)
	assert.Contains(t, err.Error(), "create snapshot dir")

	d.snapdir = os.TempDir()
	_, _, _, _, err = d.Boot(nil)
	assert.Contains(t, err.Error(), "create WAL dir")

	// it now should create dir
	d.snapdir = temp
	d.waldir = temp
	_, _, _, _, err = d.Boot(nil)
	assert.NoError(t, err)
	assert.True(t, fileutil.Exist(temp))
}

func TestDiskBoot(t *testing.T) {
	temp := filepath.Join(os.TempDir(), "/test_disk_boot")
	defer os.RemoveAll(temp)

	d := newTestDisk(temp)

	t.Run("it return error when wal locked", func(t *testing.T) {
		defer d.Close()
		os.RemoveAll(temp)

		_, _, _, _, err := d.Boot(nil)
		assert.NoError(t, err)

		_, _, _, _, err = d.Boot(nil)
		assert.Contains(t, err.Error(), "file already locked")
	})

	t.Run("it open the existing wal", func(t *testing.T) {
		defer d.Close()
		os.RemoveAll(temp)
		meta := []byte("wal metadata")

		_, _, _, _, err := d.Boot(meta)
		assert.NoError(t, err)
		d.Close()

		got, _, _, _, err := d.Boot(nil)
		assert.NoError(t, err)
		assert.Equal(t, meta, got)
	})
}

func TestDiskExist(t *testing.T) {
	d := new(disk)
	assert.False(t, d.Exist())
}

func newTestDisk(dir string) *disk {
	d := new(disk)
	d.snapdir = dir
	d.waldir = dir
	d.gc = newGC(context.TODO(), dir, dir, 100)
	return d
}
