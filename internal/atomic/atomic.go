package atomic

import (
	"strconv"
	"sync/atomic"
)

// Bool is an atomic type-safe wrapper for bool values.
type Bool uint32

// Set sets the Boolean to true.
func (a *Bool) Set() {
	atomic.StoreUint32((*uint32)(a), 1)
}

// UnSet sets the Boolean to false.
func (a *Bool) UnSet() {
	atomic.StoreUint32((*uint32)(a), 0)
}

// True returns whether the Boolean is true.
func (a *Bool) True() bool {
	return atomic.LoadUint32((*uint32)(a))&1 == 1
}

// False returns whether the Boolean is false.
func (a *Bool) False() bool {
	return !a.True()
}

// String returns a as string.
func (a *Bool) String() string {
	return strconv.FormatBool(a.True())
}

// Uint64 is an atomic type-safe wrapper for uint64 values.
type Uint64 uint64

// Add atomically adds n to u.
func (u *Uint64) Add(n uint64) {
	atomic.AddUint64((*uint64)(u), n)
}

// Get atomically gets the value of u.
func (u *Uint64) Get() uint64 {
	return atomic.LoadUint64((*uint64)(u))
}

// String returns u as string.
func (u *Uint64) String() string {
	return strconv.FormatUint(u.Get(), 10)
}
