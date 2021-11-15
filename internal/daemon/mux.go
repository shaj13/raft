package daemon

import (
	"context"

	"github.com/shaj13/raftkit/internal/log"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

func init() {
	startNode = func(c *raft.Config, peers []raft.Peer) raft.Node {
		if len(peers) == 0 {
			panic("no peers given; use RestartNode instead")
		}

		rn, err := raft.NewRawNode(c)
		if err != nil {
			panic(err)
		}

		rn.Bootstrap(peers)

		mux := new(mux)
		mux.done = make(chan struct{})
		mux.stop = make(chan struct{})
		mux.messagec = make(chan *muxMessage)
		go mux.Start()

		return mux.add(1, rn)
	}
}

const (
	addNode muxMessageType = iota
	removeNode
	invokeNode
	advanceNode
	// TODO(Sanad):  need to fan in/out heratbeat.
)

type muxMessageType uint
type invokeFunc func(rn *raft.RawNode) error

type muxMessage struct {
	mtype muxMessageType
	// gid specifies group id.
	gid uint64
	// value specifies the request/response content.
	value interface{}
	// done notify the caller that mux has proccess the msg.
	done chan struct{}
}

type mux struct {
	messagec chan *muxMessage
	stop     chan struct{}
	done     chan struct{}
}

func (m *mux) Stop() {
	// send stop signal.
	close(m.stop)
	// wait for mux to be terminated.
	<-m.done
}

func (m *mux) Start() {

	defer close(m.done)
	nodes := map[uint64]*raft.RawNode{}
	readycs := map[uint64]chan raft.Ready{}
	advcs := map[uint64]raft.Ready{}
	leads := map[uint64]uint64{}

	for {
		if len(advcs) == 0 {
			for id, n := range nodes {
				lead := leads[id]
				st := n.BasicStatus()
				if lead != st.Lead {
					if st.Lead != 0 {
						if lead == 0 {
							log.Infof("raft.node: %x elected leader %x at term %d", st.ID, st.Lead, st.Term)
						} else {
							log.Infof("raft.node: %x changed leader from %x to %x at term %d", st.ID, lead, st.Lead, st.Term)
						}
					} else {
						log.Infof("raft.node: %x lost leader %x at term %d", st.ID, st.Lead, st.Term)
					}
					leads[id] = st.Lead
				}

				if n.HasReady() {
					rd := n.Ready()
					c := readycs[id]
					advcs[id] = rd
					c <- rd
				}
			}
		}
		select {
		case msg := <-m.messagec:
			switch msg.mtype {
			case addNode:
				readyc := make(chan raft.Ready)
				nodes[msg.gid] = msg.value.(*raft.RawNode)
				readycs[msg.gid] = readyc
				msg.value = readyc
			case removeNode:
				delete(nodes, msg.gid)
				delete(advcs, msg.gid)
			case invokeNode:
				node := nodes[msg.gid]
				err := msg.value.(invokeFunc)(node)
				msg.value = err
			case advanceNode:
				if node, ok := nodes[msg.gid]; ok {
					rd := advcs[msg.gid]
					node.Advance(rd)
					delete(advcs, msg.gid)
				}
			}

			close(msg.done)

		case <-m.stop:
			return
		}
	}
}

func (m *mux) add(gid uint64, rn *raft.RawNode) raft.Node {
	msg := &muxMessage{
		mtype: addNode,
		gid:   gid,
		value: rn,
	}

	_ = m.push(context.Background(), msg)

	return &muxNode{
		gid:    gid,
		mux:    m,
		readyc: msg.value.(chan raft.Ready),
	}
}

func (m *mux) remove(gid uint64) {
	msg := &muxMessage{
		mtype: removeNode,
		gid:   gid,
	}

	_ = m.push(context.Background(), msg)
}

func (m *mux) invoke(ctx context.Context, gid uint64, fn invokeFunc) error {
	msg := &muxMessage{
		mtype: invokeNode,
		gid:   gid,
		value: fn,
	}

	if err := m.push(context.Background(), msg); err != nil {
		return err
	}

	if err, ok := msg.value.(error); ok {
		return err
	}

	return nil
}

func (m *mux) push(ctx context.Context, msg *muxMessage) error {
	msg.done = make(chan struct{})

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.done:
		return ErrStopped
	case m.messagec <- msg:
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.done:
		return ErrStopped
	case <-msg.done:
		return nil
	}
}

func (m *mux) tick(gid uint64) {
	_ = m.invoke(context.Background(), gid, func(rn *raft.RawNode) error {
		rn.Tick()
		return nil
	})
}

func (m *mux) campaign(ctx context.Context, gid uint64) error {
	return m.invoke(context.Background(), gid, func(rn *raft.RawNode) error {
		return rn.Campaign()
	})
}

func (m *mux) propose(ctx context.Context, gid uint64, data []byte) error {
	return m.invoke(context.Background(), gid, func(rn *raft.RawNode) error {
		return rn.Propose(data)
	})
}

func (m *mux) proposeConfChange(ctx context.Context, gid uint64, cc etcdraftpb.ConfChangeI) error {
	return m.invoke(context.Background(), gid, func(rn *raft.RawNode) error {
		return rn.ProposeConfChange(cc)
	})
}

func (m *mux) step(ctx context.Context, gid uint64, msg etcdraftpb.Message) error {
	return m.invoke(context.Background(), gid, func(rn *raft.RawNode) error {
		return rn.Step(msg)
	})
}

func (m *mux) advance(gid uint64) {
	msg := &muxMessage{
		mtype: advanceNode,
		gid:   gid,
	}

	_ = m.push(context.Background(), msg)
}

func (m *mux) applyConfChange(gid uint64, cc etcdraftpb.ConfChangeI) *etcdraftpb.ConfState {
	cs := new(etcdraftpb.ConfState)
	_ = m.invoke(context.Background(), gid, func(rn *raft.RawNode) error {
		cs = rn.ApplyConfChange(cc)
		return nil
	})

	return cs
}

func (m *mux) transferLeadership(ctx context.Context, gid, _, transferee uint64) {
	_ = m.invoke(ctx, gid, func(rn *raft.RawNode) error {
		rn.TransferLeader(transferee)
		return nil
	})

}

func (m *mux) readIndex(ctx context.Context, gid uint64, rctx []byte) error {
	return m.invoke(ctx, gid, func(rn *raft.RawNode) error {
		rn.ReadIndex(rctx)
		return nil
	})
}

func (m *mux) status(gid uint64) (st raft.Status) {
	_ = m.invoke(context.Background(), gid, func(rn *raft.RawNode) error {
		st = rn.Status()
		return nil
	})
	return
}

func (m *mux) reportUnreachable(gid, id uint64) {
	_ = m.invoke(context.Background(), gid, func(rn *raft.RawNode) error {
		rn.ReportUnreachable(id)
		return nil
	})
	return
}

func (m *mux) reportSnapshot(gid, id uint64, status raft.SnapshotStatus) {
	_ = m.invoke(context.Background(), gid, func(rn *raft.RawNode) error {
		rn.ReportSnapshot(id, status)
		return nil
	})
	return
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
