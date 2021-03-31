package raft

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/daemon"
	"github.com/shaj13/raftkit/internal/membership"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

type Cluster interface {
	Join(ctx context.Context, join, addr string) error
	Leave(ctx context.Context) error
	UpdateMember(ctx context.Context, id uint64, addr string) error
	RemoveMember(ctx context.Context, id uint64) error
	AddMember(ctx context.Context, addr string) (Member, error)
	GetMemebr(id uint64) (Member, bool)
	Members() []Member
	RemovedMembers() []Member
	AddressInUse(addr string) uint64
	LongestActive() (Member, error)
	IsAvailable() bool
	IsMemberRemoved(id uint64) bool
	IsMember(id uint64) bool
	Whoami() uint64
	Leader() uint64
}

type cluster struct {
	pool   *membership.Pool
	daemon daemon.Daemon
}

// TODO: rename this to method to somthing meaningful.
func (c *cluster) StateSubscribe()  {}
func (c *cluster) MemberSubscribe() {}

func (c *cluster) StepDown() {}
func (c *cluster) Join(ctx context.Context, join, addr string) error {
	return c.daemon.Start(ctx, join, addr)
}

func (c *cluster) Leave(ctx context.Context) error {
	return c.RemoveMember(
		ctx,
		c.Whoami(),
	)
}

func (c *cluster) UpdateMember(ctx context.Context, id uint64, addr string) error {
	if c.Whoami() == 0 {
		return fmt.Errorf("raft: node is not yet part of a raft cluster")
	}

	if !c.IsAvailable() {
		return fmt.Errorf("raft: quorum lost and the cluster unavailable, no new logs can be committed")
	}

	if !c.IsMember(id) {
		return fmt.Errorf("raft: unknown member %x", id)
	}

	if c.IsMemberRemoved(id) {
		return fmt.Errorf("raft: member %x already removed", id)
	}

	if id := c.AddressInUse(addr); id > 0 {
		return fmt.Errorf("raft: address used by member %x", id)
	}

	m := &api.Member{
		ID:   id,
		Type: api.RemovedMember,
	}

	return c.daemon.ProposeConfChange(ctx, m, raftpb.ConfChangeUpdateNode)
}

func (c *cluster) RemoveMember(ctx context.Context, id uint64) error {
	if c.Whoami() == 0 {
		return fmt.Errorf("raft: node is not yet part of a raft cluster")
	}

	if !c.IsAvailable() {
		return fmt.Errorf("raft: quorum lost and the cluster unavailable, no new logs can be committed")
	}

	if !c.IsMember(id) {
		return fmt.Errorf("raft: unknown member %x", id)
	}

	if c.IsMemberRemoved(id) {
		return fmt.Errorf("raft: member %x already removed", id)
	}

	m := &api.Member{
		ID:   id,
		Type: api.RemovedMember,
	}

	return c.daemon.ProposeConfChange(ctx, m, raftpb.ConfChangeRemoveNode)
}

func (c *cluster) AddMember(ctx context.Context, addr string) (Member, error) {
	if c.Whoami() == 0 {
		return nil, fmt.Errorf("raft: node is not yet part of a raft cluster")
	}

	if !c.IsAvailable() {
		return nil, fmt.Errorf("raft: quorum lost and the cluster unavailable, no new logs can be committed")
	}

	if id := c.AddressInUse(addr); id > 0 {
		return nil, fmt.Errorf("raft: address used by member %x", id)
	}

	m := &api.Member{
		ID:      c.pool.NextID(),
		Address: addr,
		Type:    api.RemoteMember,
	}

	err := c.daemon.ProposeConfChange(ctx, m, raftpb.ConfChangeAddNode)
	if err != nil {
		return nil, err
	}

	memb, _ := c.pool.Get(m.ID)
	return memb, nil
}

func (c *cluster) GetMemebr(id uint64) (Member, bool) {
	return c.pool.Get(id)
}

func (c *cluster) members(cond func(m Member) bool) []Member {
	mems := []Member{}
	for _, m := range c.pool.Members() {
		if cond(m) {
			mems = append(mems, m)
		}
	}
	return mems
}

func (c *cluster) Members() []Member {
	cond := func(m Member) bool {
		return m.Type() != api.RemovedMember
	}
	return c.members(cond)
}

func (c *cluster) RemovedMembers() []Member {
	cond := func(m Member) bool {
		return m.Type() == api.RemovedMember
	}
	return c.members(cond)
}

func (c *cluster) AddressInUse(addr string) uint64 {
	for _, m := range c.Members() {
		if m.Address() == addr {
			return m.ID()
		}
	}
	return 0
}

func (c *cluster) LongestActive() (Member, error) {
	var (
		longest     Member
		longestTime time.Time
	)

	for _, m := range c.Members() {
		since := m.Since()
		if since.IsZero() {
			continue
		}
		if longest == nil {
			longest = m
			continue
		}
		if since.Before(longestTime) {
			longest = m
			longestTime = since
		}
	}

	if longest == nil {
		return nil, errors.New("raft: failed to find longest active peer")
	}

	return longest, nil
}

func (c *cluster) IsAvailable() bool {
	cond := func(m Member) bool {
		return m.IsActive()
	}

	q := (len(c.Members()))/2 + 1
	n := len(c.members(cond))

	return n >= q
}

func (c *cluster) IsMemberRemoved(id uint64) bool {
	m, _ := c.GetMemebr(id)
	return m.Type() == api.RemovedMember
}

func (c *cluster) IsMember(id uint64) bool {
	_, ok := c.pool.Get(id)
	return ok
}

func (c *cluster) Whoami() uint64 {
	s, _ := c.daemon.Status()
	return s.ID
}

func (c *cluster) Leader() uint64 {
	s, _ := c.daemon.Status()
	return s.Lead
}
