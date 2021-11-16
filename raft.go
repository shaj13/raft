package raft

import (
	"github.com/shaj13/raftkit/internal/daemon"
	"github.com/shaj13/raftkit/internal/membership"
	"github.com/shaj13/raftkit/internal/storage/disk"
	itransport "github.com/shaj13/raftkit/internal/transport"
	"github.com/shaj13/raftkit/transport"
)

func New(fsm StateMachine, proto transport.Proto, opts ...Option) *Node {
	if fsm == nil {
		panic("raft: cannot create node from nil state machine")
	}

	cfg := newConfig(opts...)
	cfg.fsm = fsm

	newHandler, dialer := itransport.Proto(proto).Get()
	cfg.controller = new(controller)
	cfg.storage = disk.New(cfg)
	cfg.dial = dialer(cfg)
	cfg.pool = membership.New(cfg)
	cfg.daemon = daemon.New(cfg)

	node := new(Node)
	node.pool = cfg.pool
	node.daemon = cfg.daemon
	node.storage = cfg.storage
	node.dial = cfg.dial
	node.cfg = cfg
	node.handler = newHandler(cfg)

	cfg.controller.(*controller).node = node
	cfg.controller.(*controller).daemon = cfg.daemon
	cfg.controller.(*controller).pool = cfg.pool
	cfg.controller.(*controller).storage = cfg.storage

	return node
}
