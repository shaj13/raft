package raft

import (
	"context"
	"fmt"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/membership"
	"github.com/shaj13/raftkit/internal/net"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc"
)

type reporter struct {
	reprotc chan report
}

func (r reporter) ReportUnreachable(id uint64) {
	r.reprotc <- report{
		signal: unreachable,
		id:     id,
	}
}
func (reporter) ReportShutdown(id uint64)                             {}
func (reporter) ReportSnapshot(id uint64, status raft.SnapshotStatus) {}

func dial(ctx context.Context, addr string) (membership.Transport, error) {
	cc, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return tr{cc: cc, add: addr}, nil
}

type tr struct {
	cc  *grpc.ClientConn
	add string
}

func (t tr) RoundTrip(ctx context.Context, msg raftpb.Message) error {
	_, dial := net.GRPC.Get()
	cfg := defaultConfig()
	rpc, err := dial(ctx, cfg, t.add)
	if err != nil {
		return err
	}
	return rpc.Message(ctx, msg)
}

func (t tr) Close() error {
	return t.cc.Close()
}

func joinjoin(ctx context.Context, m *api.Member, cluster string) ([]api.Member, error) {
	_, dial := net.GRPC.Get()
	cfg := defaultConfig()
	rpc, err := dial(ctx, cfg, cluster)
	if err != nil {
		return []api.Member{}, err
	}

	id, p, err := rpc.Join(ctx, *m)
	m.ID = id
	m.Type = api.LocalMember
	fmt.Printf("recived rec, id %x\n", m.ID)
	return p.Members, err
}

type capi struct {
	c    *cluster
	p    *processor
	pool *membership.Pool
}

func (a *capi) Join(ctx context.Context, m *api.Member) (uint64, []api.Member, error) {
	fmt.Println(a.c.IsAvailable())
	fmt.Println("a new member requested to join the cluster")
	var (
		memb Member
		err  error
	)

	if mm, _ := a.c.GetMemebr(m.ID); mm == nil {
		memb, err = a.c.AddMember(context.Background(), m.Address)
	} else {
		err = a.c.UpdateMember(context.Background(), m.ID, m.Address)
		memb, _ = a.c.GetMemebr(m.ID)
	}

	if err != nil {
		return 0, []api.Member{}, err
	}

	pool := a.pool.Snapshot()

	for i, m := range pool {
		if m.Type == api.LocalMember {
			(&m).Type = api.RemoteMember
			pool[i] = m
			continue
		}

		if m.ID == memb.ID() {
			(&m).Type = api.LocalMember
			pool[i] = m
			continue
		}
	}

	fmt.Printf("member added id, %x\n", memb.ID())

	return memb.ID(), pool, nil
}

func (a *capi) Push(ctx context.Context, m raftpb.Message) error {
	return a.p.push(m)
}
