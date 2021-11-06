package membership

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/shaj13/raftkit/internal/raftpb"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

func newLocal(_ context.Context, cfg Config, m raftpb.Member) (Member, error) {
	return &local{
		r:      cfg.Reporter(),
		active: time.Now(),
		raw:    m,
	}, nil
}

// local represents the current cluster member.
type local struct {
	r      Reporter
	active time.Time
	mu     sync.Mutex // protects raw
	raw    raftpb.Member
}

func (l *local) ID() uint64 {
	return l.Raw().ID
}

func (l *local) Address() string {
	return l.Raw().Address
}

func (l *local) ActiveSince() time.Time {
	return l.active
}

func (l *local) IsActive() bool {
	return !l.active.IsZero()
}

func (l *local) Type() raftpb.MemberType {
	return l.Raw().Type
}

func (l *local) Update(m raftpb.Member) (err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.raw = m
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

func (l *local) Raw() raftpb.Member {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.raw
}

func (l *local) TearDown(ctx context.Context) error {
	return l.Close()
}
