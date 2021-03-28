package raft

import (
	"context"
	"log"
	"net"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/membership"
	rgrpc "github.com/shaj13/raftkit/internal/net/grpc"

	"go.etcd.io/etcd/raft/v3"
	"google.golang.org/grpc"
)

type registryKey struct{}

type registry struct {
	processor     *processor
	pool          *membership.Pool
	config        *config
	reportc       chan report
	memoryStorage *raft.MemoryStorage
	msgbus        *msgbus
	cluster       *cluster
}

func (r *registry) init() *registry {
	// TODO: some of this need to have a stand alone init -- e.g
	// NewMemoryStorage should be init by taking some data from disk
	r.processor = new(processor)
	r.reportc = make(chan report)
	r.config = defaultConfig()
	r.cluster = new(cluster)
	r.memoryStorage = raft.NewMemoryStorage()
	r.msgbus = new(msgbus)
	return r
}

func ctxWithRegistry(ctx context.Context, r *registry) context.Context {
	return context.WithValue(ctx, registryKey{}, r)
}

func registryFromCtx(ctx context.Context) *registry {
	return ctx.Value(registryKey{}).(*registry)
}

func New() {
	join()
	firstrun()
}

func firstrun() {
	r := (&registry{}).init()
	ctx := ctxWithRegistry(context.Background(), r)
	inits := []func(context.Context){
		initMsgBus,
		initProcessor,
		initCluster,
	}

	for _, init := range inits {
		init(ctx)
	}

	r.pool = membership.New(context.Background(), reporter{reprotc: r.reportc}, defaultConfig(), dial(r.config))
	r.processor.pool = r.pool
	r.cluster.pool = r.pool

	go func() {
		if err := r.processor.run(ctx, "", ":50051"); err != nil {
			panic(err)
		}
	}()

	s := grpc.NewServer()
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	capi := new(capi)
	capi.c = r.cluster
	capi.p = r.processor
	capi.pool = r.pool
	r.config.controller = capi
	srv, _ := rgrpc.NewServer(context.Background(), r.config)
	api.RegisterRaftServer(s, srv.(api.RaftServer))
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func join() {
	r := (&registry{}).init()
	r.config.statedir = "/tmp/3nd/"

	ctx := ctxWithRegistry(context.Background(), r)
	inits := []func(context.Context){
		initMsgBus,
		initProcessor,
		initCluster,
	}

	for _, init := range inits {
		init(ctx)
	}

	r.pool = membership.New(context.Background(), reporter{reprotc: r.reportc}, defaultConfig(), dial(r.config))
	r.processor.pool = r.pool
	r.cluster.pool = r.pool

	go func() {
		s := grpc.NewServer()
		lis, err := net.Listen("tcp", ":50052")
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}
		capi := new(capi)
		capi.c = r.cluster
		capi.p = r.processor
		capi.pool = r.pool
		r.config.controller = capi
		srv, _ := rgrpc.NewServer(context.Background(), r.config)

		api.RegisterRaftServer(s, srv.(api.RaftServer))
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// time.Sleep(time.Second * 100)
	if err := r.processor.run(ctx, ":50051", ":50052"); err != nil {
		panic(err)
	}
}
