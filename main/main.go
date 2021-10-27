package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	raft "github.com/shaj13/raftkit"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/transport"
	raftgrpc "github.com/shaj13/raftkit/transport/grpc"
	"google.golang.org/grpc"
	// "google.golang.org/grpc"
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
	var opt2 raft.StartOption
	if join != "" {
		opt = raft.WithFallback(
			raft.WithJoin(join, time.Second),
			raft.WithRestart(),
		)
		opt2 = raft.WithMembers(raft.RawMember{
			ID:      3,
			Address: addr,
			Type:    raftpb.LocalLearnerMember,
		})
	} else {
		opt = raft.WithFallback(
			raft.WithInitCluster(),
			raft.WithRestart(),
		)
		opt2 = raft.WithAddress(addr)
	}

	cluster, srv := raft.New(transport.GRPC, raft.WithStateDIR(dir), raft.WithDisableProposalForwarding())
	go func() {
		if join == "" {
			return
		}

		time.Sleep(time.Second * 5)
		err := cluster.PromoteMember(context.Background(), 3)
		fmt.Println("####", err)
	}()

	go startRaftServer(srv)
	if err := cluster.Start(opt, opt2); err != nil {
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

	// h := rafthttp.Handler(srv)
	// if err := http.ListenAndServe(strings.TrimPrefix(addr, "http://"), h); err != nil {
	// 	log.Fatalf("failed to serve: %v", err)
	// }
}
