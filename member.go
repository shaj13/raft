package raft

import (
	"time"

	"github.com/shaj13/raftkit/internal/raftpb"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

type Member interface {
	ID() uint64
	Address() string
	ActiveSince() time.Time
	IsActive() bool
	Update(raftpb.Member) error
	Send(etcdraftpb.Message) error
	Type() raftpb.MemberType
	Close() error
}
