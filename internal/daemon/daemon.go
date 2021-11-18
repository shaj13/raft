package daemon

import (
	"context"
	"encoding/binary"
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
	ErrStopped  = errors.New("raft: node not ready yet or has been stopped")
	ErrNoLeader = errors.New("raft: no elected cluster leader")
)

//go:generate mockgen -package daemonmock  -source internal/daemon/daemon.go -destination internal/mocks/daemon/daemon.go
//go:generate mockgen -package daemon  -source vendor/go.etcd.io/etcd/raft/v3/node.go -destination internal/daemon/node_test.go

type Daemon interface {
	LinearizableRead(ctx context.Context) error
	Push(m etcdraftpb.Message) error
	TransferLeadership(context.Context, uint64) error
	Status() (raft.Status, error)
	Shutdown(context.Context) error
	ProposeReplicate(ctx context.Context, data []byte) error
	ProposeConfChange(ctx context.Context, m *raftpb.Member, t etcdraftpb.ConfChangeType) error
	CreateSnapshot() (etcdraftpb.Snapshot, error)
	Start(addr string, oprs ...Operator) error
	ReportUnreachable(id uint64)
	ReportSnapshot(id uint64, status raft.SnapshotStatus)
	ReportShutdown(id uint64)
}

func New(cfg Config) Daemon {
	d := &daemon{}
	d.cfg = cfg
	d.fsm = d.cfg.StateMachine()
	d.storage = cfg.Storage()
	d.msgbus = msgbus.New()
	d.pool = cfg.Pool()
	d.started = atomic.NewBool()
	d.appliedIndex = atomic.NewUint64()
	d.snapIndex = atomic.NewUint64()
	return d
}

type daemon struct {
	ctx    context.Context
	cancel context.CancelFunc
	fsm    StateMachine
	local  *raftpb.Member
	cfg    Config
	node   raft.Node
	// wg waits for most of the daemon goroutines to be terminated before
	// shutting down the node.
	wg sync.WaitGroup
	// propwg waits for all the proposals to be terminated before
	// shutting down the node.
	propwg sync.WaitGroup
	// processwg waits for all the process goroutines to be terminated before
	// shutting down the node.
	processwg    sync.WaitGroup
	cache        *raft.MemoryStorage
	storage      storage.Storage
	msgbus       *msgbus.MsgBus
	idgen        *idutil.Generator
	pool         membership.Pool
	started      *atomic.Bool
	snapIndex    *atomic.Uint64
	appliedIndex *atomic.Uint64
	proposec     chan etcdraftpb.Message
	msgc         chan etcdraftpb.Message
	snapshotc    chan chan error
	confState    *etcdraftpb.ConfState
}

func (d *daemon) LinearizableRead(ctx context.Context) error {
	if d.started.False() {
		return ErrStopped
	}

	d.propwg.Add(1)
	defer d.propwg.Done()

	// read raft leader index.
	index, err := func() (uint64, error) {
		dur := d.cfg.TickInterval() * 5
		buf := make([]byte, 8)
		id := d.idgen.Next()
		binary.BigEndian.PutUint64(buf, id)
		sub := d.msgbus.SubscribeOnce(id)
		t := time.NewTicker(dur)

		defer t.Stop()
		defer sub.Unsubscribe()

		for {
			err := d.node.ReadIndex(ctx, buf)
			if err != nil {
				return 0, err
			}

			select {
			case <-t.C:
			case v := <-sub.Chan():
				if err, ok := v.(error); ok {
					return 0, err
				}
				return v.(uint64), nil
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-d.ctx.Done():
				return 0, ErrStopped
			}
		}
	}()

	if err != nil {
		return err
	}

	// current node is up to date.
	if index <= d.appliedIndex.Get() {
		return nil
	}

	// wait until leader index applied into this node.
	sub := d.msgbus.SubscribeOnce(index)
	defer sub.Unsubscribe()

	select {
	case v := <-sub.Chan():
		if v != nil {
			return v.(error)
		}
	case <-ctx.Done():
		return ctx.Err()
	case <-d.ctx.Done():
		return ErrStopped
	}

	return nil
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

	ctx, cancel := context.WithTimeout(context.Background(), d.cfg.DrainTimeout())
	defer cancel()

	if err := d.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
}

// Push msg to the daemon queue.
func (d *daemon) Push(msg etcdraftpb.Message) error {
	if d.started.False() {
		return ErrStopped
	}

	d.propwg.Add(1)
	defer d.propwg.Done()

	if err := d.ctx.Err(); err != nil {
		return err
	}

	// chan based on msg type.
	c := d.msgc
	if msg.Type == etcdraftpb.MsgProp {
		c = d.proposec
	}

	select {
	case c <- msg:
	case <-d.ctx.Done():
		return d.ctx.Err()
	default:
		return errors.New("buffer is full (overloaded network)")
	}

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
func (d *daemon) Shutdown(ctx context.Context) error {
	if d.started.False() {
		return ErrStopped
	}

	d.started.UnSet()

	// spawn a goroutine to force shutdown when the provided context
	// expires before the graceful shutdown is complete.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func(ctx context.Context) {
		<-ctx.Done()
		d.cancel()
	}(ctx)

	fns := []func() error{
		nopClose(d.propwg.Wait),
		nopClose(func() {
			close(d.proposec)
			close(d.msgc)
			d.processwg.Wait()
		}),
		nopClose(d.cancel),
		nopClose(d.wg.Wait),
		nopClose(func() { close(d.snapshotc) }),
		nopClose(d.node.Stop),
		d.msgbus.Clsoe,
		d.storage.Close,
		func() error {
			return d.pool.TearDown(ctx)
		},
	}

	for _, fn := range fns {
		if err := fn(); err != nil {
			return err
		}
	}

	return nil
}

// TransferLeadership attempts to transfer leadership to the given transferee.
func (d *daemon) TransferLeadership(ctx context.Context, transferee uint64) error {
	if d.started.False() {
		return ErrStopped
	}

	d.propwg.Add(1)
	defer d.propwg.Done()

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
		case <-d.ctx.Done():
			return ErrStopped
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
	case <-d.ctx.Done():
		return ErrStopped
	}
}

// ProposeConfChange proposes a configuration change to the cluster pool members.
func (d *daemon) ProposeConfChange(ctx context.Context, m *raftpb.Member, cct etcdraftpb.ConfChangeType) error {
	if d.started.False() {
		return ErrStopped
	}

	d.propwg.Add(1)
	defer d.propwg.Done()

	id, err := d.proposeConfChange(ctx, m, cct)
	if err != nil {
		return err
	}

	// wait for changes to be done
	sub := d.msgbus.SubscribeOnce(id)
	defer sub.Unsubscribe()

	select {
	case v := <-sub.Chan():
		if v != nil {
			return v.(error)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-d.ctx.Done():
		return ErrStopped
	}
}

// CreateSnapshot begin a snapshot and return snap metadata.
func (d *daemon) CreateSnapshot() (etcdraftpb.Snapshot, error) {
	if d.started.False() {
		return etcdraftpb.Snapshot{}, ErrStopped
	}

	appliedIndex := d.appliedIndex.Get()
	snapIndex := d.snapIndex.Get()

	if appliedIndex == snapIndex {
		// up to date just return the latest snap to load it from disk.
		return d.cache.Snapshot()
	}

	c := make(chan error)
	d.snapshotc <- c
	if err := <-c; err != nil {
		return etcdraftpb.Snapshot{}, err
	}

	return d.cache.Snapshot()
}

// Start daemon.
func (d *daemon) Start(addr string, oprs ...Operator) error {
	// create memory storage at first place so the operators append hs/ents
	// and to avoid using the same storage on different start invocations.
	d.cache = raft.NewMemoryStorage()
	sp := setup{addr: addr}
	ssp := stateSetup{publishSnapshotFile: d.publishSnapshotFile}
	rm := removedMembers{}
	oprs = append(oprs, sp, ssp, rm)
	ost, err := invoke(d, oprs...)
	if err != nil {
		return err
	}

	if d.node == nil {
		return errors.New("raft: node not initialized, use raft.WithInitCluster() or raft.WithRestart()")
	}

	// set local member.
	d.local = ost.local
	d.idgen = idutil.NewGenerator(uint16(d.local.ID), time.Now())
	d.ctx, d.cancel = context.WithCancel(d.cfg.Context())
	d.proposec = make(chan etcdraftpb.Message, 4096)
	d.msgc = make(chan etcdraftpb.Message, 4096)
	d.snapshotc = make(chan chan error)
	d.started.Set()

	go d.process(d.proposec)
	go d.process(d.msgc)
	return d.eventLoop()
}

func (d *daemon) eventLoop() error {
	d.wg.Add(1)
	defer d.wg.Done()

	ticker := time.NewTicker(d.cfg.TickInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.node.Tick()
		case rd := <-d.node.Ready():
			prevIndex := d.appliedIndex.Get()

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
			d.publishReadState(rd.ReadStates)
			d.publishAppliedIndices(prevIndex, d.appliedIndex.Get())
			d.promotions()
			d.maybeCreateSnapshot()
			d.node.Advance()
		case c := <-d.snapshotc:
			c <- d.createSnapshot()
		case <-d.ctx.Done():
			return ErrStopped
		}
	}
}

func (d *daemon) proposeConfChange(ctx context.Context, m *raftpb.Member, t etcdraftpb.ConfChangeType) (uint64, error) {
	buf, err := m.Marshal()
	if err != nil {
		return 0, err
	}

	cc := etcdraftpb.ConfChange{
		ID:      d.idgen.Next(),
		Type:    t,
		NodeID:  m.ID,
		Context: buf,
	}

	log.Debugf("raft.daemon: propose conf change, change id => %d", cc.ID)
	return cc.ID, d.node.ProposeConfChange(ctx, cc)
}

func (d *daemon) publishReadState(rss []raft.ReadState) {
	for _, rs := range rss {
		id := binary.BigEndian.Uint64(rs.RequestCtx)
		d.msgbus.Broadcast(id, rs.Index)
	}
}

func (d *daemon) publishAppliedIndices(prev, curr uint64) {
	for i := prev + 1; i < curr+1; i++ {
		d.msgbus.Broadcast(i, nil)
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

	meta := snap.Metadata
	sf, err := d.storage.Snapshotter().Read(meta.Term, meta.Index)
	if err != nil {
		return err
	}

	return d.publishSnapshotFile(sf)
}

func (d *daemon) publishSnapshotFile(sf *storage.Snapshot) error {
	snap := sf.Raw

	if err := d.cache.ApplySnapshot(snap); err != nil {
		return err
	}

	d.pool.Restore(sf.Members)

	if err := d.fsm.Restore(sf.Data); err != nil {
		return err
	}

	d.confState = &snap.Metadata.ConfState
	d.snapIndex.Set(snap.Metadata.Index)
	d.appliedIndex.Set(snap.Metadata.Index)
	return nil
}

func (d *daemon) publishCommitted(ents []etcdraftpb.Entry) {
	for _, ent := range ents {
		if ent.Type == etcdraftpb.EntryNormal && len(ent.Data) > 0 {
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
			log.Warnf("raft.daemon: publishing conf change: %v", err)
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

	switch cc.Type {
	case etcdraftpb.ConfChangeAddNode, etcdraftpb.ConfChangeAddLearnerNode:
		err = d.pool.Add(*mem)
	case etcdraftpb.ConfChangeUpdateNode:
		err = d.pool.Update(*mem)
	case etcdraftpb.ConfChangeRemoveNode:
		d.wg.Add(1)
		go func(mem raftpb.Member) {
			defer d.wg.Done()
			select {
			// wait for two ticks then go and remove the member from the pool.
			// to make sure the commit ack sent before closing connection.
			case <-time.After(d.cfg.TickInterval() * 2):
				if err := d.pool.Remove(mem); err != nil {
					log.Errorf("raft.daemon: removing member %x: %v", mem.ID, err)
				}
			case <-d.ctx.Done():
				return
			}
		}(*mem)
	}

	d.confState = d.node.ApplyConfChange(cc)
}

// process the incoming messages from the given chan.
func (d *daemon) process(c chan etcdraftpb.Message) {
	d.processwg.Add(1)
	defer d.processwg.Done()

	// process must keep processing msg until c closed or ctx.Done(),
	// for graceful shutdown purposes.
	for m := range c {
		if err := d.ctx.Err(); err != nil {
			return
		}

		if err := d.node.Step(d.ctx, m); err != nil {
			log.Warnf("raft.daemon: process raft message: %v", err)
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

		if d.forceSnapshot(m) {
			continue
		}

		if err := mem.Send(m); err != nil {
			lg(m, err.Error())
		}
	}
}

func (d *daemon) promotions() {
	rs := d.node.Status()

	// the current node is not the leader.
	if rs.Progress == nil {
		return
	}

	promotions := []raftpb.Member{}
	membs := d.pool.Members()
	reachables := 0
	voters := 0

	for _, mem := range membs {
		raw := mem.Raw()
		if raw.Type == raftpb.VoterMember {
			voters++
		}

		if mem.IsActive() && raw.Type == raftpb.VoterMember {
			reachables++
		}

		if raw.Type != raftpb.StagingMember {
			continue
		}

		leader := rs.Progress[rs.ID].Match
		staging := rs.Progress[raw.ID].Match

		// the staging Match not caught up with the leader yet.
		if float64(staging) < float64(leader)*0.9 {
			continue
		}

		(&raw).Type = raftpb.VoterMember
		promotions = append(promotions, raw)
	}

	// quorum lost and the cluster unavailable, no new logs can be committed.
	if reachables < voters/2+1 {
		return
	}

	for _, m := range promotions {
		log.Infof("raft.daemon: promoting staging member %x", m.ID)
		ctx, cancel := context.WithTimeout(d.ctx, d.cfg.TickInterval()*5)
		_, err := d.proposeConfChange(ctx, &m, etcdraftpb.ConfChangeAddNode)
		if err != nil {
			log.Warnf("raft.daemon: promoting staging member %x: %v", m.ID, err)
		}
		cancel()
	}
}

func (d *daemon) forceSnapshot(msg etcdraftpb.Message) bool {
	if msg.Type != etcdraftpb.MsgSnap {
		return false
	}

	found := false
	cs := msg.Snapshot.Metadata.ConfState
	for _, set := range [][]uint64{
		cs.Voters,
		cs.Learners,
		cs.VotersOutgoing,
		// `LearnersNext` doesn't need to be checked. According to the rules, if a peer in
		// `LearnersNext`, it has to be in `VotersOutgoing`.
	} {

		for _, id := range set {
			if id == msg.To {
				found = true
				break
			}
		}

		if found {
			break
		}
	}

	if found {
		return false
	}

	log.Debugf("raft.daemon: force new snapshot %x it is not in the ConfState", msg.To)

	// report snapshot failure, to re-send the new snapshot.
	defer d.ReportSnapshot(msg.To, raft.SnapshotFailure)

	if err := d.createSnapshot(); err != nil {
		log.Warnf("raft.daemon: force new snapshot: %v", err)
	}

	return true
}

func (d *daemon) maybeCreateSnapshot() {
	if d.appliedIndex.Get()-d.snapIndex.Get() <= d.cfg.SnapInterval() {
		return
	}

	if err := d.createSnapshot(); err != nil {
		log.Errorf(
			"raft.daemon: creating new snapshot at index %s failed: %v",
			d.appliedIndex,
			err,
		)
	}
}

func (d *daemon) createSnapshot() error {
	appliedIndex := d.appliedIndex.Get()
	snapIndex := d.snapIndex.Get()

	if appliedIndex == snapIndex {
		return nil
	}

	log.Infof(
		"raft.daemon: start snapshot [applied index: %d | last snapshot index: %d]",
		appliedIndex,
		snapIndex,
	)

	r, err := d.fsm.Snapshot()
	if err != nil {
		return err
	}

	snap, err := d.cache.CreateSnapshot(appliedIndex, d.confState, nil)
	if err != nil {
		return err
	}

	ss := storage.Snapshot{
		SnapshotState: raftpb.SnapshotState{
			Raw:     snap,
			Members: d.pool.Snapshot(),
		},
		Data: r,
	}

	if err := d.storage.Snapshotter().Write(&ss); err != nil {
		return err
	}

	if err := d.storage.SaveSnapshot(snap); err != nil {
		return err
	}

	d.snapIndex.Set(appliedIndex)

	if appliedIndex <= d.cfg.SnapInterval() {
		return nil
	}

	compactIndex := appliedIndex - d.cfg.SnapInterval()
	if err := d.cache.Compact(compactIndex); err != nil {
		return err
	}

	log.Infof("raft.daemon: compacted log at index %d", compactIndex)
	return nil
}

func nopClose(fn func()) func() error {
	return func() error {
		fn()
		return nil
	}
}
