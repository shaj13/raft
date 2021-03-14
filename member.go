package raft

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/shaj13/raftkit/api"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type MemberID uint64

// String return's MemberID as a base 16 string.
func (m MemberID) String() string {
	return fmt.Sprintf("%x", uint64(m))
}

// MarshalJSON converts MemberID into a base 16 string.
func (m MemberID) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.String())
}

// UnmarshalJSON inflates MemberID from base16 string.
func (m *MemberID) UnmarshalJSON(b []byte) error {
	u, err := strconv.ParseUint(string(b), 16, 64)
	*m = MemberID(u)
	return err
}

// Size returns the size of this datum in protobuf. It is always 8 bytes.
func (m *MemberID) Size() int {
	return 8
}

// Marshal converts MemberID into a binary representation. Called by protobuf serialization.
func (m MemberID) Marshal() ([]byte, error) {
	b := make([]byte, m.Size())
	_, err := m.MarshalTo(b)
	return b, err
}

// MarshalTo converts MemberID into a binary representation. Called by protobuf serialization.
func (m *MemberID) MarshalTo(data []byte) (n int, err error) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(*m))
	return copy(data, b[:]), nil
}

// Unmarshal inflates MemberID from a binary representation. Called by protobuf serialization.
func (m *MemberID) Unmarshal(data []byte) error {
	if len(data) < 8 {
		return fmt.Errorf("buffer is too short")
	}
	*m = MemberID(binary.BigEndian.Uint64(data))
	return nil
}

type Member interface {
	ID() uint64
	Address() string
	Since() time.Time
	IsActive() bool
	update(string) error
	send(raftpb.Message) error
	kind() api.MemberType
	close()
}

// TODO: add a memeber factory
func isRemoved(m Member) bool {
	_, ok := m.(removed)
	return ok
}

type membersConstructors map[api.MemberType]memberConstructor
type memberConstructor func(ctx context.Context, id uint64, addr string) (Member, error)

func newSelf(_ context.Context, id uint64, addr string) (Member, error) {
	return self{
		id:   id,
		addr: addr,
	}, nil
}

func newRemoved(_ context.Context, id uint64, addr string) (Member, error) {
	return removed{
		id:   id,
		addr: addr,
	}, nil
}

func newRemote(ctx context.Context, id uint64, addr string) (Member, error) {
	r := registryFromCtx(ctx)
	ctx, cancel := context.WithCancel(ctx)
	mem := new(remote)
	mem.ctx = ctx
	mem.cancel = cancel
	mem.id = id
	mem.addr = addr
	mem.cfg = r.config
	mem.reportc = r.reportc
	mem.msgc = make(chan raftpb.Message)
	mem.done = make(chan struct{})
	// assuming member is active.
	mem.active = true
	mem.activeSince = time.Now()

	if err := mem.dial(addr); err != nil {
		return nil, err
	}

	go mem.run()

	return mem, nil
}

type factory struct {
	cfg          *config
	reportc      chan<- report
	constructors membersConstructors
	ctx          context.Context
	cancel       context.CancelFunc
}

func (f *factory) from(m api.Member) (Member, bool, error) {
	return f.create(m.ID, m.Address, m.Type)
}

func (f *factory) to(m Member) api.Member {
	return api.Member{
		ID:      m.ID(),
		Address: m.Address(),
		Type:    m.kind(),
	}
}

func (f *factory) cast(m Member, t api.MemberType) (Member, bool, error) {
	return f.create(m.ID(), m.Address(), t)
}

func (f *factory) create(id uint64, addr string, t api.MemberType) (Member, bool, error) {
	c, ok := f.constructors[t]
	if !ok {
		return nil, false, nil
	}

	mem, err := c(f.ctx, id, addr)
	return mem, true, err
}

// removed represents the remote removed cluster member.
type removed struct {
	id   uint64
	addr string
}

func (r removed) ID() uint64 {
	return r.id
}

func (r removed) Address() string {
	return r.addr
}

func (r removed) send(raftpb.Message) error {
	return errors.New("removed member")
}

func (r removed) kind() api.MemberType      { return api.RemovedMember }
func (r removed) close()                    {}
func (r removed) Since() (t time.Time)      { return }
func (r removed) IsActive() (ok bool)       { return }
func (r removed) update(string) (err error) { return }

// self represents current cluster member.
type self struct {
	id     uint64
	addr   string
	active time.Time
}

func (s self) ID() uint64 {
	return s.id
}

func (s self) Address() string {
	return s.addr
}

func (s self) Since() time.Time {
	return s.active
}

func (s self) IsActive() bool {
	return !s.active.IsZero()
}
func (s self) kind() api.MemberType            { return api.SelfMember }
func (s self) close()                          {}
func (s self) send(raftpb.Message) (err error) { return }
func (s self) update(string) (err error)       { return }

// remote represents the remote active cluster remote.
type remote struct {
	id          uint64
	cfg         *config
	ctx         context.Context
	cancel      context.CancelFunc
	msgc        chan raftpb.Message
	done        chan struct{}
	reportc     chan<- report
	mu          sync.Mutex // protects followings
	cc          *grpc.ClientConn
	active      bool
	addr        string
	activeSince time.Time
}

func (r *remote) kind() api.MemberType {
	return api.RemoteMember
}
func (r *remote) send(msg raftpb.Message) (err error) {
	r.mu.Lock()
	defer func() {
		if err != nil {
			r.active = false
			r.activeSince = time.Time{}
		}
	}()
	defer r.mu.Unlock()

	if err := r.ctx.Err(); err != nil {
		return err
	}

	select {
	case r.msgc <- msg:
	case <-r.ctx.Done():
		return r.ctx.Err()
	default:
		// TODO: report unrecahble
		return fmt.Errorf("Cluster member %d is unreachable", r.id)
	}

	return
}

func (r *remote) stream(ctx context.Context, msg raftpb.Message) (err error) {
	ctx, cancel := context.WithTimeout(ctx, r.cfg.streamTimeOut)
	defer cancel()
	defer func() {
		status, _ := status.FromError(err)

		switch {
		case err == nil && msg.Type == raftpb.MsgSnap:
			r.reportc <- report{
				signal: snapshotStatus,
				id:     r.id,
				status: raft.SnapshotFinish,
			}
		case err != nil && msg.Type == raftpb.MsgSnap:
			r.reportc <- report{
				signal: snapshotStatus,
				id:     r.id,
				status: raft.SnapshotFailure,
			}
		case status.Code() == codes.NotFound:
			r.reportc <- report{
				signal: shutdown,
				id:     r.id,
			}
		case err != nil:
			r.reportc <- report{
				signal: unreachable,
				id:     r.id,
			}
		}

	}()

	cc := r.conn()
	xx := api.MessageRequest{
		Message: &msg,
	}
	_, err = api.NewRaftClient(cc).StreamMessage(ctx, &xx)
	// for _, msg := range chunkedMsg(msg) {
	// 	err = stream.Send(&msg)
	// 	if err != nil {
	// 		_, _ = stream.CloseAndRecv()
	// 		return
	// 	}
	// }
	// _, err = msg.Marshal()
	// if err != nil {
	// 	panic(err)
	// }
	// // err = stream.Send(&msg)

	// _, err = stream.CloseAndRecv()
	return
}

func (r *remote) drain() error {
	ctx, cancel := context.WithTimeout(context.Background(), r.cfg.drainTimeOut)
	defer cancel()
	for {
		select {
		case msg, ok := <-r.msgc:
			if !ok {
				return nil
			}
			if err := r.stream(ctx, msg); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (r *remote) run() {
	for {
		defer r.cfg.logger.Debug(
			"Member %d context done, ctx.Err: %s",
			r.id,
			r.ctx.Err(),
		)

		// check ctx to exist immediately,
		// otherwise, will continue to send msg without drain timeouts.
		if r.ctx.Err() != nil {
			break
		}

		select {
		case msg := <-r.msgc:
			err := r.stream(r.ctx, msg)
			if err != nil {
				r.cfg.logger.Errorf(
					"An error occurred while streaming the message to member %d, Err: %s",
					r.id,
					err,
				)
			}
			r.setStatus(err != nil)
		case <-r.ctx.Done():
			break
		}

	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.active = false
	r.activeSince = time.Time{}
	close(r.msgc)

	// drain msgc and exit
	if err := r.drain(); err != nil {
		r.cfg.logger.Warningf(
			"An error occurred while draining the member %d message queue, Err: %s",
			r.id,
			err,
		)
	}

	close(r.done)
	return
}

func (r *remote) update(addr string) error {
	if r.addr == addr {
		return nil
	}
	r.conn().Close()
	return r.dial(addr)
}

func (r *remote) dial(addr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cc, err := grpc.Dial(addr, r.cfg.memberDialOptions...)
	if err != nil {
		return err
	}
	r.addr = addr
	r.cc = cc
	return nil
}

func (r *remote) Address() string {
	r.mu.Lock()
	addr := r.addr
	r.mu.Unlock()
	return addr
}

func (r *remote) conn() *grpc.ClientConn {
	r.mu.Lock()
	cc := r.cc
	r.mu.Unlock()
	return cc
}

func (r *remote) Since() time.Time {
	r.mu.Lock()
	acts := r.activeSince
	r.mu.Unlock()
	return acts
}

func (r *remote) setStatus(active bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case !r.active && active:
		r.activeSince = time.Now()
		r.active = true
	case r.active && !active:
		r.activeSince = time.Time{}
		r.active = false
	}
}

func (r *remote) IsActive() bool {
	r.mu.Lock()
	act := r.active
	r.mu.Unlock()
	return act
}

func (r *remote) ID() uint64 {
	return r.id
}

func (r *remote) close() {
	r.cancel()
	<-r.done
}

/// those need some invistageate
// TODO: self should reporte node removed
// TODO: self should honor reportc
// TODO: move factory to standalone file
