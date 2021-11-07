package grpc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/shaj13/raftkit/internal/log"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/storage"
	"github.com/shaj13/raftkit/internal/transport"
	"github.com/shaj13/raftkit/internal/transport/grpc/pb"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

var errSnapHeader = errors.New("raft/grpc: snapshot header missing from grpc metadata")

// NewHandler return an GRPC transport Handler.
//
// NewHandler compatible with transport.NewHandler.
func NewHandler(cfg transport.HandlerConfig) transport.Handler {
	return &handler{
		ctrl: cfg.Controller(),
		snap: cfg.Snapshotter(),
	}
}

type handler struct {
	ctrl transport.Controller
	snap storage.Snapshotter
}

func (h *handler) PromoteMember(ctx context.Context, m *raftpb.Member) (*empty.Empty, error) {
	err := h.ctrl.PromoteMember(ctx, *m)
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

	m := new(etcdraftpb.Message)
	if err := m.Unmarshal(buf.Bytes()); err != nil {
		return err
	}

	if err := h.ctrl.Push(stream.Context(), *m); err != nil {
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

	log.Debugf("raft.grpc: downloading sanpshot %s file", snapname)

	w, peek, err := h.snap.Writer(snapname)
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

	if err := h.ctrl.Push(ctx, *m); err != nil {
		return err
	}

	return stream.SendAndClose(&emptypb.Empty{})
}

func (h *handler) Join(m *raftpb.Member, stream pb.Raft_JoinServer) (err error) {
	defer func() {
		if err != nil {
			log.Warnf("raft.grpc: handle join request: %v", err)
		}
	}()

	log.Debugf("raft.grpc: new member asks to join the cluster on address %s", m.Address)

	id, membs, err := h.ctrl.Join(stream.Context(), m)
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
