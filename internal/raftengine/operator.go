package raftengine

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/rakoo/raft/internal/raftpb"
	"github.com/rakoo/raft/internal/storage"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

// order define a weight to operators to obtain the execution order.
var order = map[string]int{
	new(setup).String():           0,
	new(members).String():         1,
	new(forceNewCluster).String(): 2,
	new(restore).String():         2,
	new(stateSetup).String():      3,
	new(forceJoin).String():       4,
	new(join).String():            4,
	new(initCluster).String():     4,
	new(restart).String():         4,
	new(fallback).String():        4,
	new(removedMembers).String():  5,
}

// bootstrapFunc declare function signature for initializes and return
// a raft.Node ready for use.
type bootstrapFunc func(cfg Config, peers []raft.Peer) raft.Node

// Members returns operator that adds the given members to the raft node.
func Members(membs ...raftpb.Member) Operator {
	return members{membs: membs}
}

// Join returns operator that sends rpc request to join an existing cluster.
func Join(addr string, timeout time.Duration) Operator {
	return join{
		forceJoin: ForceJoin(addr, timeout).(forceJoin),
	}
}

// ForceJoin returns operator that sends rpc request to join an existing cluster,
// even if already part of a cluster.
func ForceJoin(addr string, timeout time.Duration) Operator {
	return forceJoin{addr: addr, timeout: timeout}
}

// InitCluster returns operator that initializes a new cluster and create first raft node..
func InitCluster() Operator {
	return initCluster{
		bootstrap: bootstrap,
	}
}

// ForceNewCluster returns operator that initializes a new cluster from state dir.
func ForceNewCluster() Operator {
	return forceNewCluster{}
}

// Restore returns operator that initializes a new cluster from snapshot file
func Restore(path string) Operator {
	return restore{path: path}
}

// Restart returns operator that restarts raft node from state dir.
func Restart() Operator {
	return restart{
		bootstrap: bootstrap,
	}
}

// Fallback returns operator that can be used if other operators do not succeed.
func Fallback(ops ...Operator) Operator {
	return &fallback{operators: ops}
}

type forceJoin struct {
	addr    string
	timeout time.Duration
}

func (f forceJoin) before(ost *operatorsState) error {
	ctx, cancel := context.WithTimeout(context.TODO(), f.timeout)
	defer cancel()

	rpc, err := ost.eng.cfg.Dial()(ctx, f.addr)
	if err != nil {
		return err
	}

	resp, err := rpc.Join(ctx, *ost.local)
	if err != nil {
		return err
	}

	ost.local.ID, ost.membs = resp.ID, resp.Members
	return nil
}

func (f forceJoin) after(ost *operatorsState) error {
	for _, mem := range ost.membs {
		if err := ost.eng.pool.Add(mem); err != nil {
			return err
		}
	}

	r := restart{bootstrap: bootstrap}
	return r.after(ost)
}

func (f forceJoin) String() string {
	return "ForceJoin"
}

type join struct {
	forceJoin
}

func (j join) before(ost *operatorsState) error {
	if ost.hasExistingState {
		return errors.New("raft: this node is already part of a cluster")
	}
	return j.forceJoin.before(ost)
}

func (j join) String() string {
	return "Join"
}

type initCluster struct {
	bootstrap bootstrapFunc
}

func (c initCluster) before(ost *operatorsState) error {
	if ost.hasExistingState {
		return errors.New("raft: cluster is already exist")
	}
	return nil
}

func (c initCluster) after(ost *operatorsState) error {
	membs := ost.membs
	membs = append([]raftpb.Member{*ost.local}, membs...)
	peers := make([]raft.Peer, len(membs))

	for i, mem := range membs {
		peers[i] = raft.Peer{
			ID:      mem.ID,
			Context: pbutil.MustMarshal(&mem),
		}
	}

	ost.eng.node = c.bootstrap(ost.eng.cfg, peers)
	return nil
}

func (c initCluster) String() string {
	return "InitCluster"
}

type restart struct {
	bootstrap bootstrapFunc
}

func (r restart) before(ost *operatorsState) error {
	if !ost.hasExistingState {
		return errors.New("raft: node state not found")
	}
	return nil
}

func (r restart) after(ost *operatorsState) error {
	ost.eng.node = r.bootstrap(ost.eng.cfg, nil)
	return nil
}

func (r restart) String() string {
	return "Restart"
}

type fallback struct {
	operators []Operator
	success   Operator
}

func (f *fallback) before(ost *operatorsState) error {
	errs := []string{}
	for _, op := range f.operators {
		_, ok := op.(interface {
			noFallback()
		})
		if ok {
			return fmt.Errorf("raft: %s can't be used with fallback", op)
		}

		// call operator.
		err := op.before(ost)
		if err == nil {
			f.success = op
			return nil
		}

		errs = append(errs, err.Error())
	}

	return fmt.Errorf(strings.Join(errs, ", "))
}

func (f *fallback) after(ost *operatorsState) error {
	if f.success == nil {
		panic("fallback.after called before fallback.before")
	}

	return f.success.after(ost)
}

func (f fallback) String() string {
	return "Fallback"
}

type stateSetup struct {
	publishSnapshotFile func(*storage.Snapshot) error
}

func (s stateSetup) before(ost *operatorsState) (err error) { return }
func (s stateSetup) after(ost *operatorsState) (err error) {
	if !ost.hasExistingState {
		return
	}

	if !raft.IsEmptySnap(ost.sf.Raw) {
		if err := s.publishSnapshotFile(ost.sf); err != nil {
			return err
		}
	}

	_ = ost.eng.cache.SetHardState(ost.hst)
	_ = ost.eng.cache.Append(ost.ents)
	return
}

func (s stateSetup) String() string {
	return "StateSetup"
}

type setup struct {
	addr string
}

func (s setup) before(ost *operatorsState) (err error) {
	ost.hasExistingState = ost.eng.storage.Exist()
	ost.local = &raftpb.Member{
		// generate a random id in case this is the first member in the cluster.
		ID:      uint64(rand.Int63()) + 1,
		Address: s.addr,
	}
	return
}

func (s setup) after(ost *operatorsState) (err error) {
	if len(ost.local.Address) == 0 {
		return errors.New("raft: no address set, use raft.WithAddress() or raft.WithMembers()")
	}

	meta := pbutil.MustMarshal(ost.local)
	meta, ost.hst, ost.ents, ost.sf, err = ost.eng.storage.Boot(meta)
	if err != nil {
		return
	}

	local := new(raftpb.Member)
	pbutil.MustUnmarshal(local, meta)

	// create memory storage at first place so the operators append hs/ents
	// and to avoid using the same storage on different start invocations.
	ost.eng.cache = raft.NewMemoryStorage()
	cfg := ost.eng.cfg.RaftConfig()
	cfg.ID = local.ID
	cfg.Logger = nodeLogger{ost.eng.cfg.Logger()}
	cfg.Storage = ost.eng.cache
	ost.cfg = cfg
	ost.local = local

	ost.eng.pool.RegisterTypeMatcher(func(m raftpb.Member) raftpb.MemberType {
		if cfg.ID == m.ID && m.Type != raftpb.RemovedMember {
			return raftpb.LocalMember
		}
		return m.Type
	})

	return nil
}

func (s setup) String() string {
	return "Setup"
}

type forceNewCluster struct{}

func (forceNewCluster) noFallback()                            {}
func (forceNewCluster) before(ost *operatorsState) (err error) { return }

func (f forceNewCluster) after(ost *operatorsState) (err error) {
	storage := ost.eng.storage
	sf := ost.sf
	local := *ost.local
	membs := ost.membs
	ents := ost.ents
	hs := ost.hst
	next := hs.Commit + 1

	if !raft.IsEmptySnap(sf.Raw) {
		// reset latest snapshot pool members and conf state.
		sf.Members = []raftpb.Member{local}
		sf.Raw.Metadata.ConfState.Voters = []uint64{local.ID}

		err := storage.Snapshotter().Write(sf)
		if err != nil {
			return err
		}

		meta := sf.Raw.Metadata
		sf, err = storage.Snapshotter().Read(meta.Term, meta.Index)
		if err != nil {
			return err
		}

		ost.sf = sf
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

			mem := new(raftpb.Member)
			pbutil.MustUnmarshal(mem, cc.Context)

			mem.Type = raftpb.RemovedMember
			cc.Type = etcdraftpb.ConfChangeRemoveNode
			cc.Context = pbutil.MustMarshal(mem)

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

	// issue add conf change.
	for _, mem := range membs {
		cc := etcdraftpb.ConfChange{
			Type:    etcdraftpb.ConfChangeAddNode,
			NodeID:  mem.ID,
			Context: pbutil.MustMarshal(&mem),
		}

		e := etcdraftpb.Entry{
			Type:  etcdraftpb.EntryConfChange,
			Data:  pbutil.MustMarshal(&cc),
			Term:  hs.Term,
			Index: next,
		}

		ents = append(ents, e)
		next++

	}

	if len(ents) != 0 {
		hs.Commit = ents[len(ents)-1].Index
	}

	ost.ents = ents
	ost.hst = hs
	return storage.SaveEntries(hs, ents)
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

func (r restore) after(ost *operatorsState) (err error) { return }

func (r restore) noFallback() {}

func (r restore) before(ost *operatorsState) (err error) {
	if ost.hasExistingState {
		return errors.New("raft: found orphan node state")
	}

	storage := ost.eng.storage

	// update state to existed.
	ost.hasExistingState = true

	sf, err := storage.Snapshotter().ReadFrom(r.path)
	if err != nil {
		return err
	}

	// boot storage.
	meta := pbutil.MustMarshal(ost.local)
	_, _, _, _, err = storage.Boot(meta)
	if err != nil {
		return err
	}

	// copy membs to be used as membership pool.
	membs := make([]raftpb.Member, len(ost.membs))
	copy(membs, ost.membs)
	membs = append(membs, *ost.local)

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
	if err := storage.SaveEntries(hs, ents); err != nil {
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
	sf.Members = membs
	sf.Raw = snap

	// save new snapshot file to state dir.
	if err := storage.Snapshotter().Write(sf); err != nil {
		return err
	}

	// save snapshot to storage.
	if err := storage.SaveSnapshot(snap); err != nil {
		return err
	}

	// close storage so next operator can boot it again.
	return storage.Close()
}

func (r restore) addOns() []Operator {
	return []Operator{Restart()}
}

func (r restore) String() string {
	return "Restore"
}

type members struct {
	membs []raftpb.Member
}

func (m members) after(ost *operatorsState) (err error) { return }

func (m members) noFallback() {}

func (m members) before(ost *operatorsState) (err error) {
	if len(m.membs) >= 1 {
		id := ost.local.ID
		ost.local = &m.membs[0]
		if ost.local.ID == 0 {
			ost.local.ID = id
		}
		ost.membs = append(ost.membs, m.membs[1:]...)
	}
	return
}

func (m members) String() string {
	return "Members"
}

type removedMembers struct{}

func (rm removedMembers) before(ost *operatorsState) (err error) { return }
func (rm removedMembers) after(ost *operatorsState) (err error) {
	// removed members must be added back to the pool right away,
	// to avoid to connect to them, and getting stuck on add conf change.
	for _, ent := range ost.ents {
		if ent.Index <= ost.hst.Commit && ent.Type == etcdraftpb.EntryConfChange {
			cc := new(etcdraftpb.ConfChange)
			pbutil.MustUnmarshal(cc, ent.Data)
			if cc.Type == etcdraftpb.ConfChangeRemoveNode {
				mem := new(raftpb.Member)
				pbutil.MustUnmarshal(mem, cc.Context)
				if err := ost.eng.pool.Add(*mem); err != nil {
					return err
				}
			}
		}
	}
	return
}

func (rm removedMembers) String() string {
	return "RemovedMembers"
}

func invoke(d *engine, oprs ...Operator) (*operatorsState, error) {
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

	ost := new(operatorsState)
	ost.eng = d

	for _, opr := range oprs {
		if err := opr.before(ost); err != nil {
			return nil, err
		}
	}

	for _, opr := range oprs {
		if err := opr.after(ost); err != nil {
			return nil, err
		}
	}

	return ost, nil
}

// bootstrap implements bootstrapFunc and return's
// an raft.Node ready for use.
func bootstrap(cfg Config, peers []raft.Peer) raft.Node {
	mux := cfg.Mux()
	rcfg := cfg.RaftConfig()
	gid := cfg.GroupID()

	if mux == nil && len(peers) == 0 {
		return raft.RestartNode(rcfg)
	}

	if mux == nil && len(peers) > 0 {
		return raft.StartNode(rcfg, peers)
	}

	rn, err := raft.NewRawNode(rcfg)
	if err != nil {
		panic(err)
	}

	if len(peers) > 0 {
		if err := rn.Bootstrap(peers); err != nil {
			panic(err)
		}
	}

	return mux.add(gid, rn, rcfg)
}
