package raft

import (
	"context"
	"sync"
	"time"

	"github.com/shaj13/raftkit/internal/storage/disk"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func initProcessor(ctx context.Context) {
	r := registryFromCtx(ctx)
	p := r.processor
	p.ctx, p.cancel = context.WithCancel(ctx)
	p.cfg = r.config
	p.ticker = time.NewTicker(100 * time.Millisecond) // TODO: read second from cfg
	p.wg = sync.WaitGroup{}
	p.propwg = sync.WaitGroup{}
	p.cache = r.memoryStorage
	p.storage = disk.New(ctx, r.config)
	p.msgbus = r.msgbus
	p.repoc = r.reportc
	p.propc = make(chan raftpb.Message)
	p.recvc = make(chan raftpb.Message)
	p.started = new(atomicBool)
	p.pool = r.pool
	p.cfg.snapshoter = p.storage.Snapshoter()
}

func initMsgBus(ctx context.Context) {
	r := registryFromCtx(ctx)
	r.msgbus.chans = make(map[uint64][]chan interface{})
}

func initCluster(ctx context.Context) {
	r := registryFromCtx(ctx)
	c := r.cluster
	c.pool = r.pool
	c.processor = r.processor
}
