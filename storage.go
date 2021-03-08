package raft

import (
	"fmt"
	"os"
	"path/filepath"

	"go.etcd.io/etcd/pkg/v3/fileutil"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/wal"
	"go.etcd.io/etcd/server/v3/wal/walpb"
	"go.uber.org/zap"
)

// TODO: remove this
type Storgae interface {
	// Append the new entries to storage.
	Append(entries []raftpb.Entry) error
	// ApplySnapshot overwrites the contents of this Storage object with
	// those of the given snapshot.
	ApplySnapshot(snap raftpb.Snapshot) error
	// CreateSnapshot makes a snapshot which can be retrieved with Snapshot() and
	// can be used to reconstruct the state at that point.
	// If any configuration changes have been made since the last compaction,
	// the result of the last ApplyConfChange must be passed in.
	CreateSnapshot(i uint64, cs *raftpb.ConfState, data []byte) (raftpb.Snapshot, error)
	// Compact discards all log entries prior to compactIndex.
	// It is the application's responsibility to not attempt to compact an index
	// greater than raftLog.applied.
	Compact(compactIndex uint64) error
}

type Snapshoter interface {
	SaveSnapshot(raftpb.Snapshot) error
	SaveEntries(st raftpb.HardState, entries []raftpb.Entry) error
}

type disk struct {
	cfg  *config
	wal  *wal.WAL
	snap *snap.Snapshotter
}

// SaveSnapshot saves a given snapshot to both the WAL and the snapshot.
func (d *disk) SaveSnapshot(snap raftpb.Snapshot) error {
	walSnap := walpb.Snapshot{
		Index: snap.Metadata.Index,
		Term:  snap.Metadata.Term,
	}

	// save the snapshot file before writing the snapshot to the wal.
	// This makes it possible for the snapshot file to become orphaned, but prevents
	// a WAL snapshot entry from having no corresponding snapshot file.
	if err := d.snap.SaveSnap(snap); err != nil {
		return err
	}

	if err := d.wal.SaveSnapshot(walSnap); err != nil {
		return err
	}

	return d.wal.ReleaseLockTo(snap.Metadata.Index)
}

func (d *disk) SaveEntries(st raftpb.HardState, entries []raftpb.Entry) error {
	return d.wal.Save(st, entries)
}

// TODO: need to split boot from disk and create.
func (d *disk) bootstrap() ([]byte, raftpb.HardState, []raftpb.Entry, error) {
	// TODO: get zap looger from cfg
	fail := func(err error) ([]byte, raftpb.HardState, []raftpb.Entry, error) {
		return []byte{}, raftpb.HardState{}, []raftpb.Entry{}, err
	}
	waldir := filepath.Join(d.cfg.stateDir, "wal")
	snapdir := filepath.Join(d.cfg.stateDir, "snap")

	if !fileutil.Exist(snapdir) {
		if err := os.Mkdir(snapdir, 0750); err != nil {
			return fail(
				fmt.Errorf("raft: failed to create snapshot dir, Err: %s", err),
			)
		}
	}

	d.snap = snap.New(zap.NewExample(), snapdir)

	if !wal.Exist(waldir) {
		if err := os.Mkdir(waldir, 0750); err != nil {
			return fail(
				fmt.Errorf("raft: failed to create WAL dir, Err: %s", err),
			)
		}

		w, err := wal.Create(zap.NewExample(), waldir, nil)
		if err != nil {
			return fail(
				fmt.Errorf("raft: failed to create WAL dir, Err: %s", err),
			)
		}

		d.wal = w
		return []byte{}, raftpb.HardState{}, []raftpb.Entry{}, nil
	}

	walSnaps, err := wal.ValidSnapshotEntries(zap.NewExample(), waldir)

	if err != nil {
		return fail(
			fmt.Errorf("raft: failed to list WAL snapshots, Err: %s", err),
		)
	}

	snapshot, err := d.snap.LoadNewestAvailable(walSnaps)
	if err == snap.ErrNoSnapshot {
		snapshot = new(raftpb.Snapshot)
	} else if err != nil {
		return fail(
			fmt.Errorf("raft: failed to load newest snapshot, Err: %s", err),
		)
	}

	walsnap := walpb.Snapshot{
		Index: snapshot.Metadata.Index,
		Term:  snapshot.Metadata.Term,
	}

	w, err := wal.Open(zap.NewExample(), waldir, walsnap)
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

	return meta, st, ents, nil
}
