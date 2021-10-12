package grpc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"

	"github.com/shaj13/raftkit/internal/log"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/rpc"
	"github.com/shaj13/raftkit/internal/storage"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

var errSnapHeader = errors.New(
	"raft/net/grpc: snapshot header missing from grpc metadata",
)

// ServerConfig define common configuration used by the NewServer function.
type ServerConfig interface {
	DialConfig
	Controller() rpc.Controller
}

// NewServer return an GRPC Server.
//
// NewServer compatible with rpc.New.
func NewServer(ctx context.Context, cfg rpc.ServerConfig) (rpc.Server, error) {
	return &server{
		ctrl: cfg.(ServerConfig).Controller(),
		snap: cfg.(ServerConfig).Snapshotter(),
	}, nil
}

type server struct {
	ctrl rpc.Controller
	snap storage.Snapshotter
}

func (s *server) Message(stream raftpb.Raft_MessageServer) (err error) {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	dec := newDecoder(buf)

	defer func() {
		bufferPool.Put(buf)
		if err != nil {
			log.Warnf(
				"raft/net/grpc: Failed to handle incoming raft message, Err: %s",
				err,
			)
		}
	}()

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

	m := new(etcdraftpb.Message)
	if err := m.Unmarshal(buf.Bytes()); err != nil {
		return err
	}

	if err := s.ctrl.Push(stream.Context(), *m); err != nil {
		return err
	}

	return stream.SendAndClose(&emptypb.Empty{})
}

func (s *server) Snapshot(stream raftpb.Raft_SnapshotServer) (err error) {
	defer func() {
		if err != nil {
			log.Warnf(
				"raft/net/grpc: Failed to download the snapshot, Err: %s",
				err,
			)
		}
	}()

	ctx := stream.Context()
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return errSnapHeader
	}

	vals := md.Get(snapshotHeader)
	if len(vals) != 3 {
		return errSnapHeader
	}

	snapname := vals[0]

	to, err := strconv.ParseUint(vals[1], 0, 64)
	if err != nil {
		return err
	}

	from, err := strconv.ParseUint(vals[2], 0, 64)
	if err != nil {
		return err
	}

	log.Debugf(
		"raft/net/grpc: Start downloading the sanpshot %s file",
		snapname,
	)

	w, peek, err := s.snap.Writer(ctx, snapname)
	if err != nil {
		return err
	}

	defer w.Close()

	dec := newDecoder(w)
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

	snap, err := peek()
	if err != nil {
		return err
	}

	m := new(etcdraftpb.Message)
	m.Type = etcdraftpb.MsgSnap
	m.Snapshot = snap
	m.From = from
	m.To = to

	return s.ctrl.Push(ctx, *m)
}

func (s *server) Join(m *raftpb.Member, stream raftpb.Raft_JoinServer) (err error) {
	defer func() {
		if err != nil {
			log.Warnf("raft/net/grpc: Failed to handle join request, Err: %s", err)
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
