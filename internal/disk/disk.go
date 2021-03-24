package disk

import (
	"fmt"
	"os"

	"go.etcd.io/etcd/pkg/v3/fileutil"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/wal"
	"go.etcd.io/etcd/server/v3/wal/walpb"
	"go.uber.org/zap"
)

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
func (d *Disk) Boot(meta []byte) ([]byte, raftpb.HardState, []raftpb.Entry, *raftpb.Snapshot, error) {
	// TODO: get zap looger from cfg
	fail := func(err error) ([]byte, raftpb.HardState, []raftpb.Entry, *raftpb.Snapshot, error) {
		return []byte{}, raftpb.HardState{}, []raftpb.Entry{}, nil, err
	}

	if !fileutil.Exist(d.snapdir) {
		if err := os.Mkdir(d.snapdir, 0750); err != nil {
			return fail(
				fmt.Errorf("raft: failed to create snapshot dir, Err: %s", err),
			)
		}
	}

	tempsnap := snap.New(zap.NewExample(), d.snapdir)

	if !wal.Exist(d.waldir) {
		if err := os.Mkdir(d.waldir, 0750); err != nil {
			return fail(
				fmt.Errorf("raft: failed to create WAL dir, Err: %s", err),
			)
		}

		w, err := wal.Create(zap.NewExample(), d.waldir, meta)
		if err != nil {
			return fail(
				fmt.Errorf("raft: failed to create WAL dir, Err: %s", err),
			)
		}

		d.wal = w
		return meta, raftpb.HardState{}, []raftpb.Entry{}, nil, nil
	}

	walSnaps, err := wal.ValidSnapshotEntries(zap.NewExample(), d.waldir)

	if err != nil {
		return fail(
			fmt.Errorf("raft: failed to list WAL snapshots, Err: %s", err),
		)
	}

	snapshot, err := tempsnap.LoadNewestAvailable(walSnaps)
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

	w, err := wal.Open(zap.NewExample(), d.waldir, walsnap)
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
	return meta, st, ents, snapshot, nil
}

func (d *Disk) Exist() bool {
	return wal.Exist(d.waldir)
}