package http

import (
	"github.com/shaj13/raftkit/internal/rpc"
	"github.com/shaj13/raftkit/internal/storage"
)

const (
	snapshotHeader = "X-Raft-Snapshot"
	memberIDHeader = "X-Raft-Member-ID"
	messageURI     = "/message"
	snapshotURI    = "/snapshot"
	joinURI        = "/join"
)

// DialConfig define common configuration used by the dial function.
type DialConfig interface {
	Snapshotter() storage.Snapshotter
}

// ServerConfig define common configuration used by the NewServer function.
type ServerConfig interface {
	DialConfig
	Controller() rpc.Controller
}
