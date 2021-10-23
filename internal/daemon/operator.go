package daemon

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/shaj13/raftkit/internal/membership"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/storage"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/raft/v3"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

// node operations funcs.
// abstracted for testing purposes.
var (
	startNode   = raft.StartNode
	restartNode = raft.RestartNode
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

// Members return's operator that add the given members to the raft node.
func Members(urls ...string) Operator {
	return members{urls: urls}
}

// Join return's operator that send rpc request to join an existing cluster.
func Join(addr string, timeout time.Duration) Operator {
	return join{
		forceJoin: ForceJoin(addr, timeout).(forceJoin),
	}
}

// ForceJoin return's operator that send rpc request to join an existing cluster,
// even if already part of a cluster.
func ForceJoin(addr string, timeout time.Duration) Operator {
	return forceJoin{addr: addr, timeout: timeout}
}

// InitCluster return's operator that initialize a new cluster and create first raft node..
func InitCluster() Operator {
	return initCluster{}
}

// ForceNewCluster return's operator that initialize a new cluster from state dir.
func ForceNewCluster() Operator {
	return forceNewCluster{}
}

// Restore return's operator that initialize a new cluster from snapshot file
func Restore(path string) Operator {
	return restore{path: path}
}

// Restart return's operator that restart raft node from state dir.
func Restart() Operator {
	return restart{}
}

// Fallback return's operator that can be used if other operators do not succeed.
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

	rpc, err := ost.daemon.cfg.Dial()(ctx, f.addr)
	if err != nil {
		return err
	}

	ost.local.ID, ost.membs, err = rpc.Join(ctx, *ost.local)
	return err
}

func (f forceJoin) after(ost *operatorsState) error {
	for _, mem := range ost.membs {
		if err := ost.daemon.pool.Add(mem); err != nil {
			return err
		}
	}

	return restart{}.after(ost)
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

type initCluster struct{}

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

	ost.daemon.node = startNode(ost.cfg, peers)
	return nil
}

func (c initCluster) String() string {
	return "InitCluster"
}

type restart struct{}

func (r restart) before(ost *operatorsState) error {
	if !ost.hasExistingState {
		return errors.New("raft: node state not found")
	}
	return nil
}

func (r restart) after(ost *operatorsState) error {
	ost.daemon.node = restartNode(ost.cfg)
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
	publishSnapshotFile func(*storage.SnapshotFile) error
}

func (s stateSetup) before(ost *operatorsState) (err error) { return }
func (s stateSetup) after(ost *operatorsState) (err error) {
	if !ost.hasExistingState {
		return
	}

	if !raft.IsEmptySnap(*ost.sf.Snap) {
		if err := s.publishSnapshotFile(ost.sf); err != nil {
			return err
		}
	}

	ost.daemon.cache.SetHardState(ost.hst)
	ost.daemon.cache.Append(ost.ents)
	return
}

func (st stateSetup) String() string {
	return "StateSetup"
}

type setup struct {
	addr string
}

func (s setup) before(ost *operatorsState) (err error) {
	ost.hasExistingState = ost.daemon.storage.Exist()
	ost.local = &raftpb.Member{
		// generate a random id in case this is the first member in the cluster.
		ID:      uint64(rand.Int63()) + 1,
		Address: s.addr,
		Type:    raftpb.LocalMember,
	}
	return
}

func (s setup) after(ost *operatorsState) (err error) {
	if len(ost.local.Address) == 0 {
		return errors.New("raft: no address set, use raft.WithAddress() or raft.WithMembers")
	}

	meta := pbutil.MustMarshal(ost.local)
	meta, ost.hst, ost.ents, ost.sf, err = ost.daemon.storage.Boot(meta)
	if err != nil {
		return
	}

	local := new(raftpb.Member)
	pbutil.MustUnmarshal(local, meta)

	cfg := ost.daemon.cfg.RaftConfig()
	cfg.ID = local.ID
	cfg.Storage = ost.daemon.cache
	ost.cfg = cfg
	ost.local = local
	return
}

func (s setup) String() string {
	return "Setup"
}

type forceNewCluster struct{}

func (forceNewCluster) noFallback()                            {}
func (forceNewCluster) before(ost *operatorsState) (err error) { return }

func (f forceNewCluster) after(ost *operatorsState) (err error) {
	storage := ost.daemon.storage
	sf := ost.sf
	local := *ost.local
	membs := ost.membs
	ents := ost.ents
	hs := ost.hst
	next := hs.Commit + 1

	// override latest snapshot pool members.
	if !raft.IsEmptySnap(*sf.Snap) {
		sf.Pool = &raftpb.Pool{
			Members: append([]raftpb.Member{local}, membs...),
		}

		err := storage.Snapshotter().Write(sf)
		if err != nil {
			return err
		}

		sf, err = storage.Snapshotter().Read(*sf.Snap)
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

	storage := ost.daemon.storage

	// update state to existed.
	ost.hasExistingState = true

	sf, err := storage.Snapshotter().ReadFromPath(r.path)
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
	sf.Pool.Members = membs
	sf.Snap = &snap

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
	urls []string
}

func (m members) after(ost *operatorsState) (err error) { return }

func (m members) noFallback() {}

func (m members) before(ost *operatorsState) (err error) {
	for i, raw := range m.urls {
		id, addr, err := membership.ParseURL(raw)
		if err != nil {
			return err
		}

		mem := raftpb.Member{
			ID:      id,
			Address: addr,
			Type:    raftpb.RemoteMember,
		}

		if i == 0 {
			mem.Type = raftpb.LocalMember
			ost.local = &mem
			continue
		}

		ost.membs = append(ost.membs, mem)
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
			pbutil.MaybeUnmarshal(cc, ent.Data)
			if cc.Type == etcdraftpb.ConfChangeRemoveNode {
				mem := new(raftpb.Member)
				pbutil.MustMarshal(mem)
				if err := ost.daemon.pool.Add(*mem); err != nil {
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

func invoke(d *daemon, oprs ...Operator) (*operatorsState, error) {
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
	ost.daemon = d

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
