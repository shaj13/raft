package raft

import (
	"context"
	"math/rand"

	"github.com/shaj13/raftkit/api"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

type server struct {
	api.UnimplementedRaftServer
	cfg       *config
	processor *processor
	pool      *pool
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
	for {
		m.ID = uint64(rand.Int63()) + 1
		if _, ok := s.pool.get(m.ID); !ok {
			break
		}
	}

	// TODO: validate address etc
	s.processor.proposeConfChange(ctx, m, raftpb.ConfChangeAddNode)

	s.cfg.logger.Infof("raft: A new memebr joined %x", m.ID)
	resp := new(api.JoinResponse)
	resp.ID = m.ID
	resp.Pool = s.pool.snapshot()
	return resp, nil
}
