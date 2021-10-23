package disk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shaj13/raftkit/internal/storage"
	"go.etcd.io/etcd/pkg/v3/fileutil"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/wal"
	"go.etcd.io/etcd/server/v3/wal/walpb"
)

var _ storage.Storage = &disk{}

// Config define common configuration used by the New function.
type Config interface {
	StateDir() string
	MaxSnapshotFiles() int
}

// New return new disk storage.
func New(ctx context.Context, c Config) storage.Storage {
	snapdir := filepath.Join(c.StateDir(), "snap")
	waldir := filepath.Join(c.StateDir(), "wal")
	gc := newGC(ctx, waldir, snapdir, c.MaxSnapshotFiles())
	disk := &disk{
		gc:      gc,
		waldir:  waldir,
		snapdir: snapdir,
		shoter:  &snapshotter{snapdir: snapdir},
	}

	return disk
}

// disk implements storage.Storage
type disk struct {
	wal     *wal.WAL
	shoter  *snapshotter
	gc      *gc
	waldir  string
	snapdir string
}

func (d *disk) purge() {
	go func() {
		d.gc.notifyc <- struct{}{}
	}()
}

// SaveSnapshot saves a given snapshot into the WAL.
// The raw snapshot must be saved into disk during the,
// network transportation.
func (d *disk) SaveSnapshot(snap raftpb.Snapshot) error {
	defer d.purge()

	walSnap := walpb.Snapshot{
		Index: snap.Metadata.Index,
		Term:  snap.Metadata.Term,
	}

	if err := d.wal.SaveSnapshot(walSnap); err != nil {
		return err
	}

	return d.wal.ReleaseLockTo(snap.Metadata.Index)
}

// SaveEntries saves a given entries into the WAL.
func (d *disk) SaveEntries(st raftpb.HardState, ents []raftpb.Entry) error {
	if err := d.wal.Save(st, ents); err != nil {
		return err
	}

	// short cut, do not call sync
	if raft.IsEmptyHardState(st) && len(ents) == 0 {
		return nil
	}

	return d.wal.Sync()
}

// Boot return wal metadata, hard-state, entries, and newest snapshot,
// Otherwise, it create new wal from given metadata alongside snapshots dir.
func (d *disk) Boot(meta []byte) ([]byte, raftpb.HardState, []raftpb.Entry, *storage.SnapshotFile, error) {
	fail := func(err error) ([]byte, raftpb.HardState, []raftpb.Entry, *storage.SnapshotFile, error) {
		return []byte{}, raftpb.HardState{}, []raftpb.Entry{}, nil, err
	}

	if !fileutil.Exist(d.snapdir) {
		if err := os.MkdirAll(d.snapdir, 0750); err != nil {
			return fail(
				fmt.Errorf("raft/storage/disk: create snapshot dir failed, Err: %s", err),
			)
		}
	}

	if !wal.Exist(d.waldir) {
		if err := os.MkdirAll(d.waldir, 0750); err != nil {
			return fail(
				fmt.Errorf("raft/storage/disk: create WAL dir failed, Err: %s", err),
			)
		}

		w, err := wal.Create(nil, d.waldir, meta)
		if err != nil {
			return fail(
				fmt.Errorf("raft/storage/disk: create WAL file failed, Err: %s", err),
			)
		}

		d.wal = w
		return meta, raftpb.HardState{}, []raftpb.Entry{}, nil, nil
	}

	walSnaps, err := wal.ValidSnapshotEntries(nil, d.waldir)

	if err != nil {
		return fail(
			fmt.Errorf("raft/storage/disk: list WAL snapshots failed, Err: %s", err),
		)
	}

	sf, err := decodeNewestAvailableSnapshot(d.snapdir, walSnaps)
	if err == ErrNoSnapshot {
		sf = new(storage.SnapshotFile)
		sf.Snap = new(raftpb.Snapshot)
	} else if err != nil {
		return fail(
			fmt.Errorf("raft/storage/disk: load newest snapshot failed, Err: %s", err),
		)
	}

	walsnap := walpb.Snapshot{
		Index: sf.Snap.Metadata.Index,
		Term:  sf.Snap.Metadata.Term,
	}

	w, err := wal.Open(nil, d.waldir, walsnap)
	if err != nil {
		return fail(
			fmt.Errorf("raft/storage/disk: open WAL failed, Err: %s", err),
		)
	}
	meta, st, ents, err := w.ReadAll()

	if err != nil {
		return fail(
			fmt.Errorf("raft/storage/disk: read WAL failed, Err: %s", err),
		)
	}

	d.wal = w
	d.gc.Start()
	defer d.purge()

	return meta, st, ents, sf, nil
}

func (d *disk) Exist() bool {
	return wal.Exist(d.waldir)
}

func (d *disk) Snapshotter() storage.Snapshotter {
	return d.shoter
}

func (d *disk) Close() error {
	d.wal.Close()
	return nil
}
