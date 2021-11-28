package disk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shaj13/raft/internal/storage"
	"github.com/shaj13/raft/raftlog"
	"go.etcd.io/etcd/pkg/v3/fileutil"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/wal"
	"go.etcd.io/etcd/server/v3/wal/walpb"
)

var _ storage.Storage = &disk{}

// Config define common configuration used by the New function.
type Config interface {
	StateDir() string
	MaxSnapshotFiles() int
	Context() context.Context
	Logger() raftlog.Logger
}

// New return new disk storage.
func New(cfg Config) storage.Storage {
	snapdir := filepath.Join(cfg.StateDir(), "snap")
	waldir := filepath.Join(cfg.StateDir(), "wal")
	disk := &disk{
		maxsnaps: cfg.MaxSnapshotFiles(),
		logger:   cfg.Logger(),
		waldir:   waldir,
		snapdir:  snapdir,
		shoter:   &snapshotter{snapdir: snapdir},
	}

	return disk
}

// disk implements storage.Storage
type disk struct {
	wal      *wal.WAL
	shoter   *snapshotter
	logger   raftlog.Logger
	maxsnaps int
	waldir   string
	snapdir  string
}

func (d *disk) purge() {
	fn := func() error {
		files, err := list(d.snapdir, snapExt)
		if err != nil || len(files) < d.maxsnaps || len(files) == 0 {
			return err
		}

		// snapshots.
		var (
			current = files[0]
			oldest  string
		)

		for i, f := range files {
			if f != current && i >= d.maxsnaps {
				path := filepath.Join(d.snapdir, f)
				if err := os.Remove(path); err != nil {
					return err
				}
				continue
			}
			oldest = f
		}

		// oldest snapshot term and index.
		var st, si uint64
		_, err = fmt.Sscanf(oldest, format+snapExt, &st, &si)
		if err != nil {
			return err
		}

		files, err = list(d.waldir, walExt)
		if err != nil {
			return err
		}

		mark := -1

		for i, f := range files {
			// wal sequence and index.
			var ws, wi uint64
			_, err = fmt.Sscanf(f, format+walExt, &ws, &wi)
			if err != nil {
				return err
			}

			if wi >= si {
				mark = i
			}
		}

		if mark == 0 && len(files) > 0 {
			mark = len(files) - 1
		}

		for i := 0; i < mark; i++ {
			path := filepath.Join(d.waldir, files[len(files)-i-1])
			lock, err := fileutil.TryLockFile(path, os.O_WRONLY, fileutil.PrivateFileMode)
			if err != nil {
				return err
			}

			err = os.Remove(path)
			_ = lock.Close()

			if err != nil {
				return err
			}
		}

		return nil
	}

	if err := fn(); err != nil {
		d.logger.Warningf("raft.storage: purging oldest snapshots/WALs files: %v", err)
	}
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

	// Force WAL to fsync its hard state before ReleaseLockTo() releases
	// old data from the WAL. Otherwise could get an error like:
	// panic: tocommit(107) is out of range [lastIndex(84)]. Was the raft log corrupted, truncated, or lost?
	if err := d.wal.Sync(); err != nil {
		return err
	}

	return d.wal.ReleaseLockTo(snap.Metadata.Index)
}

// SaveEntries saves a given entries into the WAL.
func (d *disk) SaveEntries(st raftpb.HardState, ents []raftpb.Entry) error {
	return d.wal.Save(st, ents)
}

// Boot return wal metadata, hard-state, entries, and newest snapshot,
// Otherwise, it create new wal from given metadata alongside snapshots dir.
func (d *disk) Boot(meta []byte) ([]byte, raftpb.HardState, []raftpb.Entry, *storage.Snapshot, error) {
	fail := func(err error) ([]byte, raftpb.HardState, []raftpb.Entry, *storage.Snapshot, error) {
		return []byte{}, raftpb.HardState{}, []raftpb.Entry{}, nil, err
	}

	if !fileutil.Exist(d.snapdir) {
		if err := os.MkdirAll(d.snapdir, 0750); err != nil {
			return fail(
				fmt.Errorf("raft/storage: create snapshot dir: %v", err),
			)
		}
	}

	if !wal.Exist(d.waldir) {
		if err := os.MkdirAll(d.waldir, 0750); err != nil {
			return fail(
				fmt.Errorf("raft/storage: create WAL dir: %v", err),
			)
		}

		w, err := wal.Create(nil, d.waldir, meta)
		if err != nil {
			return fail(
				fmt.Errorf("raft/storage: create WAL file: %v", err),
			)
		}

		d.wal = w
		return meta, raftpb.HardState{}, []raftpb.Entry{}, nil, nil
	}

	walSnaps, err := wal.ValidSnapshotEntries(nil, d.waldir)

	if err != nil {
		return fail(
			fmt.Errorf("raft/storage: list WAL snapshots: %v", err),
		)
	}

	sf, err := decodeNewestAvailableSnapshot(d.snapdir, walSnaps)
	if err == errNoSnapshot {
		sf = new(storage.Snapshot)
	} else if err != nil {
		return fail(
			fmt.Errorf("raft/storage: load newest snapshot: %v", err),
		)
	}

	walsnap := walpb.Snapshot{
		Index: sf.Raw.Metadata.Index,
		Term:  sf.Raw.Metadata.Term,
	}

	w, err := wal.Open(nil, d.waldir, walsnap)
	if err != nil {
		return fail(
			fmt.Errorf("raft/storage: open WAL: %v", err),
		)
	}
	meta, st, ents, err := w.ReadAll()

	if err != nil {
		return fail(
			fmt.Errorf("raft/storage: read WAL: %v", err),
		)
	}

	d.wal = w
	return meta, st, ents, sf, nil
}

func (d *disk) Exist() bool {
	return wal.Exist(d.waldir)
}

func (d *disk) Snapshotter() storage.Snapshotter {
	return d.shoter
}

func (d *disk) Close() error {
	return d.wal.Close()
}
