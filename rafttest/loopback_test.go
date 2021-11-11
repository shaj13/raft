package rafttest_test

import (
	"context"
	"io"
	"sync"
	"testing"

	raft "github.com/shaj13/raftkit"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/transport"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

type rawMemberkey struct{}

func ctxWithRawMember(raw raft.RawMember) context.Context {
	return context.WithValue(context.Background(), rawMemberkey{}, raw)
}

func newLoopback(t *testing.T) *loopback {
	return &loopback{
		t:    t,
		cfgs: map[string]loopbackCfg{},
	}
}

type loopbackCfg interface {
	Context() context.Context
	transport.HandlerConfig
}

type loopback struct {
	t    *testing.T
	mu   sync.Mutex
	cfgs map[string]loopbackCfg
}

func (l *loopback) dialer(dc transport.DialerConfig) transport.Dial {
	cfg, ok := dc.(loopbackCfg)
	if !ok {
		l.t.Fatal("transport.DialerConfig does not implement loopback transport cfg")
	}

	v := cfg.Context().Value(rawMemberkey{})
	raw, ok := v.(raft.RawMember)
	if !ok {
		l.t.Fatal("ctx does not have raw member")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.cfgs[raw.Address] = cfg
	return func(ctx context.Context, addr string) (transport.Client, error) {
		l.mu.Lock()
		defer l.mu.Unlock()

		lc := &loopbackClient{
			from: l.cfgs[raw.Address],
			to:   l.cfgs[addr],
		}
		return lc, nil
	}
}

func (l *loopback) register() {
	fn := func(transport.HandlerConfig) (h transport.Handler) {
		return
	}

	transport.GRPC.Register(fn, l.dialer)
}

type loopbackClient struct {
	from loopbackCfg
	to   loopbackCfg
}

func (l *loopbackClient) Message(ctx context.Context, msg etcdraftpb.Message) error {
	if msg.Type == etcdraftpb.MsgSnap {
		meta := msg.Snapshot.Metadata
		r, err := l.from.Snapshotter().Reader(meta.Term, meta.Index)
		if err != nil {
			return err
		}

		w, err := l.to.Snapshotter().Writer(meta.Term, meta.Index)
		if err != nil {
			return err
		}

		_, err = io.Copy(w, r)
		if err != nil {
			return err
		}

		if err := r.Close(); err != nil {
			return err
		}

		if err := w.Close(); err != nil {
			return err
		}
	}

	return l.to.Controller().Push(ctx, msg)
}

func (l *loopbackClient) Join(ctx context.Context, mem raftpb.Member) (*raftpb.JoinResponse, error) {
	return l.to.Controller().Join(ctx, &mem)
}

func (l *loopbackClient) PromoteMember(ctx context.Context, mem raftpb.Member) error {
	return l.to.Controller().PromoteMember(ctx, mem)
}

func (l *loopbackClient) Close() error {
	return nil
}
