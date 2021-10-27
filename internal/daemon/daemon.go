package daemon

import (
	"context"
	"errors"
	"fmt"
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
	ErrStopped   = errors.New("raft: daemon not ready yet or has been stopped")
	ErrNotLeader = errors.New("raft: node is not the leader")
	ErrNoLeader  = errors.New("raft: no elected cluster leader")
)

type Daemon interface {
	Push(m etcdraftpb.Message) error
	TransferLeadership(context.Context, uint64) error
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
	d.disableProposal = cfg.RaftConfig().DisableProposalForwarding
	return d
}

type daemon struct {
	ctx             context.Context
	cancel          context.CancelFunc
	fsm             FSM
	local           *raftpb.Member
	cfg             Config
	node            raft.Node
	ticker          *time.Ticker
	wg              sync.WaitGroup
	propwg          sync.WaitGroup
	cache           *raft.MemoryStorage
	storage         storage.Storage
	msgbus          *msgbus.MsgBus
	idgen           *idutil.Generator
	pool            membership.Pool
	started         *atomic.Bool
	snapIndex       *atomic.Uint64
	appliedIndex    *atomic.Uint64
	proposec        chan etcdraftpb.Message
	msgc            chan etcdraftpb.Message
	snapshotc       chan struct{}
	csMu            sync.Mutex // guard ConfState.
	cState          *etcdraftpb.ConfState
	disableProposal bool
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

	log.Info("raft.daemon: this member removed from the cluster! shutting down.")

	if err := d.Close(); err != nil {
		log.Panic(err)
	}
}

// Push m to the daemon queue.
func (d *daemon) Push(m etcdraftpb.Message) error {
	if d.started.False() {
		return ErrStopped
	}

	// chan based on msg type.
	c := d.msgc
	if m.Type == etcdraftpb.MsgProp {
		c = d.proposec
	}

	c <- m
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
	d.wg.Wait()
	d.propwg.Wait()
	d.node.Stop()
	d.msgbus.Clsoe()
	return nil
}

// TransferLeadership attempts to transfer leadership to the given transferee.
func (d *daemon) TransferLeadership(ctx context.Context, transferee uint64) error {
	if d.started.False() {
		return ErrStopped
	}

	log.Infof("raft.daemon: start transfer leadership %x -> %x", d.node.Status().Lead, transferee)

	d.node.TransferLeadership(ctx, d.node.Status().Lead, transferee)
	ticker := time.NewTicker(d.cfg.TickInterval() / 10)
	defer ticker.Stop()
	for {
		leader := d.node.Status().Lead
		if leader != raft.None && leader == transferee {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}

	return nil
}

// ProposeReplicate proposes to replicate the data to be appended to the raft log.
func (d *daemon) ProposeReplicate(ctx context.Context, data []byte) error {
	if d.started.False() {
		return ErrStopped
	}

	if d.node.Status().Lead != d.local.ID && d.disableProposal {
		return ErrNotLeader
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

	log.Debugf("raft.daemon: propose replicate data, change id => %d", r.CID)

	if err := d.node.Propose(ctx, buf); err != nil {
		return err
	}

	// wait for changes to be done
	sub := d.msgbus.SubscribeOnce(r.CID)
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

// ProposeConfChange proposes a configuration change to the cluster pool members.
func (d *daemon) ProposeConfChange(ctx context.Context, m *raftpb.Member, t etcdraftpb.ConfChangeType) error {
	if d.started.False() {
		return ErrStopped
	}

	if d.node.Status().Lead != d.local.ID && d.disableProposal {
		return ErrNotLeader
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

	log.Debugf("raft.daemon: propose conf change, change id => %d", cc.ID)

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
		return d.cache.CreateSnapshot(appliedIndex, d.confState(nil), nil)
	}

	log.Infof(
		"raft.daemon: start snapshot [applied index: %d | last snapshot index: %d]",
		appliedIndex,
		snapIndex,
	)

	r, err := d.fsm.Snapshot()
	if err != nil {
		return etcdraftpb.Snapshot{}, err
	}

	snap, err := d.cache.CreateSnapshot(appliedIndex, d.confState(nil), nil)
	if err != nil {
		return snap, err
	}

	sf := storage.SnapshotFile{
		Snap: &snap,
		Pool: &raftpb.Pool{
			Members: d.pool.Snapshot(),
		},
		Data: r,
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

// Start daemon.
func (d *daemon) Start(addr string, oprs ...Operator) error {
	sp := setup{addr: addr}
	ssp := stateSetup{publishSnapshotFile: d.publishSnapshotFile}
	rm := removedMembers{}
	oprs = append(oprs, sp, ssp, rm)
	ost, err := invoke(d, oprs...)
	if err != nil {
		return err
	}

	// set local member.
	d.local = ost.local
	d.proposec = make(chan etcdraftpb.Message, 4096)
	d.msgc = make(chan etcdraftpb.Message, 4096)
	d.snapshotc = make(chan struct{}, 10)
	d.idgen = idutil.NewGenerator(uint16(d.local.ID), time.Now())

	d.started.Set()
	go d.process(d.proposec)
	go d.process(d.msgc)
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

			if rd.SoftState != nil && rd.SoftState.Lead == raft.None {
				d.msgbus.BroadcastToAll(ErrNoLeader)
			}

			d.publishCommitted(rd.CommittedEntries)

			d.snapshotc <- struct{}{}

			d.node.Advance()
		case <-d.ctx.Done():
			return ErrStopped
		}
	}
}

func (d *daemon) publishSnapshot(snap etcdraftpb.Snapshot) error {
	if raft.IsEmptySnap(snap) {
		return nil
	}

	if snap.Metadata.Index <= d.appliedIndex.Get() {
		return fmt.Errorf(
			"raft: snapshot index [%d] should > progress.appliedIndex [%s]",
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

	if err := d.fsm.Restore(sf.Data); err != nil {
		return err
	}

	d.confState(&snap.Metadata.ConfState)
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
				"raft.daemon: publishing replicate data: %v",
				err,
			)
		}
	}()

	if err = r.Unmarshal(ent.Data); err != nil {
		return
	}

	log.Debugf("raft.daemon: publishing replicate data, change id => %d", r.CID)

	d.fsm.Apply(r.Data)
}

func (d *daemon) publishConfChange(ent etcdraftpb.Entry) {
	var err error
	cc := new(etcdraftpb.ConfChange)
	mem := new(raftpb.Member)

	defer func() {
		d.msgbus.Broadcast(cc.ID, err)
		if err != nil {
			log.Warnf(
				"raft.daemon: publishing conf change: %v",
				err,
			)
		}
	}()

	if err = cc.Unmarshal(ent.Data); err != nil {
		return
	}

	log.Debugf("raft.daemon: publishing conf change, change id => %d", cc.ID)

	if len(cc.Context) == 0 {
		return
	}

	if err = mem.Unmarshal(cc.Context); err != nil {
		return
	}

	// set the correct member type.
	// this can happen when nodes sends conf change from it's perspective to other nodes.
	if mem.ID != d.local.ID && mem.Type == raftpb.LocalMember {
		mem.Type = raftpb.RemoteMember
	} else if mem.ID != d.local.ID && mem.Type == raftpb.LocalLearnerMember {
		mem.Type = raftpb.LearnerMember
	} else if mem.ID == d.local.ID && mem.Type == raftpb.RemoteMember {
		mem.Type = raftpb.LocalMember
	} else if mem.ID == d.local.ID && mem.Type == raftpb.LearnerMember {
		mem.Type = raftpb.LocalLearnerMember
	}

	switch cc.Type {
	case etcdraftpb.ConfChangeAddNode, etcdraftpb.ConfChangeAddLearnerNode:
		err = d.pool.Add(*mem)
	case etcdraftpb.ConfChangeUpdateNode:
		err = d.pool.Update(*mem)
	case etcdraftpb.ConfChangeRemoveNode:
		go func(mem raftpb.Member) {
			// wait for two ticks then go and remove the member from the pool.
			// to make sure the commit ack sent before closing connection.
			<-time.After(d.cfg.TickInterval() * 2)
			if err := d.pool.Remove(mem); err != nil {
				log.Error("raft.daemon: removing member %x: %v", mem.ID, err)
			}
		}(*mem)
	}

	d.confState(d.node.ApplyConfChange(cc))
}

// process the incoming messages from the given chan.
func (d *daemon) process(c chan etcdraftpb.Message) {
	d.wg.Add(1)
	defer d.wg.Done()

	for {
		select {
		case m := <-c:
			if err := d.node.Step(d.ctx, m); err != nil {
				log.Warnf(
					"raft.daemon: process raft message: %v",
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
			"raft.daemon: sending message %s to member %x: %v",
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

	for {
		select {
		case <-d.snapshotc:
			if d.appliedIndex.Get()-d.snapIndex.Get() <= d.cfg.SnapInterval() {
				continue
			}

			if _, err := d.CreateSnapshot(); err != nil {
				log.Errorf(
					"raft.daemon: creating new snapshot at index %s failed: %v",
					d.appliedIndex,
					err,
				)
			}
		case <-d.ctx.Done():
			return
		}

	}
}

func (d *daemon) confState(cs *etcdraftpb.ConfState) *etcdraftpb.ConfState {
	d.csMu.Lock()
	defer d.csMu.Unlock()

	if cs != nil {
		d.cState = cs
	}

	return d.cState
}
