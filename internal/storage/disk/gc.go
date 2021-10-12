package disk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shaj13/raftkit/internal/log"
	"go.etcd.io/etcd/pkg/v3/fileutil"
)

func newGC(ctx context.Context, wdir, sdir string, retain int) *gc {
	gc := new(gc)
	gc.ctx, gc.cancel = context.WithCancel(ctx)
	gc.maxsnaps = retain
	gc.waldir = wdir
	gc.snapdir = sdir
	gc.notifyc = make(chan struct{})
	gc.done = make(chan struct{})
	return gc
}

type gc struct {
	done     chan struct{}
	notifyc  chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc
	maxsnaps int
	waldir   string
	snapdir  string
}

func (gc *gc) Start() {
	go func() {
		for {
			select {
			case <-gc.notifyc:
				if err := gc.purge(); err != nil {
					log.Warnf("raft/storage/disk: purging oldest snapshots/WALs files failed, Err %v", err)
				}
			case <-gc.ctx.Done():
				close(gc.done)
				return
			}
		}
	}()
	return
}

func (gc *gc) purge() error {
	files, err := list(gc.snapdir, snapExt)
	if err != nil || len(files) < gc.maxsnaps {
		return err
	}

	// snapshots.
	var (
		current = files[0]
		oldest  string
	)

	for i, f := range files {
		if f != current && i >= gc.maxsnaps {
			path := filepath.Join(gc.snapdir, f)
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

	files, err = list(gc.waldir, walExt)
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
		path := filepath.Join(gc.waldir, files[len(files)-i-1])
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

func (gc *gc) Close() {
	gc.cancel()
	<-gc.done
}
