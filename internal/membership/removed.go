package membership

import (
	"errors"
	"time"

	"github.com/shaj13/raftkit/internal/raftpb"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

var ErrRemovedMember = errors.New("raft/membership: member was removed")

// removed represents the remote removed cluster member.
type removed struct {
	raw raftpb.Member
}

func (r removed) ID() uint64 {
	return r.raw.ID
}

func (r removed) Address() string {
	return r.raw.Address
}

func (r removed) Send(etcdraftpb.Message) error {
	return ErrRemovedMember
}

func (r removed) Type() raftpb.MemberType {
	return raftpb.RemovedMember
}

func (r removed) Update(string) error {
	return ErrRemovedMember
}

func (r removed) Raw() raftpb.Member {
	return r.raw
}

func (r removed) Close() (err error)         { return }
func (r removed) ActiveSince() (t time.Time) { return }
func (r removed) IsActive() (ok bool)        { return }
