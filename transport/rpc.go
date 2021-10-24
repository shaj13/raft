package transport

import "github.com/shaj13/raftkit/internal/transport"

const (
	GRPC Proto = Proto(transport.GRPC)
	HTTP Proto = Proto(transport.HTTP)
)

// Proto is a portmanteau of protocol
// and represents the underlying RPC protocol.
type Proto uint

// Server represents an RPC Server.
type Server = transport.Server