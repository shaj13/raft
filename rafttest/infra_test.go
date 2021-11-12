package rafttest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	raft "github.com/shaj13/raftkit"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/transport"
	etransport "github.com/shaj13/raftkit/transport"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

// canceledctx is a reusable canceled context.
var canceledctx context.Context

func init() {
	var cancel context.CancelFunc
	canceledctx, cancel = context.WithCancel(context.Background())
	cancel()
}

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

func (l *loopback) get(addr string) loopbackCfg {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.cfgs[addr]
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

func newNode() *node {
	return &node{
		fsm: newStateMachine(),
	}
}

func newOrchestrator(t *testing.T) *orchestrator {
	lb := newLoopback(t)
	lb.register()

	return &orchestrator{
		t:        t,
		loopback: lb,
	}
}

type node struct {
	raftnode   *raft.Node
	rawMembers []raft.RawMember
	startOpts  []raft.StartOption
	opts       []raft.Option
	fsm        *stateMachine
}

func (n *node) withRawMember(raw raft.RawMember) *node {
	n.rawMembers = append(n.rawMembers, raw)
	return n
}

func (n *node) withStartOptions(opts ...raft.StartOption) *node {
	n.startOpts = append(n.startOpts, opts...)
	return n
}

func (n *node) withOptions(opts ...raft.Option) *node {
	n.opts = append(n.opts, opts...)
	return n
}

type orchestrator struct {
	t        *testing.T
	loopback *loopback
	nodes    []*node
}

func (o *orchestrator) create(n int) []*node {
	nodes := make([]*node, n)

	for i := 1; i <= n; i++ {
		raw := raft.RawMember{
			ID:      uint64(i),
			Address: fmt.Sprintf(":%d", i),
		}

		node := newNode().withRawMember(raw).withStartOptions(raft.WithInitCluster())

		// add other nodes raws.
		for j := 1; j <= n; j++ {
			if j == i {
				continue
			}

			raw := raft.RawMember{
				ID:      uint64(j),
				Address: fmt.Sprintf(":%d", j),
			}
			node.withRawMember(raw)
		}

		nodes[i-1] = node
	}

	return nodes
}

func (o *orchestrator) start(nodes ...*node) *orchestrator {
	o.nodes = append(o.nodes, nodes...)
	for _, n := range nodes {

		membs := raft.WithMembers(n.rawMembers...)
		n.startOpts = append(n.startOpts, membs)

		if n.raftnode == nil {
			ctx := raft.WithContext(ctxWithRawMember(n.rawMembers[0]))
			state := raft.WithStateDIR(o.t.TempDir())
			n.opts = append(n.opts, ctx, state)

			n.raftnode = raft.New(n.fsm, etransport.Proto(transport.GRPC), n.opts...)
		}

		go func(n *node) {
			id := n.rawMembers[0].ID
			err := n.raftnode.Start(n.startOpts...)
			if err != nil && err != raft.ErrNodeStopped {
				o.t.Errorf("orchestrator: node %d start returned: %v", id, err)
			}
		}(n)
	}
	return o
}

func (o *orchestrator) teardown() {
	wg := sync.WaitGroup{}
	for _, n := range o.nodes {
		wg.Add(1)
		go func(n *node) {
			defer wg.Done()
			err := n.raftnode.Shutdown(canceledctx)
			if err != nil {
				o.t.Error(err)
			}
		}(n)
	}

	wg.Wait()
	o.nodes = make([]*node, 0)
}

func (o *orchestrator) waitAll() *orchestrator {
	wg := sync.WaitGroup{}
	for _, n := range o.nodes {
		wg.Add(1)
		go func(n *node) {
			defer wg.Done()
			o.wait(n)
		}(n)
	}

	wg.Wait()
	return o
}

func (o *orchestrator) wait(node *node) *orchestrator {
	for i := 0; i < 30; i++ {
		if node.raftnode.Leader() != raft.None {
			return o
		}

		time.Sleep(time.Millisecond * 500)
	}

	o.t.Errorf("orchestrator: failed to wait for node to start %d", node.rawMembers[0].ID)
	return o
}

func (o *orchestrator) leader() *node {
	for _, n := range o.nodes {
		id := n.raftnode.Whoami()
		if id == raft.None {
			continue
		}

		if n.raftnode.Leader() == id {
			return n
		}
	}

	o.t.Fatal("orchestrator: failed to find leader")
	return new(node)
}

func (o *orchestrator) follower() *node {
	for _, n := range o.nodes {
		id := n.raftnode.Leader()
		if id == raft.None {
			continue
		}

		if n.raftnode.Whoami() != id {
			return n
		}
	}

	o.t.Fatal("orchestrator: failed to find first follower")
	return new(node)
}

func (o *orchestrator) anyNode() *node {
	i := rand.Intn(len(o.nodes))
	return o.nodes[i]
}

func (o *orchestrator) produceData(n int) {
	for i := 0; i <= n; i++ {
		node := o.anyNode().raftnode
		data := newBytesEntry(i, i)
		err := node.Replicate(context.Background(), data)
		if err != nil {
			o.t.Error(err)
		}
	}
}

func newStateMachine() *stateMachine {
	return &stateMachine{
		kv: map[int]int{},
	}
}

func newBytesEntry(key, value int) []byte {
	ent := &entry{
		Key:   key,
		Value: value,
	}

	buf, err := json.Marshal(ent)
	if err != nil {
		panic(err)
	}

	return buf
}

type entry struct {
	Key   int
	Value int
}

type stateMachine struct {
	mu sync.Mutex
	kv map[int]int
}

func (s *stateMachine) Apply(data []byte) {
	var e entry
	if err := json.Unmarshal(data, &e); err != nil {
		log.Println("unable to Unmarshal entry", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.kv[e.Key] = e.Value
}

func (s *stateMachine) Snapshot() (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	buf, err := json.Marshal(&s.kv)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(strings.NewReader(string(buf))), nil
}

func (s *stateMachine) Restore(r io.ReadCloser) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	err = json.Unmarshal(buf, &s.kv)
	if err != nil {
		return err
	}

	return r.Close()
}

func (s *stateMachine) Read(key int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.kv[key]
}
