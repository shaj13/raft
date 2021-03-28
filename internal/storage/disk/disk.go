package disk

import (
	"fmt"
	"os"

	"github.com/shaj13/raftkit/internal/storage"
	"go.etcd.io/etcd/pkg/v3/fileutil"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/wal"
	"go.etcd.io/etcd/server/v3/wal/walpb"
)

// TODO: need to connect gc to disk and creat Purge method.

type Disk struct {
	wal     *wal.WAL
	waldir  string
	snapdir string
}

// SaveSnapshot saves a given snapshot into the WAL.
// The raw snapshot must be saved into disk during the,
// network transportation.
func (d *Disk) SaveSnapshot(snap raftpb.Snapshot) error {
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
func (d *Disk) SaveEntries(st raftpb.HardState, entries []raftpb.Entry) error {
	return d.wal.Save(st, entries)
}

// Boot return wal metadata, hard-state, entries, and newest snapshot,
// Otherwise, it create new wal from given metadata alongside snapshots dir.
func (d *Disk) Boot(meta []byte) ([]byte, raftpb.HardState, []raftpb.Entry, *storage.SnapshotFile, error) {
	fail := func(err error) ([]byte, raftpb.HardState, []raftpb.Entry, *storage.SnapshotFile, error) {
		return []byte{}, raftpb.HardState{}, []raftpb.Entry{}, nil, err
	}

	if !fileutil.Exist(d.snapdir) {
		if err := os.MkdirAll(d.snapdir, 0750); err != nil {
			return fail(
				fmt.Errorf("raft: failed to create snapshot dir, Err: %s", err),
			)
		}
	}

	if !wal.Exist(d.waldir) {
		if err := os.MkdirAll(d.waldir, 0750); err != nil {
			return fail(
				fmt.Errorf("raft: failed to create WAL dir, Err: %s", err),
			)
		}

		w, err := wal.Create(nil, d.waldir, meta)
		if err != nil {
			return fail(
				fmt.Errorf("raft: failed to create WAL dir, Err: %s", err),
			)
		}

		d.wal = w
		return meta, raftpb.HardState{}, []raftpb.Entry{}, nil, nil
	}

	walSnaps, err := wal.ValidSnapshotEntries(nil, d.waldir)

	if err != nil {
		return fail(
			fmt.Errorf("raft: failed to list WAL snapshots, Err: %s", err),
		)
	}

	sf, err := decodeNewestAvailableSnapshot(d.snapdir, walSnaps)
	if err == ErrNoSnapshot {
		sf = new(storage.SnapshotFile)
		sf.Snap = new(raftpb.Snapshot)
	} else if err != nil {
		return fail(
			fmt.Errorf("raft: failed to load newest snapshot, Err: %s", err),
		)
	}

	walsnap := walpb.Snapshot{
		Index: sf.Snap.Metadata.Index,
		Term:  sf.Snap.Metadata.Term,
	}

	w, err := wal.Open(nil, d.waldir, walsnap)
	if err != nil {
		return fail(
			fmt.Errorf("raft: failed to open WAL, Err: %s", err),
		)
	}
	meta, st, ents, err := w.ReadAll()

	if err != nil {
		return fail(
			fmt.Errorf("raft: failed to read WAL, Err: %s", err),
		)
	}

	d.wal = w
	return meta, st, ents, sf, nil
}

func (d *Disk) Exist() bool {
	return wal.Exist(d.waldir)
}

func (d *Disk) Close() {
	d.wal.Close()
}
