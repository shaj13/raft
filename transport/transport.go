package transport

import "github.com/shaj13/raft/internal/transport"

const (
	GRPC Proto = Proto(transport.GRPC)
	HTTP Proto = Proto(transport.HTTP)
)

// Proto is a portmanteau of protocol
// and represents the underlying RPC protocol.
type Proto uint

// Handler responds to an RPC request.
type Handler = transport.Handler
