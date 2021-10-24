package transport

import (
	"strconv"
)

const (
	GRPC Proto = iota + 1
	HTTP
	max
)

var registry = make([]*protoPair, max)

type protoPair struct {
	nsrv NewServer
	dial Dialer
}

// Proto is a portmanteau of protocol
// and represents the underlying RPC protocol.
type Proto uint

// Register registers a function that returns a proto server and client,
// of the given proto function.
// This is intended to be called from the init function,
// in packages that implement proto function.
func (c Proto) Register(ns NewServer, dial Dialer) {
	if c <= 0 && c >= max { //nolint:staticcheck
		panic("raft/rpc: Register of unknown proto function")
	}

	registry[c] = &protoPair{
		nsrv: ns,
		dial: dial,
	}
}

// Available reports whether the given proto is linked into the binary.
func (c Proto) Available() bool {
	return c > 0 && c < max && registry[c] != nil
}

// Get returns proto server and client.
// Get panics if the proto function is not linked into the binary.
func (c Proto) Get() (NewServer, Dialer) {
	if !c.Available() {
		panic("raft/rpc: Requested proto function #" + strconv.Itoa(int(c)) + " is unavailable")
	}
	p := registry[c]
	return p.nsrv, p.dial
}

// String returns string describes the proto function.
func (c Proto) String() string {
	switch c {
	case GRPC:
		return "gRPC"
	default:
		return "unknown proto value " + strconv.Itoa(int(c))
	}

}
