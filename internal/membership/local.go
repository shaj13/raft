package membership

import (
	"log"
	"sync"
	"time"

	"github.com/shaj13/raftkit/internal/raftpb"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

// local represents the current cluster member.
type local struct {
	r      Reporter
	active time.Time
	mu     sync.Mutex // protects raw
	raw    *raftpb.Member
}

func (l *local) ID() uint64 {
	return l.raw.ID
}

func (l *local) Address() string {
	return l.raw.Address
}

func (l *local) ActiveSince() time.Time {
	return l.active
}

func (l *local) IsActive() bool {
	return !l.active.IsZero()
}

func (l *local) Type() raftpb.MemberType {
	return raftpb.LocalMember
}

func (l *local) Update(add string) (err error) {
	l.mu.Lock()
	l.raw.Address = add
	l.mu.Unlock()
	return
}

func (l *local) Close() error {
	l.r.ReportShutdown(l.ID())
	return nil
}

func (l *local) Send(etcdraftpb.Message) error {
	log.Panic("raft.membership: attempted to send msg to local member; should never happen")
	return nil
}
