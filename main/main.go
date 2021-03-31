package main

import (
	"context"
	"flag"
	"log"
	"net"

	raft "github.com/shaj13/raftkit"
	"github.com/shaj13/raftkit/api"
	"google.golang.org/grpc"
)

var (
	addr string
	join string
)

func init() {
	flag.StringVar(&addr, "raft", "", "raft server addr")
	flag.StringVar(&join, "join", "", "join cluster addr")
	flag.Parse()
}

func main() {
	ctx := context.Background()
	cluster, srv := raft.New(ctx)
	go startRaftServer(srv.(api.RaftServer))
	if err := cluster.Join(ctx, join, addr); err != nil {
		panic(err)
	}
}

func startRaftServer(srv api.RaftServer) {
	s := grpc.NewServer()
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	api.RegisterRaftServer(s, srv.(api.RaftServer))
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
