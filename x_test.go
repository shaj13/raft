package raft

import (
	"testing"
	"time"
)

func Test(t *testing.T) {
	m := new(msgbus)
	m.chans = make(map[uint64][]chan interface{})
	m.cancel(1)
	time.Sleep(time.Second)
	// <-ch
}
