package raft

import (
	"context"
	"fmt"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/membership"
	raftrpc "github.com/shaj13/raftkit/internal/net/grpc"
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
	ctx = raftrpc.NewDialContext(ctx, grpc.WithInsecure())
	rpc, err := raftrpc.Dial(ctx, t.add)
	if err != nil {
		return err
	}
	return rpc.Message(ctx, msg)
}

func (t tr) Close() error {
	return t.cc.Close()
}

func joinjoin(ctx context.Context, m *api.Member, cluster string) ([]api.Member, error) {
	ctx = raftrpc.NewDialContext(ctx, grpc.WithInsecure())
	rpc, err := raftrpc.Dial(ctx, cluster)
	if err != nil {
		return []api.Member{}, err
	}

	id, p, err := rpc.Join(ctx, *m)
	m.ID = id
	m.Type = api.LocalMember
	fmt.Printf("recived rec, id %x\n", m.ID)
	return p.Members, err
}

func joincont(c *cluster, p *membership.Pool) func(ctx context.Context, m *api.Member) (uint64, []api.Member, error) {
	return func(ctx context.Context, m *api.Member) (uint64, []api.Member, error) {
		fmt.Println(c.IsAvailable())
		fmt.Println("a new member requested to join the cluster")
		var (
			memb Member
			err  error
		)

		if mm, _ := c.GetMemebr(m.ID); mm == nil {
			memb, err = c.AddMember(context.Background(), m.Address)
		} else {
			err = c.UpdateMember(context.Background(), m.ID, m.Address)
			memb, _ = c.GetMemebr(m.ID)
		}

		if err != nil {
			return 0, []api.Member{}, err
		}

		pool := p.Snapshot()

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
}
