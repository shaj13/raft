package membership

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shaj13/raft/internal/log"
	"github.com/shaj13/raft/internal/raftpb"
	"github.com/shaj13/raft/internal/transport"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

func newRemote(cfg Config, m raftpb.Member) (Member, error) {
	ctx := cfg.Context()

	rpc, err := cfg.Dial()(ctx, m.Address)
	if err != nil {
		return nil, err
	}

	mem := new(remote)
	mem.ctx, mem.cancel = context.WithCancel(ctx)
	mem.rc = rpc
	mem.cfg = cfg
	mem.r = cfg.Reporter()
	mem.dial = cfg.Dial()
	mem.msgc = make(chan etcdraftpb.Message, 4096)
	mem.done = make(chan struct{})
	mem.active = true
	mem.activeSince = time.Now()
	mem.raw.Store(m)
	go mem.process(mem.ctx)

	return mem, nil
}

// remote represents the remote cluster member.
type remote struct {
	ctx         context.Context
	cancel      context.CancelFunc
	r           Reporter
	cfg         Config
	dial        transport.Dial
	msgc        chan etcdraftpb.Message
	done        chan struct{}
	mu          sync.Mutex // protects following fields
	raw         atomic.Value
	active      bool
	rc          transport.Client
	activeSince time.Time
}

func (r *remote) Raw() raftpb.Member {
	return r.raw.Load().(raftpb.Member)
}

func (r *remote) Type() raftpb.MemberType {
	return r.Raw().Type
}

func (r *remote) Send(msg etcdraftpb.Message) (err error) {
	defer func() {
		if err != nil {
			r.report(msg, err)
		}
	}()

	if err := r.ctx.Err(); err != nil {
		return err
	}

	select {
	case r.msgc <- msg:
	case <-r.ctx.Done():
		return r.ctx.Err()
	default:
		return fmt.Errorf("cluster member %x, buffer is full (overloaded network)", r.ID())
	}

	return
}

func (r *remote) Update(m raftpb.Member) error {

	if r.Raw().Address == m.Address || r.ctx.Err() != nil {
		r.raw.Store(m)
		return r.ctx.Err()
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	rc, err := r.dial(r.ctx, m.Address)
	if err != nil {
		return err
	}

	if err := r.rc.Close(); err != nil {
		return err
	}

	r.rc = rc
	r.raw.Store(m)
	return nil
}

func (r *remote) Address() string {
	return r.Raw().Address
}

func (r *remote) ActiveSince() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.activeSince
}

func (r *remote) IsActive() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.active
}

func (r *remote) ID() uint64 {
	return r.Raw().ID
}

func (r *remote) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), r.cfg.DrainTimeout())
	defer cancel()
	return r.TearDown(ctx)
}

func (r *remote) TearDown(ctx context.Context) error {
	r.cancel()
	r.setStatus(false)
	close(r.msgc)  // ctx.Done no goroutines will write to msgc.
	r.process(ctx) // drain msgc
	return r.client().Close()
}

func (r *remote) setStatus(active bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch {
	case !r.active && active:
		r.activeSince = time.Now()
		r.active = true
	case r.active && !active:
		r.activeSince = time.Time{}
		r.active = false
	}
}

func (r *remote) report(msg etcdraftpb.Message, err error) {
	switch {
	case err == nil && msg.Type == etcdraftpb.MsgSnap:
		r.r.ReportSnapshot(r.ID(), raft.SnapshotFinish)
	case err != nil && msg.Type == etcdraftpb.MsgSnap:
		r.r.ReportSnapshot(r.ID(), raft.SnapshotFailure)
	case err != nil:
		r.r.ReportUnreachable(r.ID())
	}
}

func (r *remote) client() transport.Client {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rc
}

func (r *remote) process(ctx context.Context) {
	for msg := range r.msgc {
		if err := ctx.Err(); err != nil {
			return
		}

		ctx, cancel := context.WithTimeout(ctx, r.cfg.StreamTimeout())
		rpc := r.client()
		err := rpc.Message(ctx, msg)
		if err != nil {
			log.Errorf("raft.membership: sending message to member %x: %v", r.ID(), err)
		}
		r.report(msg, err)
		r.setStatus(err == nil)
		cancel()
	}
}
