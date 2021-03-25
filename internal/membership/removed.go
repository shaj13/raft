package membership

import (
	"errors"
	"time"

	"github.com/shaj13/raftkit/api"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

var ErrRemovedMember = errors.New("raft/membership: member was removed")

// removed represents the remote removed cluster member.
type removed struct {
	id   uint64
	addr string
}

func (r removed) ID() uint64 {
	return r.id
}

func (r removed) Address() string {
	return r.addr
}

func (r removed) Send(raftpb.Message) error {
	return ErrRemovedMember
}

func (r removed) Type() api.MemberType {
	return api.RemovedMember
}

func (r removed) Update(string) error {
	return ErrRemovedMember
}

func (r removed) Close()               {}
func (r removed) Since() (t time.Time) { return }
func (r removed) IsActive() (ok bool)  { return }
