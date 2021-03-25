package raft

import (
	"log"
	"os"
	"time"

	"go.etcd.io/etcd/raft/v3"
	"google.golang.org/grpc"
)

type config struct {
	logger            Logger
	streamTimeOut     time.Duration // 10s
	drainTimeOut      time.Duration // 10s
	memberDialOptions []grpc.DialOption
	stateDir          string
	snapInterval      uint64
}

func (c *config) StreamTimeout() time.Duration {
	return c.streamTimeOut
}

func (c *config) DrainTimeout() time.Duration {
	return c.drainTimeOut
}

func defaultConfig() *config {
	return &config{
		logger:        &raft.DefaultLogger{Logger: log.New(os.Stderr, "raft", log.LstdFlags)},
		streamTimeOut: time.Second * 10,
		drainTimeOut:  time.Second * 10,
		snapInterval:  1000,
		stateDir:      "/tmp/",
		memberDialOptions: []grpc.DialOption{
			grpc.WithInsecure(),
		},
	}
}
