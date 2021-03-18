package raft

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.etcd.io/etcd/pkg/v3/fileutil"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/wal"
	"go.etcd.io/etcd/server/v3/wal/walpb"
	"go.uber.org/zap"
)

const (
	snapExt = ".snap"
	walExt  = ".wal"
	format  = "%016x-%016x"
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
	cfg     *config
	wal     *wal.WAL
	walDir  string
	snapDir string
	snap    *snap.Snapshotter
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

func (d *disk) bootstrap(b []byte) ([]byte, raftpb.HardState, []raftpb.Entry, *raftpb.Snapshot, error) {
	// TODO: get zap looger from cfg
	fail := func(err error) ([]byte, raftpb.HardState, []raftpb.Entry, *raftpb.Snapshot, error) {
		return []byte{}, raftpb.HardState{}, []raftpb.Entry{}, nil, err
	}

	if !fileutil.Exist(d.snapDir) {
		if err := os.Mkdir(d.snapDir, 0750); err != nil {
			return fail(
				fmt.Errorf("raft: failed to create snapshot dir, Err: %s", err),
			)
		}
	}

	tempsnap := snap.New(zap.NewExample(), d.snapDir)

	if !wal.Exist(d.walDir) {
		if err := os.Mkdir(d.walDir, 0750); err != nil {
			return fail(
				fmt.Errorf("raft: failed to create WAL dir, Err: %s", err),
			)
		}

		w, err := wal.Create(zap.NewExample(), d.walDir, b)
		if err != nil {
			return fail(
				fmt.Errorf("raft: failed to create WAL dir, Err: %s", err),
			)
		}

		d.wal = w
		d.snap = tempsnap
		return b, raftpb.HardState{}, []raftpb.Entry{}, nil, nil
	}

	walSnaps, err := wal.ValidSnapshotEntries(zap.NewExample(), d.walDir)

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

	w, err := wal.Open(zap.NewExample(), d.walDir, walsnap)
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
	d.snap = tempsnap

	return meta, st, ents, snapshot, nil
}

// TODO: need to be run in standlalone go
func (d *disk) clean() error {
	// TODO: move to config
	target := 3

	files, err := list(d.snapDir, snapExt)
	if err != nil || len(files) < target {
		return err
	}

	// snapshots.
	var (
		current = files[0]
		oldest  string
	)

	for i, f := range files {
		if f != current && i >= target {
			path := filepath.Join(d.snapDir, f)
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

	files, err = list(d.walDir, walExt)
	if err != nil {
		return err
	}

	var (
		mark int
	)

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

	// if there only one wal then skip the cleaning
	if mark < 1 {
		return nil
	}

	for i := 0; i < mark; i++ {
		path := filepath.Join(d.walDir, files[i])
		lock, err := fileutil.TryLockFile(path, os.O_WRONLY, fileutil.PrivateFileMode)
		if err != nil {
			return err
		}

		err = os.Remove(path)
		lock.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

func (d *disk) exist() bool {
	return wal.Exist(d.walDir)
}

func list(path, ext string) ([]string, error) {
	ls, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, f := range ls {
		if strings.HasSuffix(f.Name(), ext) {
			files = append(files, f.Name())
		}
	}

	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	return files, nil
}
