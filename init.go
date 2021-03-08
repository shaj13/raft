package raft

import (
	"context"
	"sync"
	"time"

	"github.com/shaj13/raftkit/api"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func initPool(ctx context.Context) {
	r := registryFromCtx(ctx)
	p := r.pool
	p.cfg = r.config
	p.factory = r.factory
	p.membs = make(map[uint64]Member)
}

func initFactory(ctx context.Context) {
	r := registryFromCtx(ctx)
	f := r.factory
	f.ctx, f.cancel = context.WithCancel(ctx)
	f.reportc = r.reportc
	f.cfg = r.config
	f.constructors = r.mcons
}

func initProcessor(ctx context.Context) {
	r := registryFromCtx(ctx)
	p := r.processor
	p.ctx, p.cancel = context.WithCancel(ctx)
	p.cfg = r.config
	p.ticker = time.NewTicker(time.Second) // TODO: read second from cfg
	p.wg = sync.WaitGroup{}
	p.propwg = sync.WaitGroup{}
	p.storg = r.memoryStorage
	p.sshot = r.snapshoter
	p.wait = r.msgbus
	p.repoc = r.reportc
	p.propc = make(chan raftpb.Message)
	p.recvc = make(chan raftpb.Message)
	p.pool = r.pool
}

func initServer(ctx context.Context) {
	// TODO: init server should check if http or grpc
	r := registryFromCtx(ctx)
	s := r.server
	s.pool = r.pool
	s.processor = r.processor
	s.cfg = r.config
	s.UnimplementedRaftServer = api.UnimplementedRaftServer{}
}
