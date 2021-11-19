package raft

import (
	"time"

	"github.com/shaj13/raft/internal/raftpb"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

// TODO: need to cut some of methods.
type Member interface {
	ID() uint64
	Address() string
	ActiveSince() time.Time
	IsActive() bool
	Update(raftpb.Member) error
	Send(etcdraftpb.Message) error
	Type() raftpb.MemberType
	Raw() raftpb.Member
	Close() error
}
