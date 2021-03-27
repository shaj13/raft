package grpc

import (
	"bytes"
	"context"
	"io"
	"strconv"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/log"
	"github.com/shaj13/raftkit/internal/net"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

// NewServer return an GRPC Server.
func NewServer(ctx context.Context, ctrl net.Controller, cfg interface{}) (net.Server, error) {
	return &server{ctrl: ctrl}, nil
}

type server struct {
	ctrl net.Controller
}

func (s *server) Message(stream api.Raft_MessageServer) (err error) {
	defer func() {
		if err != nil {
			log.Warnf("raft/net/grpc: Cannot handle incoming raft message, Err: %s", err)
		}
	}()

	buf := new(bytes.Buffer)
	dec := newDecoder(buf)

	for {
		c, err := stream.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		if err := dec.Decode(c); err != nil {
			return err
		}
	}

	m := new(raftpb.Message)
	if err := m.Unmarshal(buf.Bytes()); err != nil {
		return err
	}

	if err := s.ctrl.Push(stream.Context(), *m); err != nil {
		return err
	}

	return stream.SendAndClose(&emptypb.Empty{})
}

func (s *server) Snapshot(stream api.Raft_SnapshotServer) error {
	return nil
}

func (s *server) Join(m *api.Member, stream api.Raft_JoinServer) (err error) {
	defer func() {
		if err != nil {
			log.Warnf("raft/net/grpc: Cannot handle join request, Err: %s", err)
			return
		}
	}()

	log.Debugf("raft/net/grpc: A new member asks to join the cluster on address %s", m.Address)

	id, membs, err := s.ctrl.Join(stream.Context(), m)
	if err != nil {
		return err
	}

	md := metadata.Pairs(memberIDHeader, strconv.FormatUint(id, 10))
	if err := stream.SendHeader(md); err != nil {
		return err
	}

	for _, m := range membs {
		if err := stream.Send(&m); err != nil {
			return err
		}
	}

	return nil
}
