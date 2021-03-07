package raft

import (
	"context"
	"errors"
	"sync"
	"time"

	"google.golang.org/grpc"
)

var errUnreachable = errors.New("raftkit: Member is unreachable")

type member struct {
	id     uint64
	ctx    context.Context
	cancel context.CancelFunc
	// replace me with raftpb.Message
	msgc        chan struct{}
	done        chan struct{}
	cc          *grpc.ClientConn
	addr        string
	mu          sync.Mutex // protects followings
	active      bool
	failure     int
	activeSince time.Time
}

// investage do i need to return the conn or just keep it.
// func (m *member) conn() *grpc.ClientConn {
// 	return m.cc
// }

func (m *member) Send(msg struct{}) (err error) {
	m.mu.Lock()
	defer func() {
		if err != nil {
			m.active = false
			m.activeSince = time.Time{}
		}
		m.mu.Unlock()
	}()

	if err := m.ctx.Err(); err != nil {
		return err
	}

	select {
	case m.msgc <- msg:
	case <-m.ctx.Done():
		return m.ctx.Err()
	default:
		return errUnreachable
	}

	return nil
}

func (m *member) stream(ctx context.Context, msgs []struct{}) {
	ctx, cancel := context.WithTimeout(ctx, 16*time.Second) // TODO: read me from cfg
	defer cancel()
	
}

func (m *member) drain() error {
	ctx, cancel := context.WithTimeout(context.Background(), 16*time.Second) // TODO: read me from cfg
	defer cancel()
	for {
		select {
		case _, ok := <-m.msgc: //TODO: send msg over grpc
			if !ok {
				return nil
			}
			// if err := p.sendProcessMessage(ctx, m); err != nil {
			// 	return errors.Wrap(err, "send drain message")
			// }
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (m *member) Address() string {
	// m.mu.Lock()
	// addr := m.addr
	// m.mu.Unlock()
	return m.addr
}

func (m *member) Close() {
	m.cancel()
	<-m.done
}
