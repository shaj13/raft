package raft

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/shaj13/raftkit/api"
	"go.etcd.io/etcd/pkg/v3/idutil"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc"
)

var (
	errReadyProcessor = errors.New("raft: processor not ready yet or has been stopped")
)

const (
	// unreachable signals remote memeber unreachable
	unreachable signal = iota
	// shutdown signals current member removed and should stop.
	shutdown
	// snapshotSent signals sending status
	snapshotStatus
)

type signal int

type report struct {
	signal signal
	id     uint64
	status raft.SnapshotStatus
}

type processor struct {
	ctx          context.Context
	cancel       context.CancelFunc
	cfg          *config
	node         raft.Node
	ticker       *time.Ticker
	wg           sync.WaitGroup
	propwg       sync.WaitGroup
	storg        *raft.MemoryStorage
	sshot        *disk
	msgbus       *msgbus
	repoc        chan report
	propc        chan raftpb.Message
	recvc        chan raftpb.Message
	idgen        *idutil.Generator
	pool         *pool
	cState       raftpb.ConfState
	started      *atomicBool
	snapIndex    uint64
	appliedIndex uint64
}

func (p *processor) join(ctx context.Context, m *api.Member, cluster string) ([]api.Member, error) {
	conn, err := grpc.Dial(cluster, p.cfg.memberDialOptions...)
	if err != nil {
		return nil, err
	}

	c := api.NewRaftClient(conn)
	resp, err := c.Join(ctx, m)

	if err != nil {
		return nil, err
	}

	m.ID = resp.ID
	m.Type = api.SelfMember

	return resp.Pool, nil
}

func (p *processor) jj(ctx context.Context, cluster, addr string) (uint64, error) {
	var (
		err  error
		pool []api.Member
	)

	exist := p.sshot.exist()
	m := &api.Member{
		ID:      uint64(rand.Int63()) + 1,
		Address: addr,
		Type:    api.SelfMember,
	}

	if !exist && len(cluster) > 0 {
		m.ID = 0
		pool, err = p.join(ctx, m, cluster)
	}

	if err != nil {
		return 0, err
	}

	buf, err := m.Marshal()
	if err != nil {
		return 0, err
	}

	meta, hs, ents, snap, err := p.sshot.bootstrap(buf)
	if err != nil {
		return 0, err
	}

	if !exist {
		p.pool.add(*m)
		p.pool.recover(pool)
		return m.ID, err
	}

	if err := m.Unmarshal(meta); err != nil {
		return 0, err
	}

	if exist && len(cluster) > 0 && m.Address != addr {
		pool, err = p.join(ctx, m, cluster)
	}

	if pool != nil {
		s := new(api.Snapshot)
		if err := s.Unmarshal(snap.Data); err != nil {
			return 0, err
		}
		s.Pool = pool

		data, err := s.Marshal()
		if err != nil {
			return 0, err
		}

		snap.Data = data
	}

	p.publishSnapshot(*snap)
	p.storg.Append(ents)
	p.storg.SetHardState(hs)
	p.pool.add(*m)
	return m.ID, err
}

func (p *processor) run(ctx context.Context, cluster, addr string) error {
	exist := p.sshot.exist()
	id, err := p.jj(ctx, cluster, addr)
	if err != nil {
		return err
	}
	p.idgen = idutil.NewGenerator(uint16(id), time.Now())
	c := &raft.Config{
		ID:                        id,
		ElectionTick:              10,
		HeartbeatTick:             1,
		Storage:                   p.storg,
		MaxSizePerMsg:             1024 * 1024,
		MaxInflightMsgs:           256,
		MaxUncommittedEntriesSize: 1 << 30,
	}
	if exist {
		p.node = raft.RestartNode(c)
	} else {
		peer := raft.Peer{ID: id}
		p.node = raft.StartNode(c, []raft.Peer{peer})
	}
	p.started.set()
	go p.report()
	go p.process(p.propc)
	go p.process(p.recvc)
	return p.eventLoop()
}

func (p *processor) eventLoop() error {
	p.wg.Add(1)
	defer p.wg.Done()

	for {
		if p.ctx.Err() != nil {
			return p.ctx.Err()
		}

		select {
		case <-p.ticker.C:
			p.node.Tick()
		case rd := <-p.node.Ready():
			if err := p.sshot.SaveEntries(rd.HardState, rd.Entries); err != nil {
				return err
			}

			if err := p.publishSnapshot(rd.Snapshot); err != nil {
				return err
			}

			if err := p.storg.Append(rd.Entries); err != nil {
				return err
			}

			p.send(rd.Messages)
			p.publishCommitted(rd.CommittedEntries)

			if err := p.triggerSnapshot(); err != nil {
				return err
			}

			p.node.Advance()

		case <-p.ctx.Done():
			return p.ctx.Err()
		}
	}
}

func (p *processor) publishSnapshot(snap raftpb.Snapshot) error {
	if raft.IsEmptySnap(snap) {
		return nil
	}

	if snap.Metadata.Index <= p.appliedIndex {
		return fmt.Errorf(
			"raft: Snapshot index [%d] should > progress.appliedIndex [%d]",
			snap.Metadata.Index,
			p.appliedIndex,
		)
	}

	p.cfg.logger.Debugf("raft: Publishing snapshot at index %d", p.snapIndex)
	if err := p.sshot.SaveSnapshot(snap); err != nil {
		return err
	}

	if err := p.storg.ApplySnapshot(snap); err != nil {
		return err
	}

	s := new(api.Snapshot)
	if err := s.Unmarshal(snap.Data); err != nil {
		return err
	}

	p.pool.recover(s.Pool)

	// TODO: trigger original user to load snapshot

	p.cState = snap.Metadata.ConfState
	p.snapIndex = snap.Metadata.Index
	p.appliedIndex = snap.Metadata.Index

	return nil
}

func (p *processor) triggerSnapshot() error {
	if p.appliedIndex-p.snapIndex <= p.cfg.snapInterval {
		return nil
	}

	p.cfg.logger.Infof("raft: Start snapshot [applied index: %d | last snapshot index: %d]", p.appliedIndex, p.snapIndex)

	// TODO: get  snapshot from user
	// data, err := rc.getSnapshot()
	// if err != nil {
	// 	log.Panic(err)
	// }

	snap, err := p.storg.CreateSnapshot(p.appliedIndex, &p.cState, nil) // TODO: pass data
	if err != nil {
		return err
	}

	if err := p.sshot.SaveSnapshot(snap); err != nil {
		return err
	}

	compactIndex := uint64(1)
	if p.appliedIndex > p.cfg.snapInterval {
		compactIndex = p.appliedIndex - p.cfg.snapInterval
	}

	if err := p.storg.Compact(compactIndex); err != nil {
		return err
	}

	p.cfg.logger.Infof("raft: Compacted log at index %d", compactIndex)

	p.snapIndex = p.appliedIndex
	return nil
}

func (p *processor) publishCommitted(ents []raftpb.Entry) {
	for _, ent := range ents {
		if ent.Type == raftpb.EntryNormal && ent.Data != nil {
			p.publishReplicate(ent)
		}
		if ent.Type == raftpb.EntryConfChange {
			p.publishConfChange(ent)
		}
		p.appliedIndex = ent.Index
	}
}

func (p *processor) publishReplicate(ent raftpb.Entry) {
	var err error
	r := new(api.Replicate)
	defer func() {
		p.msgbus.trigger(r.CID, err)
		if err != nil {
			p.cfg.logger.Warningf("raft: An error occured while publish replicate data, Err: %s", err)
		}
	}()
	if err = r.Unmarshal(ent.Data); err != nil {
		return
	}

	// TODO: publish it to end user
}

func (p *processor) publishConfChange(ent raftpb.Entry) {
	var err error
	cc := new(raftpb.ConfChange)
	mem := new(api.Member)

	defer func() {
		p.msgbus.trigger(cc.ID, err)
		if err != nil {
			p.cfg.logger.Warningf("raft: An error occured while publish conf change, Err: %s", err)
		}
	}()

	if err = cc.Unmarshal(ent.Data); err != nil {
		return
	}

	if len(cc.Context) == 0 {
		// TODO: add debug msg
		return
	}

	if err = mem.Unmarshal(cc.Context); err != nil {
		return
	}

	// TODO: need to check that removed added etc is not the current node
	switch cc.Type {
	case raftpb.ConfChangeAddNode:
		err = p.pool.add(*mem)
	case raftpb.ConfChangeUpdateNode:
		err = p.pool.update(*mem)
	case raftpb.ConfChangeRemoveNode:
		err = p.pool.remove(*mem)
	}

	p.cState = *p.node.ApplyConfChange(cc)
}

// process the incoming messages from the givven chan.
func (p *processor) process(ch chan raftpb.Message) {
	p.wg.Add(1)
	defer p.wg.Done()

	for {
		select {
		case m := <-ch:
			if err := p.node.Step(p.ctx, m); err != nil {
				p.cfg.logger.Warningf("raft: Failed to process raft message, Err: %s", err)
			}
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *processor) report() {
	p.wg.Add(1)
	defer p.wg.Done()

	for {
		select {
		case r := <-p.repoc:
			switch r.signal {
			case shutdown:
				//TODO:  we should close gracfully right now kkep it and need a listener that close all raftkit structs
			case unreachable:
				p.node.ReportUnreachable(r.id)
			case snapshotStatus:
				p.node.ReportSnapshot(r.id, r.status)
			}
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *processor) send(msgs []raftpb.Message) {
	log := func(m raftpb.Message, str string) {
		p.cfg.logger.Warningf("raft: Failed to send message %s to member %x, Err: %s", m.Type, m.To, str)
	}

	p.cfg.logger.Debug("raft: Sending messages to raft cluster members")

	for _, m := range msgs {
		mem, ok := p.pool.get(m.To)
		if !ok {
			log(m, "unknown member")
			continue
		}

		if err := mem.send(m); err != nil {
			log(m, err.Error())
		}
	}
}

// push msg to proccessor queue.
// its used by the current node server.
func (p *processor) push(m raftpb.Message) error {
	if !p.started.isSet() {
		return errReadyProcessor
	}
	ch := p.recvc
	if m.Type == raftpb.MsgProp {
		ch = p.propc
	}
	ch <- m
	return nil
}

func (p *processor) proposeConfChange(ctx context.Context, m *api.Member, t raftpb.ConfChangeType) error {
	if !p.started.isSet() {
		return errReadyProcessor
	}
	p.propwg.Add(1)
	defer p.propwg.Done()

	buf, err := m.Marshal()
	if err != nil {
		return err
	}

	cc := raftpb.ConfChange{
		ID:      p.idgen.Next(),
		Type:    t,
		NodeID:  m.ID,
		Context: buf,
	}

	if err := p.node.ProposeConfChange(ctx, cc); err != nil {
		return err
	}

	// wait for changes to be done
	ch := p.msgbus.subscribe(cc.ID)
	defer p.msgbus.cancel(cc.ID)

	select {
	case v := <-ch:
		if v != nil {
			return err.(error)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *processor) proposeReplicate(ctx context.Context, data []byte) error {
	if !p.started.isSet() {
		return errReadyProcessor
	}

	p.propwg.Add(1)
	defer p.propwg.Done()

	r := &api.Replicate{
		CID:  p.idgen.Next(),
		Data: data,
	}

	buf, err := r.Marshal()
	if err != nil {
		return err
	}

	if err := p.node.Propose(ctx, buf); err != nil {
		return err
	}

	// wait for changes to be done
	ch := p.msgbus.subscribe(r.CID)
	defer p.msgbus.cancel(r.CID)

	select {
	case err := <-ch:
		return err.(error)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *processor) status() (raft.Status, error) {
	if !p.started.isSet() {
		return raft.Status{}, errReadyProcessor
	}

	return p.node.Status(), nil
}

func (p *processor) close() {
	p.cancel()
	p.started.unSet()
	p.wg.Done()
	p.propwg.Done()
	p.node.Stop()
}
