package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"sync"
	"time"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/atomic"
	"github.com/shaj13/raftkit/internal/log"
	"github.com/shaj13/raftkit/internal/membership"
	"github.com/shaj13/raftkit/internal/storage"
	"go.etcd.io/etcd/pkg/v3/idutil"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

var (
	ErrStopped = errors.New("raft/daemon: daemon not ready yet or has been stopped")
)

const (
	// Unreachable indicate remote memeber unreachable.
	Unreachable Signal = iota
	// Shutdown indicate current member removed and should stop.
	Shutdown
	// SnapshotOK indicate remote memeber received the snapshot.
	SnapshotOK
	// SnapshotFailure indicate remote memeber failed to receive the snapshot.
	SnapshotFailure
)

// Signal is an interrupt sent to daemon to indicate
// that an important event has occurred.
type Signal int

type Config interface {
	RaftConfig() *raft.Config
	SnapInterval() uint64
}

type daemon struct {
	ctx          context.Context
	cancel       context.CancelFunc
	cfg          Config
	node         raft.Node
	ticker       *time.Ticker
	wg           sync.WaitGroup
	propwg       sync.WaitGroup
	cache        *raft.MemoryStorage
	storage      storage.Storage
	msgbus       *MsgBus
	idgen        *idutil.Generator
	pool         *membership.Pool
	cState       raftpb.ConfState
	started      *atomic.Bool
	snapIndex    *atomic.Uint64
	appliedIndex *atomic.Uint64
}

// MsgBus returns daemon msgbus.
func (d *daemon) MsgBus() *MsgBus {
	return d.msgbus
}

// Notify causes daemon to relay incoming signals to raft node.
func (d *daemon) Notify(sig Signal, id uint64) {
	if d.started.False() {
		return
	}

	switch sig {
	case Unreachable:
		d.node.ReportUnreachable(id)
	case SnapshotOK:
		d.node.ReportSnapshot(id, raft.SnapshotFinish)
	case SnapshotFailure:
		d.node.ReportSnapshot(id, raft.SnapshotFailure)
	case Shutdown:
		// TODO: push to msgbus when events are defined.
		// d.msgbus.Broadcast()
	default:
		log.Panicf("raft/daemon: Received %d unknown signal", sig)
	}
}

// Push m to the daemon queue.
func (d *daemon) Push(m raftpb.Message) error {
	if d.started.False() {
		return ErrStopped
	}

	// event id based on msg type.
	id := uint64(1) // TODO: change once event defined
	if m.Type == raftpb.MsgProp {
		id = uint64(2) // TODO: change once event defined
	}

	d.msgbus.Broadcast(id, m)
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
func (p *daemon) Close() {
	p.cancel()
	p.started.UnSet()
	p.wg.Done()
	p.propwg.Done()
	p.node.Stop()
}

// ProposeReplicate proposes to replicate the data to be appended to the raft log.
func (d *daemon) ProposeReplicate(ctx context.Context, data []byte) error {
	if d.started.False() {
		return ErrStopped
	}

	d.propwg.Add(1)
	defer d.propwg.Done()

	r := &api.Replicate{
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
func (d *daemon) ProposeConfChange(ctx context.Context, m *api.Member, t raftpb.ConfChangeType) error {
	if d.started.False() {
		return ErrStopped
	}
	d.propwg.Add(1)
	defer d.propwg.Done()

	buf, err := m.Marshal()
	if err != nil {
		return err
	}

	cc := raftpb.ConfChange{
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
			return err.(error)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// CreateSnapshot begin a snapshot and return snap metadata.
func (d *daemon) CreateSnapshot() (raftpb.Snapshot, error) {
	appliedIndex := d.appliedIndex.Get()
	snapIndex := d.snapIndex.Get()

	if appliedIndex == snapIndex {
		// up to date just return the latest snap to load it from disk.
		return d.cache.CreateSnapshot(appliedIndex, &d.cState, nil)
	}

	log.Infof(
		"raft/daemon: Start snapshot [applied index: %d | last snapshot index: %d]",
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
		Pool: &api.Pool{
			Members: d.pool.Snapshot(),
		},
		Data: ioutil.NopCloser(bytes.NewBufferString("sample dta")),
	}

	if err := d.storage.Snapshoter().Write(&sf); err != nil {
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

	log.Infof("raft/daemon: Compacted log at index %d", compactIndex)

	d.snapIndex.Set(appliedIndex)
	return snap, err
}

func (d *daemon) join(ctx context.Context, m *api.Member, cluster string) ([]api.Member, error) {
	return nil, nil
}

func (d *daemon) boot(ctx context.Context, cluster, addr string) (*api.Member, []api.Member, error) {
	var (
		err  error
		pool []api.Member
	)

	exist := d.storage.Exist()
	m := &api.Member{
		// generate a random id in case this is the first member in the cluster.
		ID:      uint64(rand.Int63()) + 1,
		Address: addr,
		Type:    api.LocalMember,
	}

	if !exist && len(cluster) > 0 {
		m.ID = 0
		pool, err = d.join(ctx, m, cluster)
	}

	if err != nil {
		return nil, nil, err
	}

	meta := pbutil.MustMarshal(m)
	meta, hs, ents, snap, err := d.storage.Boot(meta)
	if err != nil {
		return nil, nil, err
	}

	if !exist {
		return m, pool, nil
	}

	pbutil.MustUnmarshal(m, meta)

	// find the current member from wal conf-change entries,
	// metadata isn't up to date, need to get the latest previous address.
	for _, ent := range ents {
		if ent.Type != raftpb.EntryConfChange {
			continue
		}
		cc := new(raftpb.ConfChange)
		old := new(api.Member)
		pbutil.MustUnmarshal(cc, ent.Data)
		pbutil.MustUnmarshal(old, cc.Context)
		if m.ID == old.ID {
			m = old
		}
	}

	// current member address changed, re-join the cluster.
	if len(cluster) > 0 && m.Address != addr {
		pool, err = d.join(ctx, m, cluster)
	}

	if err != nil {
		return nil, nil, err
	}

	d.publishSnapshotFile(snap)
	d.cache.SetHardState(hs)
	d.cache.Append(ents)
	return m, pool, nil
}

func (d *daemon) run(ctx context.Context, cluster, addr string) error {
	exist := d.storage.Exist()
	m, pool, err := d.boot(ctx, cluster, addr)
	if err != nil {
		return err
	}

	d.idgen = idutil.NewGenerator(uint16(m.ID), time.Now())
	// TODO: load this from the config object.
	c := &raft.Config{
		ID:                        m.ID,
		ElectionTick:              10,
		HeartbeatTick:             1,
		Storage:                   d.cache,
		MaxSizePerMsg:             1024 * 1024,
		MaxInflightMsgs:           256,
		MaxUncommittedEntriesSize: 1 << 30,
	}

	if exist && len(pool) == 0 {
		d.node = raft.RestartNode(c)
	} else {
		pool = append(pool, *m)
		peers := make([]raft.Peer, len(pool))
		for i, m := range pool {
			peers[i] = raft.Peer{ID: m.ID, Context: pbutil.MustMarshal(&m)}
		}
		d.node = raft.StartNode(c, peers)
	}

	// subscribe to propose message.
	prop := d.msgbus.SubscribeBuffered(uint64(1), 4096)
	// subscribe to recived received.
	recv := d.msgbus.SubscribeBuffered(uint64(1), 4096)

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

			d.msgbus.Broadcast(uint64(1), nil) // TODO: add snapid event

			d.node.Advance()
		case <-d.ctx.Done():
			return d.ctx.Err()
		}
	}
}

func (d *daemon) publishSnapshot(snap raftpb.Snapshot) error {
	if raft.IsEmptySnap(snap) {
		log.Debug("raft/daemon: ignore empty snapshot")
		return nil
	}

	if snap.Metadata.Index <= d.appliedIndex.Get() {
		return fmt.Errorf(
			"raft: Snapshot index [%d] should > progress.appliedIndex [%s]",
			snap.Metadata.Index,
			d.appliedIndex,
		)
	}

	log.Infof("raft/daemon: Publishing snapshot at index %s", d.snapIndex)

	if err := d.storage.SaveSnapshot(snap); err != nil {
		return err
	}

	sf, err := d.storage.Snapshoter().Read(snap)
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

func (d *daemon) publishCommitted(ents []raftpb.Entry) {
	for _, ent := range ents {
		if ent.Type == raftpb.EntryNormal && ent.Data != nil {
			d.publishReplicate(ent)
		}
		if ent.Type == raftpb.EntryConfChange {
			d.publishConfChange(ent)
		}
		d.appliedIndex.Set(ent.Index)
	}
}

func (d *daemon) publishReplicate(ent raftpb.Entry) {
	var err error
	r := new(api.Replicate)
	defer func() {
		d.msgbus.Broadcast(r.CID, err)
		if err != nil {
			log.Warnf(
				"raft/daemon: An error occured while publish replicate data, Err: %s",
				err,
			)
		}
	}()
	if err = r.Unmarshal(ent.Data); err != nil {
		return
	}

	// TODO: publish it to end user
}

func (d *daemon) publishConfChange(ent raftpb.Entry) {
	var err error
	cc := new(raftpb.ConfChange)
	mem := new(api.Member)

	defer func() {
		d.msgbus.Broadcast(cc.ID, err)
		if err != nil {
			log.Warnf(
				"raft/daemon: An error occured while publish conf change, Err: %s",
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

	// TODO: need to check that removed added etc is not the current node
	switch cc.Type {
	case raftpb.ConfChangeAddNode:
		err = d.pool.Add(*mem)
	case raftpb.ConfChangeUpdateNode:
		err = d.pool.Update(*mem)
	case raftpb.ConfChangeRemoveNode:
		err = d.pool.Remove(*mem)
	}

	d.cState = *d.node.ApplyConfChange(cc)
}

// process the incoming messages from the given chan.
func (d *daemon) process(sub *Subscription) {
	d.wg.Add(1)
	defer d.wg.Done()
	defer sub.Unsubscribe()

	for {
		select {
		case v := <-sub.Chan():
			if err := d.node.Step(d.ctx, v.(raftpb.Message)); err != nil {
				log.Warnf(
					"raft/daemon: Failed to process raft message, Err: %s",
					err,
				)
			}
		case <-d.ctx.Done():
			return
		}
	}
}

func (d *daemon) send(msgs []raftpb.Message) {
	lg := func(m raftpb.Message, str string) {
		log.Warnf(
			"raft/daemon: Failed to send message %s to member %x, Err: %s",
			m.Type,
			m.To,
			str,
		)
	}

	log.Debug("raft/daemon: Sending messages to raft cluster members")

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

	sub := d.msgbus.Subscribe(uint64(1)) // TODO: add snapshot event id.
	defer sub.Unsubscribe()

	for {
		select {
		case <-sub.Chan():
			if d.appliedIndex.Get()-d.snapIndex.Get() <= d.cfg.SnapInterval() {
				continue
			}

			if _, err := d.CreateSnapshot(); err != nil {
				log.Errorf(
					"raft/daemon: Failed to create new snapshot at index %s, Err: %s",
					d.appliedIndex,
					err,
				)
			}
		case <-d.ctx.Done():
			return
		}

	}
}
