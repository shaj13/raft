package raft

import (
	"context"
	"log"
	"net"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/membership"
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
	snapshoter    Snapshoter
	msgbus        *msgbus
	server        *server
	cluster       *cluster
}

func (r *registry) init() *registry {
	// TODO: some of this need to have a stand alone init -- e.g
	// NewMemoryStorage should be init by taking some data from disk
	r.processor = new(processor)
	r.reportc = make(chan report)
	r.pool = membership.New(context.Background(), reporter{reprotc: r.reportc}, defaultConfig(), dial)
	r.config = defaultConfig()
	r.cluster = new(cluster)
	r.memoryStorage = raft.NewMemoryStorage()
	r.snapshoter = new(disk)
	r.msgbus = new(msgbus)
	r.server = new(server)
	return r
}

func ctxWithRegistry(ctx context.Context, r *registry) context.Context {
	return context.WithValue(ctx, registryKey{}, r)
}

func registryFromCtx(ctx context.Context) *registry {
	return ctx.Value(registryKey{}).(*registry)
}

func New() {
	// join()
	firstrun()
}

func firstrun() {
	r := (&registry{}).init()
	ctx := ctxWithRegistry(context.Background(), r)
	inits := []func(context.Context){
		initMsgBus,
		initProcessor,
		initCluster,
		initServer,
		initDisk,
	}

	for _, init := range inits {
		init(ctx)
	}

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
	api.RegisterRaftServer(s, r.server)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func join() {
	r := (&registry{}).init()
	r.config.stateDir = "/tmp/3nd/"

	ctx := ctxWithRegistry(context.Background(), r)
	inits := []func(context.Context){
		initMsgBus,
		initProcessor,
		initCluster,
		initServer,
		initDisk,
	}

	for _, init := range inits {
		init(ctx)
	}

	go func() {
		s := grpc.NewServer()
		lis, err := net.Listen("tcp", ":50052")
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}
		api.RegisterRaftServer(s, r.server)
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// time.Sleep(time.Second * 100)
	if err := r.processor.run(ctx, ":50051", ":50052"); err != nil {
		panic(err)
	}
}
