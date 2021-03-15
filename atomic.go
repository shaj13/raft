package raft

import "sync/atomic"

type atomicBool int32

// set sets the Boolean to true.
func (a *atomicBool) set() {
	atomic.StoreInt32((*int32)(a), 1)
}

// UnSet sets the Boolean to false.
func (a *atomicBool) unSet() {
	atomic.StoreInt32((*int32)(a), 0)
}

// is returns whether the Boolean is true.
func (a *atomicBool) isSet() bool {
	return atomic.LoadInt32((*int32)(a))&1 == 1
}
