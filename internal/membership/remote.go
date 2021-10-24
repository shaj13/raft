package membership

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/shaj13/raftkit/internal/log"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/transport"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

// remote represents the remote cluster member.
type remote struct {
	ctx         context.Context
	cancel      context.CancelFunc
	id          uint64
	r           Reporter
	cfg         Config
	dial        transport.Dial
	msgc        chan etcdraftpb.Message
	done        chan struct{}
	mu          sync.Mutex // protects followings
	rc          transport.Client
	active      bool
	addr        string
	activeSince time.Time
}

func (r *remote) Type() raftpb.MemberType {
	return raftpb.RemoteMember
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
		return fmt.Errorf("cluster member %x, buffer is full (overloaded network)", r.id)
	}

	return
}

func (r *remote) Update(addr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.addr == addr || r.ctx.Err() != nil {
		return r.ctx.Err()
	}

	rc, err := r.dial(r.ctx, addr)
	if err != nil {
		return err
	}

	if err := r.rc.Close(); err != nil {
		return err
	}

	r.rc = rc
	r.addr = addr
	return nil
}

func (r *remote) Address() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.addr
}

func (r *remote) Since() time.Time {
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
	return r.id
}

func (r *remote) Close() error {
	r.cancel()
	<-r.done
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

func (r *remote) stream(ctx context.Context, msg etcdraftpb.Message) error {
	ctx, cancel := context.WithTimeout(ctx, r.cfg.StreamTimeout())
	defer cancel()

	rpc := r.client()
	err := rpc.Message(ctx, msg)
	r.report(msg, err)

	return err
}

func (r *remote) drain() error {
	ctx, cancel := context.WithTimeout(context.Background(), r.cfg.DrainTimeout())
	defer cancel()

	for {
		select {
		case msg, ok := <-r.msgc:

			if !ok {
				return nil
			}

			if err := r.stream(ctx, msg); err != nil {
				return err
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (r *remote) run() {
	defer func() {
		log.Debugf(
			"raft/membership: Member %x context done, ctx.Err: %s",
			r.id,
			r.ctx.Err(),
		)

		r.setStatus(false)
		close(r.msgc) // ctx.Done no goroutines will write to msgc.

		// drain msgc and exit
		if err := r.drain(); err != nil {
			log.Warnf(
				"raft/membership: draining the member %x message queue failed, Err: %s",
				r.id,
				err,
			)
		}

		close(r.done)
	}()

	for {
		select {
		case msg := <-r.msgc:
			err := r.stream(r.ctx, msg)
			if err != nil {
				log.Errorf(
					"raft/membership: streaming the message to member %x failed, Err: %s",
					r.id,
					err,
				)
			}
			r.setStatus(err == nil)
		case <-r.ctx.Done():
			return
		}

	}
}
