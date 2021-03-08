package raft

import (
	"context"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/shaj13/raftkit/api"
	"go.etcd.io/etcd/raft/v3"
	"google.golang.org/grpc"
)

type registryKey struct{}

type registry struct {
	processor     *processor
	pool          *pool
	factory       *factory
	config        *config
	reportc       chan report
	mcons         membersConstructors
	memoryStorage *raft.MemoryStorage
	snapshoter    Snapshoter
	msgbus        *wait
	server        *server
}

func (r *registry) init() *registry {
	// TODO: some of this need to have a stand alone init -- e.g
	// NewMemoryStorage should be init by taking some data from disk
	r.processor = new(processor)
	r.pool = new(pool)
	r.factory = new(factory)
	r.config = defaultConfig()
	r.reportc = make(chan report)
	r.mcons = membersConstructors{
		api.RemoteMember:  newRemote,
		api.RemovedMember: newRemoved,
	}
	r.memoryStorage = raft.NewMemoryStorage()
	r.snapshoter = &disk{
		cfg: r.config,
	}
	r.msgbus = newWait()
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
	join()
	// firstrun()
}

func firstrun() {
	r := (&registry{}).init()
	ctx := ctxWithRegistry(context.Background(), r)
	inits := []func(context.Context){
		initFactory,
		initPool,
		initProcessor,
		initServer,
	}

	for _, init := range inits {
		init(ctx)
	}

	_, _, _, err := r.snapshoter.(*disk).bootstrap()
	if err != nil {
		panic(err)
	}

	id := uint64(rand.Int63()) + 1
	go func() {
		if err := r.processor.run(id); err != nil {
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
	ctx := ctxWithRegistry(context.Background(), r)
	inits := []func(context.Context){
		initFactory,
		initPool,
		initProcessor,
		initServer,
	}

	r.config.stateDir = "/tmp/2nd/"
	for _, init := range inits {
		init(ctx)
	}

	_, _, _, err := r.snapshoter.(*disk).bootstrap()
	if err != nil {
		panic(err)
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

	time.Sleep(time.Second)
	conn, err := grpc.Dial(":50051", r.config.memberDialOptions...)
	if err != nil {
		panic(err)
	}

	c := api.NewRaftClient(conn)
	resp, err := c.Join(ctx, &api.Member{
		Address: ":50052",
	})

	if err != nil {
		panic(err)
	}

	r.pool.recover(resp.Pool)
	if err := r.processor.run(resp.ID); err != nil {
		panic(err)
	}
}
