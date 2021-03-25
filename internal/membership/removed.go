package membership

import (
	"errors"
	"time"

	"github.com/shaj13/raftkit/api"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

var ErrRemovedMember = errors.New("raft/membership: member was removed")

// Removed represents the remote Removed cluster member.
type Removed struct {
	id   uint64
	addr string
}

func (r Removed) ID() uint64 {
	return r.id
}

func (r Removed) Address() string {
	return r.addr
}

func (r Removed) Send(raftpb.Message) error {
	return ErrRemovedMember
}

func (r Removed) Type() api.MemberType {
	return api.RemovedMember
}

func (r Removed) Update(string) error {
	return ErrRemovedMember
}

func (r Removed) Close()               {}
func (r Removed) Since() (t time.Time) { return }
func (r Removed) IsActive() (ok bool)  { return }
