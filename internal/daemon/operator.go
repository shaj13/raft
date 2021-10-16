package daemon

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/storage"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

// order define a weight to operator be used on sort and conflict detection.
var order = map[string]int{
	new(setup).String():           0,
	new(forceNewCluster).String(): 1,
	new(restore).String():         1,
	new(stateSetup).String():      2,
	new(forceJoin).String():       3,
	new(join).String():            3,
	new(initCluster).String():     3,
	new(restart).String():         3,
	new(fallback).String():        3,
}

// Operator is a bootstrapper func that determine the action that is to be performed or considered.
type Operator interface {
	fmt.Stringer
	before(d *daemon) error
	after(d *daemon) error
}

// Join TBD
func Join(addr string, timeout time.Duration) Operator {
	return join{
		forceJoin: ForceJoin(addr, timeout).(forceJoin),
	}
}

// ForceJoin TBD
func ForceJoin(addr string, timeout time.Duration) Operator {
	return forceJoin{addr: addr, timeout: timeout}
}

// InitCluster TBD
func InitCluster() Operator {
	return initCluster{}
}

func ForceNewCluster() Operator {
	return forceNewCluster{}
}

func Restore(path string) Operator {
	return restore{path: path}
}

// Reload
func Restart() Operator {
	return restart{}
}

func Fallback(ops ...Operator) Operator {
	return &fallback{operators: ops}
}

type forceJoin struct {
	addr    string
	timeout time.Duration
}

func (f forceJoin) before(d *daemon) error {
	ctx, cancel := context.WithTimeout(context.TODO(), f.timeout)
	defer cancel()

	rpc, err := d.cfg.Dial()(ctx, f.addr)
	if err != nil {
		return err
	}

	d.bState.mem.ID, d.bState.membs, err = rpc.Join(ctx, *d.bState.mem)
	return err
}

func (f forceJoin) after(d *daemon) error {
	for _, mem := range d.bState.membs {
		if err := d.pool.Add(mem); err != nil {
			return err
		}
	}

	d.node = raft.RestartNode(d.bState.raftCfg)
	return nil
}

func (f forceJoin) String() string {
	return "ForceJoin"
}

type join struct {
	forceJoin
}

func (j join) before(d *daemon) error {
	if d.bState.wasExisted {
		return errors.New("raft: this node is already part of a cluster")
	}
	return j.forceJoin.before(d)
}

func (j join) String() string {
	return "Join"
}

type initCluster struct{}

func (c initCluster) before(d *daemon) error {
	if d.bState.wasExisted {
		return errors.New("raft: cluster is already exist")
	}
	return nil
}

func (c initCluster) after(d *daemon) error {
	peer := raft.Peer{
		ID:      d.bState.mem.ID,
		Context: pbutil.MustMarshal(d.bState.mem),
	}

	d.node = raft.StartNode(d.bState.raftCfg, []raft.Peer{peer})
	return nil
}

func (c initCluster) String() string {
	return "InitCluster"
}

type restart struct{}

func (r restart) before(d *daemon) error {
	if !d.bState.wasExisted {
		return errors.New("raft: node state not found")
	}
	return nil
}

func (r restart) after(d *daemon) error {
	d.node = raft.RestartNode(d.bState.raftCfg)
	return nil
}

func (r restart) String() string {
	return "Restart"
}

type fallback struct {
	operators []Operator
	success   Operator
}

func (f *fallback) before(d *daemon) error {
	errs := []string{}
	for _, op := range f.operators {
		_, ok := op.(interface {
			noFallback()
		})
		if ok {
			return fmt.Errorf("raft: %s can't be used with fallback", op)
		}

		// call operator.
		err := op.before(d)
		if err == nil {
			f.success = op
			return nil
		}

		errs = append(errs, err.Error())
	}

	return fmt.Errorf(strings.Join(errs, ", "))
}

func (f *fallback) after(d *daemon) error {
	if f.success == nil {
		panic("fallback.after called before fallback.before")
	}

	return f.success.after(d)
}

func (f fallback) String() string {
	return "Fallback"
}

type stateSetup struct{}

func (s stateSetup) before(d *daemon) (err error) { return }
func (s stateSetup) after(d *daemon) (err error) {
	if !d.bState.wasExisted {
		return
	}
	d.publishSnapshotFile(d.bState.sf)
	d.cache.SetHardState(d.bState.hState)
	d.cache.Append(d.bState.ents)
	return
}

func (st stateSetup) String() string {
	return "StateSetup"
}

type setup struct {
	addr string
}

func (s setup) before(d *daemon) (err error) {
	d.bState = new(bootState)
	d.bState.wasExisted = d.storage.Exist()
	d.bState.mem = &raftpb.Member{
		// generate a random id in case this is the first member in the cluster.
		ID:      uint64(rand.Int63()) + 1,
		Address: s.addr,
		Type:    raftpb.LocalMember,
	}
	return
}

func (s setup) after(d *daemon) (err error) {
	meta := pbutil.MustMarshal(d.bState.mem)
	meta, d.bState.hState, d.bState.ents, d.bState.sf, err = d.storage.Boot(meta)
	if err != nil {
		return
	}

	pbutil.MustUnmarshal(d.bState.mem, meta)

	cfg := d.cfg.RaftConfig()
	cfg.ID = d.bState.mem.ID
	cfg.Storage = d.cache
	d.bState.raftCfg = cfg
	return
}

func (s setup) String() string {
	return "Setup"
}

type forceNewCluster struct{}

func (forceNewCluster) noFallback()                  {}
func (forceNewCluster) before(d *daemon) (err error) { return }

func (f forceNewCluster) after(d *daemon) (err error) {
	sf := d.bState.sf
	local := *d.bState.mem
	ents := d.bState.ents
	hs := d.bState.hState
	next := hs.Commit + 1

	// override latest snapshot pool members.
	if !raft.IsEmptySnap(*sf.Snap) {
		sf.Pool = &raftpb.Pool{
			Members: []raftpb.Member{local},
		}

		err := d.storage.Snapshotter().Write(sf)
		if err != nil {
			return err
		}

		sf, err = d.storage.Snapshotter().Read(*sf.Snap)
		if err != nil {
			return err
		}

		d.bState.sf = sf
	}

	// discard uncommitted entries
	for i, ent := range ents {
		if ent.Index > hs.Commit {
			ents = ents[:i]
			break
		}
	}

	// issue remove conf changes.
	for _, ent := range ents {
		if ent.Type == etcdraftpb.EntryConfChange {
			cc := new(etcdraftpb.ConfChange)
			pbutil.MustUnmarshal(cc, ent.Data)
			if cc.NodeID == local.ID || cc.Type == etcdraftpb.ConfChangeRemoveNode {
				continue
			}

			cc.Type = etcdraftpb.ConfChangeRemoveNode
			e := etcdraftpb.Entry{
				Type:  etcdraftpb.EntryConfChange,
				Data:  pbutil.MustMarshal(cc),
				Term:  hs.Term,
				Index: next,
			}

			ents = append(ents, e)
			next++
		}
	}

	if len(ents) != 0 {
		hs.Commit = ents[len(ents)-1].Index
	}

	d.bState.ents = ents
	d.bState.hState = hs
	return d.storage.SaveEntries(hs, ents)
}

func (f forceNewCluster) addOns() []Operator {
	return []Operator{Restart()}
}

func (f forceNewCluster) String() string {
	return "ForceNewCluster"
}

type restore struct {
	path string
}

func (r restore) after(d *daemon) (err error) { return }

func (r restore) noFallback() {}

func (r restore) before(d *daemon) (err error) {
	if d.bState.wasExisted {
		return errors.New("raft: found orphan node state")
	}
	// update state to existed.
	d.bState.wasExisted = true

	// TODO: remove this when snapshotter updated
	st := d.storage.Snapshotter().(interface {
		ReadFromPath(path string) (*storage.SnapshotFile, error)
	})

	sf, err := st.ReadFromPath(r.path)
	if err != nil {
		return err
	}

	// boot storage.
	meta := pbutil.MustMarshal(d.bState.mem)
	_, _, _, _, err = d.storage.Boot(meta)
	if err != nil {
		return err
	}

	// copy membs to be used as membership pool.
	// membs := make([]raftpb.Member, len(d.bState.membs)+1)
	// copy(d.bState.membs, membs)
	// membs = append(membs, *d.bState.mem)
	membs := []raftpb.Member{*d.bState.mem}

	// issue conf change for membs.
	ents := make([]etcdraftpb.Entry, len(membs))
	nodeIDs := make([]uint64, len(membs))
	for i, m := range membs {
		nodeIDs[i] = m.ID
		cc := etcdraftpb.ConfChange{
			Type:    etcdraftpb.ConfChangeAddNode,
			NodeID:  m.ID,
			Context: pbutil.MustMarshal(&m),
		}

		ents[i] = etcdraftpb.Entry{
			Type:  etcdraftpb.EntryConfChange,
			Term:  1,
			Index: uint64(i + 1),
			Data:  pbutil.MustMarshal(&cc),
		}
	}

	commit, term := uint64(len(ents)), uint64(1)
	hs := etcdraftpb.HardState{
		Term:   term,
		Vote:   membs[0].ID,
		Commit: commit,
	}

	// save entries to storgae.
	if err := d.storage.SaveEntries(hs, ents); err != nil {
		return err
	}

	confState := etcdraftpb.ConfState{
		Voters: nodeIDs,
	}

	snap := etcdraftpb.Snapshot{
		Metadata: etcdraftpb.SnapshotMetadata{
			Index:     commit,
			Term:      term,
			ConfState: confState,
		},
	}

	// update snapshot file meta.
	sf.Pool.Members = membs
	sf.Snap = &snap

	// save new snapshot file to state dir.
	if err := d.storage.Snapshotter().Write(sf); err != nil {
		return err
	}

	// save snapshot to storage.
	if err := d.storage.SaveSnapshot(snap); err != nil {
		return err
	}

	// close storage so next operator can boot it again.
	return d.storage.Close()
}

func (r restore) addOns() []Operator {
	return []Operator{Restart()}
}

func (r restore) String() string {
	return "Restore"
}

func invoke(d *daemon, oprs ...Operator) error {
	for _, opr := range oprs {
		a, ok := opr.(interface {
			addOns() []Operator
		})

		if ok {
			oprs = append(oprs, a.addOns()...)
		}
	}

	sort.SliceStable(oprs, func(i, j int) bool {
		return order[oprs[i].String()] < order[oprs[j].String()]
	})

	for _, opr := range oprs {
		if err := opr.before(d); err != nil {
			return err
		}
	}

	for _, opr := range oprs {
		if err := opr.after(d); err != nil {
			return err
		}
	}

	return nil
}
