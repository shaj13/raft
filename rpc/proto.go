package net

import "github.com/shaj13/raftkit/internal/rpc"

const (
	GRPC Proto = Proto(rpc.GRPC)
)

// Proto is a portmanteau of protocol
// and represents the underlying RPC protocol.
type Proto uint
