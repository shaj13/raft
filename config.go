package raft

import (
	"log"
	"os"
	"time"

	"github.com/shaj13/raftkit/internal/net"
	"github.com/shaj13/raftkit/internal/storage"
	"go.etcd.io/etcd/raft/v3"
	"google.golang.org/grpc"
)

type config struct {
	logger            Logger
	streamTimeOut     time.Duration // 10s
	drainTimeOut      time.Duration // 10s
	memberDialOptions []grpc.DialOption
	statedir          string
	maxSnapshotFiles  int
	snapInterval      uint64
	snapshoter        storage.Snapshoter
	controller        net.Controller
}

func (c *config) StreamTimeout() time.Duration {
	return c.streamTimeOut
}

func (c *config) DrainTimeout() time.Duration {
	return c.drainTimeOut
}

func (c *config) CallOption() []grpc.CallOption {
	return []grpc.CallOption{}
}

func (c *config) DialOption() []grpc.DialOption {
	return []grpc.DialOption{grpc.WithInsecure()}
}

func (c *config) Snapshoter() storage.Snapshoter {
	return c.snapshoter
}

func (c *config) StateDir() string {
	return c.statedir
}

func (c *config) MaxSnapshotFiles() int {
	return c.maxSnapshotFiles
}

func (c *config) Controller() net.Controller {
	return c.controller
}

func defaultConfig() *config {
	return &config{
		logger:           &raft.DefaultLogger{Logger: log.New(os.Stderr, "raft", log.LstdFlags)},
		streamTimeOut:    time.Second * 10,
		drainTimeOut:     time.Second * 10,
		maxSnapshotFiles: 5,
		snapInterval:     1,
		statedir:         "/tmp/",
		memberDialOptions: []grpc.DialOption{
			grpc.WithInsecure(),
		},
	}
}
