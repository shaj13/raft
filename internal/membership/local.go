package membership

import (
	"sync"
	"time"

	"github.com/shaj13/raftkit/api"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

// local represents the current cluster member.
type local struct {
	r      reporter
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

func (l *local) Type() api.MemberType {
	// TODO: change this to local.
	return api.SelfMember
}

func (l *local) Update(add string) (err error) {
	l.mu.Lock()
	l.addr = add
	l.mu.Unlock()
	return
}

func (l *local) Close() {
	l.r.ReportShutdown(l.ID())
}

func (l *local) Send(raftpb.Message) (err error) { return }
