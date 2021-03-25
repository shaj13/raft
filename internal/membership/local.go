package membership

import (
	"sync"
	"time"

	"github.com/shaj13/raftkit/api"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

// Local represents the current cluster member.
type Local struct {
	r      reporter
	id     uint64
	active time.Time
	mu     sync.Mutex // protects addr
	addr   string
}

func (l *Local) ID() uint64 {
	return l.id
}

func (l *Local) Address() string {
	return l.addr
}

func (l *Local) Since() time.Time {
	return l.active
}

func (l *Local) IsActive() bool {
	return !l.active.IsZero()
}

func (l *Local) Type() api.MemberType {
	// TODO: change this to local.
	return api.SelfMember
}

func (l *Local) Update(add string) (err error) {
	l.mu.Lock()
	l.addr = add
	l.mu.Unlock()
	return
}

func (l *Local) Close() {
	l.r.ReportShutdown(l.ID())
}

func (l *Local) Send(raftpb.Message) (err error) { return }
