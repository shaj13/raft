package raft

import (
	"github.com/shaj13/raftkit/internal/daemon"
	"github.com/shaj13/raftkit/internal/membership"
	"github.com/shaj13/raftkit/internal/storage/disk"
	itransport "github.com/shaj13/raftkit/internal/transport"
	"github.com/shaj13/raftkit/transport"
)

func New(proto transport.Proto, opts ...Option) *Node {
	cfg := newConfig(opts...)
	ctx := cfg.ctx
	newHandler, dialer := itransport.Proto(proto).Get()
	cfg.controller = new(controller)
	cfg.storage = disk.New(ctx, cfg)
	cfg.dial = dialer(ctx, cfg)
	cfg.pool = membership.New(ctx, cfg)
	cfg.daemon = daemon.New(ctx, cfg)

	node := new(Node)
	node.pool = cfg.pool
	node.daemon = cfg.daemon
	node.storage = cfg.storage
	node.dial = cfg.dial
	node.disableForwarding = cfg.rcfg.DisableProposalForwarding
	node.handler = newHandler(ctx, cfg)

	cfg.controller.(*controller).node = node
	cfg.controller.(*controller).daemon = cfg.daemon
	cfg.controller.(*controller).pool = cfg.pool

	return node
}
