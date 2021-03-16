package raft

import (
	"math/rand"
	"testing"
)

func Test(t *testing.T) {
	id := uint64(rand.Int63()) + 1
	t.Error(id)
	// <-ch
}
