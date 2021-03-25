package raft

import (
	"time"

	"github.com/shaj13/raftkit/api"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

type Member interface {
	ID() uint64
	Address() string
	Since() time.Time
	IsActive() bool
	Update(string) error
	Send(raftpb.Message) error
	Type() api.MemberType
	Close()
}
