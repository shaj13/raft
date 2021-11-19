package raftgrpc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/shaj13/raft/internal/log"
	"github.com/shaj13/raft/internal/raftpb"
	"github.com/shaj13/raft/internal/transport"
	"github.com/shaj13/raft/internal/transport/raftgrpc/pb"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

var errSnapHeader = errors.New("raft/grpc: snapshot header missing from grpc metadata")

// NewHandler return an GRPC transport Handler.
//
// NewHandler compatible with transport.NewHandler.
func NewHandler(cfg transport.Config) transport.Handler {
	return &handler{
		ctrl: cfg.Controller(),
	}
}

type handler struct {
	ctrl transport.Controller
}

func (h *handler) PromoteMember(ctx context.Context, m *raftpb.Member) (*empty.Empty, error) {
	gid := groupID(ctx)
	err := h.ctrl.PromoteMember(ctx, gid, *m)
	return &emptypb.Empty{}, err
}

func (h *handler) Message(stream pb.Raft_MessageServer) (err error) {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	dec := newDecoder(buf)

	defer func() {
		bufferPool.Put(buf)
		if err != nil {
			log.Warnf("raft.grpc: handle incoming message: %v", err)
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

	ctx := stream.Context()
	gid := groupID(ctx)
	m := new(etcdraftpb.Message)
	if err := m.Unmarshal(buf.Bytes()); err != nil {
		return err
	}

	if err := h.ctrl.Push(ctx, gid, *m); err != nil {
		return err
	}

	return stream.SendAndClose(&emptypb.Empty{})
}

func (h *handler) Snapshot(stream pb.Raft_SnapshotServer) (err error) {
	defer func() {
		if err != nil {
			log.Warnf("raft.grpc: downloading snapshot: %v", err)
		}
	}()

	ctx := stream.Context()
	gid := groupID(ctx)
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return errSnapHeader
	}

	vals := md.Get(snapshotHeader)
	if len(vals) != 2 {
		return errSnapHeader
	}

	term, err := strconv.ParseUint(vals[0], 0, 64)
	if err != nil {
		return err
	}

	index, err := strconv.ParseUint(vals[1], 0, 64)
	if err != nil {
		return err
	}

	log.Debugf("raft.grpc: downloading sanpshot file [term: %d, index: %d]", term, index)

	w, err := h.ctrl.SnapshotWriter(gid, term, index)
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

	return stream.SendAndClose(&emptypb.Empty{})
}

func (h *handler) Join(ctx context.Context, m *raftpb.Member) (resp *raftpb.JoinResponse, err error) {
	defer func() {
		if err != nil {
			log.Warnf("raft.grpc: handle join request: %v", err)
		}
	}()

	gid := groupID(ctx)
	log.Debugf("raft.grpc: new member asks to join the cluster on address %s", m.Address)

	return h.ctrl.Join(ctx, gid, m)
}

func groupID(ctx context.Context) uint64 {
	md, _ := metadata.FromIncomingContext(ctx)
	vals := md.Get(groupIDHeader)
	if len(vals) == 0 {
		return 0
	}
	gid, _ := strconv.ParseUint(vals[0], 0, 64)
	return gid
}
