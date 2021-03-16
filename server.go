package raft

import (
	"context"

	"github.com/shaj13/raftkit/api"
)

type server struct {
	api.UnimplementedRaftServer
	cfg       *config
	processor *processor
	pool      *pool
	cluster   *cluster
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
	memb, err := s.cluster.AddMember(ctx, m.Address)
	if err != nil {
		return &api.JoinResponse{}, err
	}

	s.cfg.logger.Infof("raft: A new memebr joined %x", memb.ID())
	resp := new(api.JoinResponse)
	resp.ID = memb.ID()
	resp.Pool = s.pool.snapshot()
	return resp, nil
}
