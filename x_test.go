package raft

import (
	"testing"
)

func Test(t *testing.T) {
	d := new(disk)
	d.snapDir = "/root/etcd/contrib/raftexample/raftexample-1-snap"
	d.walDir = "/root/etcd/contrib/raftexample/raftexample-1"
	err := d.clean()
	t.Error(err)
	// <-ch
}
