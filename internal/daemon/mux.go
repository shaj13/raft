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
		mux.messagec = make(chan *message)
		go mux.Start()

		return mux.add(1, rn)
	}
}

const (
	add operation = iota
	remove
	call
	advance
	// TODO(Sanad):  need to fan in/out heratbeat.
)

type operation uint
type callFunc func(rn *raft.RawNode) error

type message struct {
	op operation
	// gid specifies group id.
	gid uint64
	// value specifies the request/response content.
	value interface{}
	// done notify the caller that mux has proccess the msg.
	done chan struct{}
}

type mux struct {
	messagec chan *message
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
			for gid, n := range nodes {
				lead := leads[gid]
				st := n.BasicStatus()
				if lead != st.Lead {
					if st.Lead != 0 {
						if lead == 0 {
							log.Infof("raft.node: %x elected leader %x for group %x at term %d", st.ID, st.Lead, gid, st.Term)
						} else {
							log.Infof("raft.node: %x changed group %x leader from %x to %x at term %d", st.ID, gid, lead, st.Lead, st.Term)
						}
					} else {
						log.Infof("raft.node: %x lost group %x leader(%x) at term %d", st.ID, gid, st.Lead, st.Term)
					}
					leads[gid] = st.Lead
				}

				if n.HasReady() {
					rd := n.Ready()
					c := readycs[gid]
					advcs[gid] = rd
					c <- rd
				}
			}
		}
		select {
		case msg := <-m.messagec:
			switch msg.op {
			case add:
				readyc := make(chan raft.Ready)
				nodes[msg.gid] = msg.value.(*raft.RawNode)
				readycs[msg.gid] = readyc
				msg.value = readyc
			case remove:
				delete(nodes, msg.gid)
				delete(advcs, msg.gid)
			case call:
				node := nodes[msg.gid]
				err := msg.value.(callFunc)(node)
				msg.value = err
			case advance:
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
	msg := &message{
		op:    add,
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
	msg := &message{
		op:  remove,
		gid: gid,
	}

	_ = m.push(context.Background(), msg)
}

func (m *mux) call(ctx context.Context, gid uint64, fn callFunc) error {
	msg := &message{
		op:    call,
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

func (m *mux) push(ctx context.Context, msg *message) error {
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
	msg := &message{
		op:  advance,
		gid: gid,
	}

	_ = m.push(context.Background(), msg)
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

//  get node ids
// for _, v := range []map[uint64]struct{}{
// 	s.Config.Voters.IDs(),
// 	s.Config.Learners,
// } {
// 	for id := range v {
// 		fmt.Println(id)
// 	}
// }
