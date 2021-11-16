package daemon

import (
	"context"

	"github.com/shaj13/raftkit/internal/log"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

const (
	// add nodeState into the multiplexer.
	add operationType = iota
	// remove nodeState from the multiplexer.
	remove
	// call a method on raft raw node.
	call
	// advance raft raw node.
	advance
)

// operationType specifies the type of operation that multiplexer need to performs.
type operationType uint

// callFunc is the type of the function called by multiplexer to call the desired raw node method.
type callFunc func(rn *raft.RawNode) error

// operation define the method by which a multiplexer performs the requested function.
type operation struct {
	// ot specifies the operation type.
	ot operationType
	// gid specifies group id.
	gid uint64
	// value specifies the operation request/reply content.
	value interface{}
	// done notify the caller that mux has process the op.
	done chan struct{}
}

// nodeState represents the internal state of a RawNode object. All variables here
// are accessible only from the mux.start goroutine so they can be accessed without
// synchronization.
type nodeState struct {
	rn     *raft.RawNode
	cfg    *raft.Config
	lead   uint64
	readyc chan raft.Ready
}

// mux represents a multi node state that is participating in multiple consensus groups,
// a mux is more efficient than a collection of nodes.
// the name mux stands for "multiplexer". Like the standard "http.ServeMux".
type mux struct {
	operationc chan *operation
	stop       chan struct{}
	done       chan struct{}
}

func (m *mux) Stop() {
	// send stop signal.
	close(m.stop)
	// wait for mux to be terminated.
	<-m.done
}

func (m *mux) Start() {

	defer close(m.done)
	nodes := map[uint64]*nodeState{}
	advcs := map[uint64]raft.Ready{}
	nodeID := raft.None

	for {
		if len(advcs) == 0 {
			for gid, n := range nodes {
				st := n.rn.BasicStatus()
				if n.lead != st.Lead {
					if st.Lead != 0 {
						if n.lead == 0 {
							log.Infof("raft.node: %x elected leader %x for group %x at term %d", st.ID, st.Lead, gid, st.Term)
						} else {
							log.Infof("raft.node: %x changed group %x leader from %x to %x at term %d", st.ID, gid, n.lead, st.Lead, st.Term)
						}
					} else {
						log.Infof("raft.node: %x lost group %x leader(%x) at term %d", st.ID, gid, n.lead, st.Term)
					}
					n.lead = st.Lead
				}

				if n.rn.HasReady() {
					rd := n.rn.Ready()
					advcs[gid] = rd
					n.readyc <- rd
				}
			}
		}

		var node *nodeState

		select {
		case op := <-m.operationc:
			node = nodes[op.gid]
			if node == nil && op.ot != add {
				log.Warnf("raft: unknown group %x", op.gid)
				close(op.done)
				continue
			}

			switch op.ot {
			case add:
				node = op.value.(*nodeState)
				id := node.cfg.ID
				if id != nodeID && nodeID != raft.None {
					log.Panic("raft: all node group must have the same id !!")
				}
				nodes[op.gid] = node
				nodeID = id
			case remove:
				delete(nodes, op.gid)
				delete(advcs, op.gid)
			case call:
				err := op.value.(callFunc)(node.rn)
				op.value = err
			case advance:
				rd := advcs[op.gid]
				node.rn.Advance(rd)
				delete(advcs, op.gid)
			}

			close(op.done)

		case <-m.stop:
			return
		}
	}
}

func (m *mux) add(gid uint64, rn *raft.RawNode, cfg *raft.Config) raft.Node {
	node := &nodeState{
		rn:     rn,
		cfg:    cfg,
		readyc: make(chan raft.Ready, 128),
	}

	op := &operation{
		ot:    add,
		gid:   gid,
		value: node,
	}

	_ = m.push(context.Background(), op)

	return &muxNode{
		gid:    gid,
		mux:    m,
		readyc: node.readyc,
	}
}

func (m *mux) remove(gid uint64) {
	op := &operation{
		ot:  remove,
		gid: gid,
	}

	_ = m.push(context.Background(), op)
}

func (m *mux) call(ctx context.Context, gid uint64, fn callFunc) error {
	op := &operation{
		ot:    call,
		gid:   gid,
		value: fn,
	}

	if err := m.push(context.Background(), op); err != nil {
		return err
	}

	if err, ok := op.value.(error); ok {
		return err
	}

	return nil
}

func (m *mux) push(ctx context.Context, op *operation) error {
	op.done = make(chan struct{})

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.done:
		return ErrStopped
	case m.operationc <- op:
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.done:
		return ErrStopped
	case <-op.done:
		return nil
	}
}

func (m *mux) tick(gid uint64) {
	_ = m.call(context.Background(), gid, func(rn *raft.RawNode) error {
		rn.Tick()
		return nil
	})
}

func (m *mux) campaign(ctx context.Context, gid uint64) error {
	return m.call(context.Background(), gid, func(rn *raft.RawNode) error {
		return rn.Campaign()
	})
}

func (m *mux) propose(ctx context.Context, gid uint64, data []byte) error {
	return m.call(context.Background(), gid, func(rn *raft.RawNode) error {
		return rn.Propose(data)
	})
}

func (m *mux) proposeConfChange(ctx context.Context, gid uint64, cc etcdraftpb.ConfChangeI) error {
	return m.call(context.Background(), gid, func(rn *raft.RawNode) error {
		return rn.ProposeConfChange(cc)
	})
}

func (m *mux) step(ctx context.Context, gid uint64, msg etcdraftpb.Message) error {
	return m.call(context.Background(), gid, func(rn *raft.RawNode) error {
		return rn.Step(msg)
	})
}

func (m *mux) advance(gid uint64) {
	op := &operation{
		ot:  advance,
		gid: gid,
	}

	_ = m.push(context.Background(), op)
}

func (m *mux) applyConfChange(gid uint64, cc etcdraftpb.ConfChangeI) *etcdraftpb.ConfState {
	cs := new(etcdraftpb.ConfState)
	_ = m.call(context.Background(), gid, func(rn *raft.RawNode) error {
		cs = rn.ApplyConfChange(cc)
		return nil
	})

	return cs
}

func (m *mux) transferLeadership(ctx context.Context, gid, _, transferee uint64) {
	_ = m.call(ctx, gid, func(rn *raft.RawNode) error {
		rn.TransferLeader(transferee)
		return nil
	})

}

func (m *mux) readIndex(ctx context.Context, gid uint64, rctx []byte) error {
	return m.call(ctx, gid, func(rn *raft.RawNode) error {
		rn.ReadIndex(rctx)
		return nil
	})
}

func (m *mux) status(gid uint64) (st raft.Status) {
	_ = m.call(context.Background(), gid, func(rn *raft.RawNode) error {
		st = rn.Status()
		return nil
	})
	return
}

func (m *mux) reportUnreachable(gid, id uint64) {
	_ = m.call(context.Background(), gid, func(rn *raft.RawNode) error {
		rn.ReportUnreachable(id)
		return nil
	})
}

func (m *mux) reportSnapshot(gid, id uint64, status raft.SnapshotStatus) {
	_ = m.call(context.Background(), gid, func(rn *raft.RawNode) error {
		rn.ReportSnapshot(id, status)
		return nil
	})
}

type muxNode struct {
	readyc <-chan raft.Ready
	gid    uint64
	mux    *mux
}

func (m *muxNode) Tick() {
	m.mux.tick(m.gid)
}

func (m *muxNode) Campaign(ctx context.Context) error {
	return m.mux.campaign(ctx, m.gid)
}

func (m *muxNode) Propose(ctx context.Context, data []byte) error {
	return m.mux.propose(ctx, m.gid, data)
}

func (m *muxNode) ProposeConfChange(ctx context.Context, cc etcdraftpb.ConfChangeI) error {
	return m.mux.proposeConfChange(ctx, m.gid, cc)
}

func (m *muxNode) Step(ctx context.Context, msg etcdraftpb.Message) error {
	return m.mux.step(ctx, m.gid, msg)
}

func (m *muxNode) Ready() <-chan raft.Ready {
	return m.readyc
}

func (m *muxNode) Advance() {
	m.mux.advance(m.gid)
}

func (m *muxNode) ApplyConfChange(cc etcdraftpb.ConfChangeI) *etcdraftpb.ConfState {
	return m.mux.applyConfChange(m.gid, cc)
}

func (m *muxNode) TransferLeadership(ctx context.Context, lead, transferee uint64) {
	m.mux.transferLeadership(ctx, m.gid, lead, transferee)
}

func (m *muxNode) ReadIndex(ctx context.Context, rctx []byte) error {
	return m.mux.readIndex(ctx, m.gid, rctx)
}

func (m *muxNode) Status() raft.Status {
	return m.mux.status(m.gid)
}

func (m *muxNode) ReportUnreachable(id uint64) {
	m.mux.reportUnreachable(m.gid, id)
}

func (m *muxNode) ReportSnapshot(id uint64, status raft.SnapshotStatus) {
	m.mux.reportSnapshot(m.gid, id, status)
}

func (m *muxNode) Stop() {
	m.mux.remove(m.gid)
}
