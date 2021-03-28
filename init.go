package raft

import (
	"context"
	"path/filepath"
	"sync"
	"time"

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
	p.storage = r.snapshoter.(*disk)
	p.msgbus = r.msgbus
	p.repoc = r.reportc
	p.propc = make(chan raftpb.Message)
	p.recvc = make(chan raftpb.Message)
	p.started = new(atomicBool)
	p.pool = r.pool
}

func initServer(ctx context.Context) {
	// TODO: init server should check if http or grpc
	// r := registryFromCtx(ctx)
	// s := r.server
	// s.pool = r.pool
	// s.processor = r.processor
	// s.cfg = r.config
	// s.cluster = r.cluster
	// s.UnimplementedRaftServer = api.UnimplementedRaftServer{}
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

func initDisk(ctx context.Context) {
	r := registryFromCtx(ctx)
	cfg := r.config
	d := r.snapshoter.(*disk)
	d.cfg = cfg
	d.walDir = filepath.Join(cfg.stateDir, "wal")
	d.snapDir = filepath.Join(cfg.stateDir, "snap")
}
