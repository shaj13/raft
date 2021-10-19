package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/shaj13/raftkit/internal/atomic"
	"github.com/shaj13/raftkit/internal/log"
	"github.com/shaj13/raftkit/internal/membership"
	"github.com/shaj13/raftkit/internal/msgbus"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/storage"
	"go.etcd.io/etcd/pkg/v3/idutil"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

var (
	ErrStopped = errors.New("raft: daemon not ready yet or has been stopped")
)

const (
	msg Event = iota + 1
	propose
	snapshot
)

// TODO: add event comment
type Event uint64

// ID returns event id.
func (e Event) ID() uint64 {
	return uint64(e)
}

type Daemon interface {
	MsgBus() *msgbus.MsgBus
	Push(m etcdraftpb.Message) error
	Status() (raft.Status, error)
	Close() error
	ProposeReplicate(ctx context.Context, data []byte) error
	ProposeConfChange(ctx context.Context, m *raftpb.Member, t etcdraftpb.ConfChangeType) error
	CreateSnapshot() (etcdraftpb.Snapshot, error)
	Start(addr string, oprs ...Operator) error
	ReportUnreachable(id uint64)
	ReportSnapshot(id uint64, status raft.SnapshotStatus)
	ReportShutdown(id uint64)
}

func New(ctx context.Context, cfg Config) Daemon {
	d := &daemon{}
	d.ctx, d.cancel = context.WithCancel(ctx)
	d.cfg = cfg
	d.ticker = time.NewTicker(cfg.TickInterval())
	d.wg = sync.WaitGroup{}
	d.propwg = sync.WaitGroup{}
	d.cache = raft.NewMemoryStorage()
	d.storage = cfg.Storage()
	d.msgbus = msgbus.New()
	d.pool = cfg.Pool()
	d.started = atomic.NewBool()
	d.appliedIndex = atomic.NewUint64()
	d.snapIndex = atomic.NewUint64()
	return d
}

type daemon struct {
	ctx          context.Context
	cancel       context.CancelFunc
	ost          *operatorsState
	cfg          Config
	node         raft.Node
	ticker       *time.Ticker
	wg           sync.WaitGroup
	propwg       sync.WaitGroup
	cache        *raft.MemoryStorage
	storage      storage.Storage
	msgbus       *msgbus.MsgBus
	idgen        *idutil.Generator
	pool         membership.Pool
	cState       etcdraftpb.ConfState
	started      *atomic.Bool
	snapIndex    *atomic.Uint64
	appliedIndex *atomic.Uint64
}

// MsgBus returns daemon msgbus.
func (d *daemon) MsgBus() *msgbus.MsgBus {
	return d.msgbus
}

// ReportUnreachable reports the given node is not reachable for the last send.
func (d *daemon) ReportUnreachable(id uint64) {
	if d.started.False() {
		return
	}

	d.node.ReportUnreachable(id)
}

func (d *daemon) ReportSnapshot(id uint64, status raft.SnapshotStatus) {
	if d.started.False() {
		return
	}

	d.node.ReportSnapshot(id, status)
}

func (d *daemon) ReportShutdown(id uint64) {
	if d.started.False() {
		return
	}

	// TODO: push to msgbus when events are defined.
	// d.msgbus.Broadcast()
}

// Push m to the daemon queue.
func (d *daemon) Push(m etcdraftpb.Message) error {
	if d.started.False() {
		return ErrStopped
	}

	// event id based on msg type.
	event := msg
	if m.Type == etcdraftpb.MsgProp {
		event = propose
	}

	d.msgbus.Broadcast(event.ID(), m)
	return nil
}

// Status returns the current status of the raft state machine.
func (d *daemon) Status() (raft.Status, error) {
	if d.started.False() {
		return raft.Status{}, ErrStopped
	}

	return d.node.Status(), nil
}

// Close the daemon.
func (d *daemon) Close() error {
	d.cancel()
	d.started.UnSet()
	d.wg.Done()
	d.propwg.Done()
	d.node.Stop()
	d.MsgBus().Clsoe()
	return nil
}

// ProposeReplicate proposes to replicate the data to be appended to the raft log.
func (d *daemon) ProposeReplicate(ctx context.Context, data []byte) error {
	if d.started.False() {
		return ErrStopped
	}

	d.propwg.Add(1)
	defer d.propwg.Done()

	r := &raftpb.Replicate{
		CID:  d.idgen.Next(),
		Data: data,
	}

	buf, err := r.Marshal()
	if err != nil {
		return err
	}

	if err := d.node.Propose(ctx, buf); err != nil {
		return err
	}

	// wait for changes to be done
	sub := d.msgbus.SubscribeOnce(r.CID)
	defer sub.Unsubscribe()

	select {
	case v := <-sub.Chan():
		if v != nil {
			return err.(error)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ProposeConfChange proposes a configuration change to the cluster pool members.
func (d *daemon) ProposeConfChange(ctx context.Context, m *raftpb.Member, t etcdraftpb.ConfChangeType) error {
	if d.started.False() {
		return ErrStopped
	}
	d.propwg.Add(1)
	defer d.propwg.Done()

	buf, err := m.Marshal()
	if err != nil {
		return err
	}

	cc := etcdraftpb.ConfChange{
		ID:      d.idgen.Next(),
		Type:    t,
		NodeID:  m.ID,
		Context: buf,
	}

	if err := d.node.ProposeConfChange(ctx, cc); err != nil {
		return err
	}

	// wait for changes to be done
	sub := d.msgbus.SubscribeOnce(cc.ID)
	defer sub.Unsubscribe()

	select {
	case v := <-sub.Chan():
		if v != nil {
			return v.(error)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// CreateSnapshot begin a snapshot and return snap metadata.
func (d *daemon) CreateSnapshot() (etcdraftpb.Snapshot, error) {
	appliedIndex := d.appliedIndex.Get()
	snapIndex := d.snapIndex.Get()

	if appliedIndex == snapIndex {
		// up to date just return the latest snap to load it from disk.
		return d.cache.CreateSnapshot(appliedIndex, &d.cState, nil)
	}

	log.Infof(
		"raft.daemon: start snapshot [applied index: %d | last snapshot index: %d]",
		appliedIndex,
		snapIndex,
	)

	// TODO: get  snapshot from user
	// data, err := rc.getSnapshot()
	// if err != nil {
	// 	log.Panic(err)
	// }

	// TODO: organize me
	snap, err := d.cache.CreateSnapshot(appliedIndex, &d.cState, nil) // TODO: pass data
	if err != nil {
		return snap, err
	}

	sf := storage.SnapshotFile{
		Snap: &snap,
		Pool: &raftpb.Pool{
			Members: d.pool.Snapshot(),
		},
		Data: ioutil.NopCloser(bytes.NewBufferString("sample dta")),
	}

	if err := d.storage.Snapshotter().Write(&sf); err != nil {
		return snap, err
	}

	if err := d.storage.SaveSnapshot(snap); err != nil {
		return snap, err
	}

	compactIndex := uint64(1)
	if appliedIndex > d.cfg.SnapInterval() {
		compactIndex = appliedIndex - d.cfg.SnapInterval()
	}

	if err := d.cache.Compact(compactIndex); err != nil {
		return snap, err
	}

	log.Infof("raft.daemon: compacted log at index %d", compactIndex)

	d.snapIndex.Set(appliedIndex)
	return snap, err
}

// TODO: more comment
// Start daemon.
func (d *daemon) Start(addr string, oprs ...Operator) error {
	oprs = append(oprs, setup{addr: addr}, stateSetup{publishSnapshotFile: d.publishSnapshotFile})
	if err := invoke(d, oprs...); err != nil {
		return err
	}

	// subscribe to propose message.
	prop := d.msgbus.SubscribeBuffered(propose.ID(), 4096)
	// subscribe to recived received.
	recv := d.msgbus.SubscribeBuffered(msg.ID(), 4096)

	d.idgen = idutil.NewGenerator(uint16(d.ost.local.ID), time.Now())
	d.started.Set()
	go d.process(prop)
	go d.process(recv)
	go d.snapshots()
	return d.eventLoop()
}

func (d *daemon) eventLoop() error {
	d.wg.Add(1)
	defer d.wg.Done()

	for {
		if d.ctx.Err() != nil {
			return d.ctx.Err()
		}

		select {
		case <-d.ticker.C:
			d.node.Tick()
		case rd := <-d.node.Ready():
			if err := d.storage.SaveEntries(rd.HardState, rd.Entries); err != nil {
				return err
			}

			if err := d.publishSnapshot(rd.Snapshot); err != nil {
				return err
			}

			if err := d.cache.Append(rd.Entries); err != nil {
				return err
			}

			d.send(rd.Messages)
			d.publishCommitted(rd.CommittedEntries)

			d.msgbus.Broadcast(snapshot.ID(), nil) // TODO: add snapid event

			d.node.Advance()
		case <-d.ctx.Done():
			return d.ctx.Err()
		}
	}
}

func (d *daemon) publishSnapshot(snap etcdraftpb.Snapshot) error {
	if raft.IsEmptySnap(snap) {
		return nil
	}

	if snap.Metadata.Index <= d.appliedIndex.Get() {
		return fmt.Errorf(
			"raft: Snapshot index [%d] should > progress.appliedIndex [%s]",
			snap.Metadata.Index,
			d.appliedIndex,
		)
	}

	if err := d.storage.SaveSnapshot(snap); err != nil {
		return err
	}

	sf, err := d.storage.Snapshotter().Read(snap)
	if err != nil {
		return err
	}

	return d.publishSnapshotFile(sf)
}

func (d *daemon) publishSnapshotFile(sf *storage.SnapshotFile) error {
	snap := *sf.Snap

	if err := d.cache.ApplySnapshot(*sf.Snap); err != nil {
		return err
	}

	d.pool.Restore(*sf.Pool)

	// TODO: trigger original user to load snapshot

	d.cState = snap.Metadata.ConfState
	d.snapIndex.Set(snap.Metadata.Index)
	d.appliedIndex.Set(snap.Metadata.Index)
	return nil
}

func (d *daemon) publishCommitted(ents []etcdraftpb.Entry) {
	for _, ent := range ents {
		if ent.Type == etcdraftpb.EntryNormal && ent.Data != nil {
			d.publishReplicate(ent)
		}
		if ent.Type == etcdraftpb.EntryConfChange {
			d.publishConfChange(ent)
		}
		d.appliedIndex.Set(ent.Index)
	}
}

func (d *daemon) publishReplicate(ent etcdraftpb.Entry) {
	var err error
	r := new(raftpb.Replicate)
	defer func() {
		d.msgbus.Broadcast(r.CID, err)
		if err != nil {
			log.Warnf(
				"raft.daemon: publishing replicate data failed, Err: %s",
				err,
			)
		}
	}()

	if err = r.Unmarshal(ent.Data); err != nil {
		return
	}

	// TODO: publish it to end user
}

func (d *daemon) publishConfChange(ent etcdraftpb.Entry) {
	var err error
	cc := new(etcdraftpb.ConfChange)
	mem := new(raftpb.Member)

	defer func() {
		d.msgbus.Broadcast(cc.ID, err)
		if err != nil {
			log.Warnf(
				"raft.daemon: publishing conf change failed, Err: %s",
				err,
			)
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

	// got remote member as a local, cast it back to remote.
	// this can happen when leader replicate conf change from his perspective to follower.
	if mem.ID != d.ost.local.ID && mem.Type == raftpb.LocalMember { // TODO: dont use ost.
		mem.Type = raftpb.RemoteMember
	}

	// TODO: need to check that removed added etc is not the current node
	switch cc.Type {
	case etcdraftpb.ConfChangeAddNode:
		err = d.pool.Add(*mem)
	case etcdraftpb.ConfChangeUpdateNode:
		err = d.pool.Update(*mem)
	case etcdraftpb.ConfChangeRemoveNode:
		err = d.pool.Remove(*mem)
	}

	d.cState = *d.node.ApplyConfChange(cc)
}

// process the incoming messages from the given chan.
func (d *daemon) process(sub *msgbus.Subscription) {
	d.wg.Add(1)
	defer d.wg.Done()
	defer sub.Unsubscribe()

	for {
		select {
		case v := <-sub.Chan():
			if err := d.node.Step(d.ctx, v.(etcdraftpb.Message)); err != nil {
				log.Warnf(
					"raft.daemon: process raft message failed, Err: %s",
					err,
				)
			}
		case <-d.ctx.Done():
			return
		}
	}
}

func (d *daemon) send(msgs []etcdraftpb.Message) {
	lg := func(m etcdraftpb.Message, str string) {
		log.Warnf(
			"raft.daemon: sending message %s to member %x failed, Err: %s",
			m.Type,
			m.To,
			str,
		)
	}

	for _, m := range msgs {
		mem, ok := d.pool.Get(m.To)
		if !ok {
			lg(m, "unknown member")
			continue
		}

		if err := mem.Send(m); err != nil {
			lg(m, err.Error())
		}
	}
}

func (d *daemon) snapshots() {
	d.wg.Add(1)
	defer d.wg.Done()

	sub := d.msgbus.SubscribeBuffered(snapshot.ID(), 10)
	defer sub.Unsubscribe()

	for {
		select {
		case <-sub.Chan():
			if d.appliedIndex.Get()-d.snapIndex.Get() <= d.cfg.SnapInterval() {
				continue
			}

			if _, err := d.CreateSnapshot(); err != nil {
				log.Errorf(
					"raft.daemon: creating new snapshot at index %s failed, Err: %s",
					d.appliedIndex,
					err,
				)
			}
		case <-d.ctx.Done():
			return
		}

	}
}
