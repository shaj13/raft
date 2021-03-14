package raft

import (
	"testing"
)

func Test(t *testing.T) {
	id := MemberID(12)
	buf, _ := id.Marshal()
	mm := MemberID(0)
	mm.Unmarshal(buf)
	t.Errorf("%d, %s", mm, mm)
}

type Event interface{}
type chann struct {
	C chan Event
}
