package membership

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/log"
	"github.com/shaj13/raftkit/internal/net"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

// remote represents the remote cluster member.
type remote struct {
	ctx         context.Context
	cancel      context.CancelFunc
	id          uint64
	r           Reporter
	cfg         config
	dial        net.Dial
	msgc        chan raftpb.Message
	done        chan struct{}
	mu          sync.Mutex // protects followings
	rpc         net.RPC
	active      bool
	addr        string
	activeSince time.Time
}

func (r *remote) Type() api.MemberType {
	return api.RemoteMember
}

func (r *remote) Send(msg raftpb.Message) (err error) {
	defer func() {
		if err != nil {
			r.report(msg, err)
		}
	}()

	select {
	case r.msgc <- msg:
	case <-r.ctx.Done():
		return r.ctx.Err()
	default:
		return fmt.Errorf("Cluster member %x, buffer is full (overloaded network)", r.id)

	}

	return
}

func (r *remote) Update(addr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.addr == addr {
		return nil
	}

	rpc, err := r.dial(r.ctx, addr)
	if err != nil {
		return err
	}

	r.rpc.Close()
	r.rpc = rpc
	r.addr = addr
	return nil
}

func (r *remote) Address() string {
	r.mu.Lock()
	addr := r.addr
	r.mu.Unlock()
	return addr
}

func (r *remote) Since() time.Time {
	r.mu.Lock()
	acts := r.activeSince
	r.mu.Unlock()
	return acts
}

func (r *remote) IsActive() bool {
	r.mu.Lock()
	act := r.active
	r.mu.Unlock()
	return act
}

func (r *remote) ID() uint64 {
	return r.id
}

func (r *remote) Close() {
	r.cancel()
	<-r.done
	r.RPC().Close()
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

func (r *remote) report(msg raftpb.Message, err error) {
	switch {
	case err == nil && msg.Type == raftpb.MsgSnap:
		r.r.ReportSnapshot(r.ID(), raft.SnapshotFinish)
	case err != nil && msg.Type == raftpb.MsgSnap:
		r.r.ReportSnapshot(r.ID(), raft.SnapshotFailure)
	case err != nil:
		r.r.ReportUnreachable(r.ID())
	}
}

func (r *remote) RPC() net.RPC {
	r.mu.Lock()
	rpc := r.rpc
	r.mu.Unlock()
	return rpc
}

func (r *remote) stream(ctx context.Context, msg raftpb.Message) error {
	ctx, cancel := context.WithTimeout(ctx, r.cfg.StreamTimeout())
	defer cancel()
	rpc := r.RPC()
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
	for {
		// check ctx to exist immediately,
		// otherwise, will continue to send msg without drain timeouts.
		if r.ctx.Err() != nil {
			break
		}

		select {
		case msg := <-r.msgc:
			err := r.stream(r.ctx, msg)
			if err != nil {
				log.Errorf(
					"raft/membership: An error occurred while streaming the message to member %x, Err: %s",
					r.id,
					err,
				)
			}
			r.setStatus(err == nil)
		case <-r.ctx.Done():
			break
		}

	}

	log.Debugf(
		"raft/membership: Member %x context done, ctx.Err: %s",
		r.id,
		r.ctx.Err(),
	)

	r.mu.Lock()
	defer r.mu.Unlock()
	r.active = false
	r.activeSince = time.Time{}
	close(r.msgc)

	// drain msgc and exit
	if err := r.drain(); err != nil {
		log.Warnf(
			"raft/membership: An error occurred while draining the member %x message queue, Err: %s",
			r.id,
			err,
		)
	}

	close(r.done)
	return
}
