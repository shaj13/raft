package raft

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/shaj13/raftkit/api"
	"google.golang.org/grpc"
)

func New() {
	join()
	firstrun()
}

func firstrun() {
	ctx := context.Background()
	d, srv := NewNew(ctx)

	go func() {
		if err := d.Start(ctx, "", ":50051"); err != nil {
			panic(err)
		}
	}()

	s := grpc.NewServer()
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	api.RegisterRaftServer(s, srv.(api.RaftServer))
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func join() {

	ctx := context.Background()
	d, srv := NewNew(ctx)

	go func() {
		s := grpc.NewServer()
		lis, err := net.Listen("tcp", ":50052")
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}

		api.RegisterRaftServer(s, srv.(api.RaftServer))
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	time.Sleep(time.Second)
	if err := d.Start(ctx, ":50051", ":50052"); err != nil {
		panic(err)
	}
}
