package membership

import (
	"sync"
	"time"

	"github.com/shaj13/raftkit/internal/raftpb"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

// local represents the current cluster member.
type local struct {
	r      Reporter
	id     uint64
	active time.Time
	mu     sync.Mutex // protects addr
	addr   string
}

func (l *local) ID() uint64 {
	return l.id
}

func (l *local) Address() string {
	return l.addr
}

func (l *local) Since() time.Time {
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
	l.addr = add
	l.mu.Unlock()
	return
}

func (l *local) Close() error {
	l.r.ReportShutdown(l.ID())
	return nil
}

func (l *local) Send(etcdraftpb.Message) (err error) { return }
