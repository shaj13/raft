package raft

import (
	"testing"
)

func Test(t *testing.T) {
	t.Error("done", cc)
}

type Event interface{}
type chann struct {
	C chan Event
}

