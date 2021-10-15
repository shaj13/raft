package daemon

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/shaj13/raftkit/internal/raftpb"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	"go.etcd.io/etcd/raft/v3"
)

// Operator is a bootstrapper func that determine the action that is to be performed or considered.
type Operator interface {
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

// Reload
func Reload() Operator {
	return reload{}
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

type join struct {
	forceJoin
}

func (j join) before(d *daemon) error {
	if d.bState.wasExisted {
		return fmt.Errorf("raft: this node is already part of a cluster")
	}
	return j.forceJoin.before(d)
}

type initCluster struct{}

func (c initCluster) before(d *daemon) error {
	if d.bState.wasExisted {
		return fmt.Errorf("raft: cluster is already exist")
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

type reload struct{}

func (r reload) before(d *daemon) error {
	if !d.bState.wasExisted {
		return fmt.Errorf("raft: cluster not found")
	}
	return nil
}

func (r reload) after(d *daemon) error {
	d.node = raft.RestartNode(d.bState.raftCfg)
	return nil
}

type fallback struct {
	operators []Operator
	success   Operator
}

func (f *fallback) before(d *daemon) error {
	errs := []string{}
	for _, op := range f.operators {
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
