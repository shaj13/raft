package raft

import (
	"bytes"
	"context"
	"io"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/membership"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

type server struct {
	api.UnimplementedRaftServer
	cfg       *config
	processor *processor
	pool      *membership.Pool
	cluster   *cluster
}

func (s *server) Message(stream api.Raft_MessageServer) error {
	a := new(assembler)
	a.writer = new(bytes.Buffer)

	for {
		c, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.cfg.logger.Warningf("raft: An error occurred while reading from grpc stream, Err: %s", err)
			return err
		}
		err = a.Write(c)
		if err != nil {
			return err
		}
	}

	buf := a.writer.(*bytes.Buffer).Bytes()
	msg := new(raftpb.Message)
	mustUnmarshal(buf, msg)
	s.processor.push(*msg)
	return stream.SendAndClose(&api.StreamResponse{})
}

func (s *server) StreamMessage(ctx context.Context, m *api.MessageRequest) (*api.StreamResponse, error) {
	// msgs := []raftpb.Message{}
	// for {
	// 	m, err := stream.Recv()
	// 	if err == io.EOF {
	// 		break
	// 	}

	// 	if err != nil {
	// 		s.cfg.logger.Warningf("raft: An error occurred while reading from grpc stream, Err: %s", err)
	// 		return err
	// 	}

	// 	msgs = append(msgs, *m)
	// }

	// fn := assembleEntries

	// // TODO: remove me when encoding re-organized
	// if msgs[0].Type == raftpb.MsgSnap {
	// 	fn = assembleSnap
	// }

	// msg, err := fn(msgs)
	// if err != nil {
	// 	return status.Error(codes.InvalidArgument, err.Error())
	// }
	s.processor.push(*m.Message)
	return &api.StreamResponse{}, nil
}

func (s *server) Join(ctx context.Context, m *api.Member) (*api.JoinResponse, error) {
	s.cfg.logger.Infof("raft: A new memebr requested to join Addr %s", m.Address)
	var (
		memb Member
		err  error
	)

	if mm, _ := s.cluster.GetMemebr(m.ID); mm == nil {
		memb, err = s.cluster.AddMember(ctx, m.Address)
	} else {
		err = s.cluster.UpdateMember(ctx, m.ID, m.Address)
		memb, _ = s.cluster.GetMemebr(m.ID)
	}

	if err != nil {
		return &api.JoinResponse{}, err
	}

	s.cfg.logger.Infof("raft: A new memebr joined %x", memb.ID())
	resp := &api.JoinResponse{}
	resp.ID = memb.ID()
	resp.Pool = s.pool.Snapshot()

	//
	for i, m := range resp.Pool {
		if m.Type == api.SelfMember {
			(&m).Type = api.RemoteMember
			resp.Pool[i] = m
			continue
		}

		if m.ID == memb.ID() {
			(&m).Type = api.SelfMember
			resp.Pool[i] = m
			continue
		}
	}

	return resp, nil
}
