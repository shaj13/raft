package net

import (
	"strconv"
)

const (
	GRPC Codec = iota + 1
	max
)

var registry = make([]*codecpair, max)

type codecpair struct {
	nsrv New
	dial Dial
}

// Codec is a portmanteau of coder-decoder
// and represents the underlying server and client.
type Codec uint

// Register registers a function that returns a codec server and client,
// of the given codec function.
// This is intended to be called from the init function,
// in packages that implement codec function.
func (c Codec) Register(srv New, dial Dial) {
	if c <= 0 && c >= max { //nolint:staticcheck
		panic("raft/net: Register of unknown codec function")
	}

	registry[c] = &codecpair{
		nsrv: srv,
		dial: dial,
	}
}

// Available reports whether the given codec is linked into the binary.
func (c Codec) Available() bool {
	return c > 0 && c < max && registry[c] != nil
}

// Get returns codec server and client.
// Get panics if the codec function is not linked into the binary.
func (c Codec) Get() (New, Dial) {
	if !c.Available() {
		panic("raft/net: Requested codec function #" + strconv.Itoa(int(c)) + " is unavailable")
	}
	p := registry[c]
	return p.nsrv, p.dial
}

// String returns string describes the codec function.
func (c Codec) String() string {
	switch c {
	case GRPC:
		return "GRPC"
	default:
		return "unknown codec value " + strconv.Itoa(int(c))
	}

}
