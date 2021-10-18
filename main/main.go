package main

import (
	"context"
	"flag"
	"log"
	"net"
	"time"

	raft "github.com/shaj13/raftkit"
	raftgrpc "github.com/shaj13/raftkit/rpc/grpc"
	"google.golang.org/grpc"
)

var (
	addr string
	join string
	dir  string
)

func init() {
	raftgrpc.Register(
		raftgrpc.WithDialOptions(grpc.WithInsecure()),
	)
	flag.StringVar(&addr, "raft", "", "raft server addr")
	flag.StringVar(&join, "join", "", "join cluster addr")
	flag.StringVar(&dir, "dir", "", "join cluster addr")
	flag.Parse()
}

func main() {
	var opt raft.StartOption
	if join != "" {
		opt = raft.WithFallback(
			raft.WithJoin(join, time.Second),
			raft.WithRestart(),
		)
	} else {
		opt = raft.WithFallback(
			raft.WithInitCluster(),
			raft.WithRestart(),
		)
	}
	ctx := context.Background()
	cluster, srv := raft.New(ctx, raft.WithStateDIR(dir))
	go startRaftServer(srv)
	if err := cluster.Start(opt, raft.WithAddress(":8081")); err != nil {
		panic(err)
	}
}

func startRaftServer(srv interface{}) {
	s := grpc.NewServer()
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	raftgrpc.RegisterServer(s, srv)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
