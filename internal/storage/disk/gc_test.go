package disk

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGCStart(t *testing.T) {
	dir := createTestDir("gc", t)
	defer os.RemoveAll(dir)

	files := []string{}
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf(format, i, i)
		snap := name + snapExt
		wal := name + walExt
		files = append(files, snap, wal)
	}

	createTestFiles(dir, files, t)

	// create gc
	notifyc := make(chan struct{})
	gc := newGC(context.Background(), dir, dir, 1, notifyc)
	gc.Start()
	notifyc <- struct{}{}
	gc.Close()

	snaps, _ := list(dir, snapExt)
	wals, _ := list(dir, walExt)
	assert.Equal(t, 1, len(snaps))
	assert.Equal(t, 1, len(wals))
	assert.Equal(t, snaps[0], fmt.Sprintf(format, 4, 4)+snapExt)
	assert.Equal(t, wals[0], fmt.Sprintf(format, 4, 4)+walExt)
}
