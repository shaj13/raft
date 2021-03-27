package net

import (
	"context"
	"strconv"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/storage"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

const (
	GRPC Codec = iota + 1
	max
)

var registry = make([]*codecpair, max)

type Server interface{}

// Dial connects to the RPC address.
type Dial func(ctx context.Context, cfg interface{}, addr string) (RPC, error)

// RPC provides access to the exported methods of an object across a network.
type RPC interface {
	Message(context.Context, raftpb.Message) error
	Join(context.Context, api.Member) (uint64, api.Pool, error)
	Close() error
}

type Controller interface {
	Snapshoter() storage.Snapshoter
	Push(context.Context, raftpb.Message) error
	Join(context.Context, *api.Member) (uint64, []api.Member, error)
}

type codecpair struct {
	srv  Server
	dial Dial
}

type Codec uint

// Register registers a function that returns a codec server and client dial,
// of the given codec function.
// This is intended to be called from the init function,
// in packages that implement codec function.
func (c Codec) Register(srv Server, dial Dial) {
	if c <= 0 && c >= max { //nolint:staticcheck
		panic("raft/net: Register of unknown codec function")
	}

	registry[c] = &codecpair{
		srv:  srv,
		dial: dial,
	}
}

// Available reports whether the given codec is linked into the binary.
func (c Codec) Available() bool {
	return c > 0 && c < max && registry[c] != nil
}

// Get returns codec server and client dial.
// Get panics if the codec function is not linked into the binary.
func (c Codec) Get(cap int) (Server, Dial) {
	if !c.Available() {
		panic("raft/net: Requested codec function #" + strconv.Itoa(int(c)) + " is unavailable")
	}
	p := registry[c]
	return p.srv, p.dial
}

// String returns string describes the rpc codec function.
func (c Codec) String() string {
	switch c {
	case GRPC:
		return "GRPC"
	default:
		return "unknown codec value " + strconv.Itoa(int(c))
	}

}
