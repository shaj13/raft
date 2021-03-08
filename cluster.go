package raft

import (
	"errors"
	"sync"
	"time"
)

type cluster struct {
	pool      *pool
	amu       sync.Mutex // protects active
	active    map[uint64]*remote
	rmu       sync.Mutex
	removed   map[uint64]struct{}
	processor *processor
}

// TODO: rename this to method to somthing meaningful.
func (c *cluster) StateSubscribe()  {}
func (c *cluster) MemberSubscribe() {}

func (c *cluster) StepDown()     {}
func (c *cluster) Join()         {}
func (c *cluster) Leave()        {}
func (c *cluster) AddMember()    {}
func (c *cluster) UpdateMember() {}
func (c *cluster) RemoveMember() {}

func (c *cluster) RemovedMembers() []Member {
	mems := []Member{}
	for _, m := range c.pool.members() {
		if isRemoved(m) {
			mems = append(mems)
		}
	}
	return mems
}

func (c *cluster) GetMemebr(id uint64) (Member, bool) {
	return c.pool.get(id)
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

func (c *cluster) Members() []Member {
	mems := []Member{}
	for _, m := range c.pool.members() {
		if !isRemoved(m) {
			mems = append(mems, m)
		}
	}
	return mems
}

func (c *cluster) CanMemberLeave(id uint64) bool {
	mid := c.Whoami()
	mems := c.Members()
	q := (len(mems)-1)/2 + 1
	n := 0
	if mid != id {
		n++
	}

	return n >= q
}

func (c *cluster) IsMemberRemoved(id uint64) bool {
	m, _ := c.GetMemebr(id)
	return isRemoved(m)
}

func (c *cluster) IsMember(id uint64) bool {
	_, ok := c.pool.get(id)
	return ok
}

func (c *cluster) Whoami() uint64 {
	return c.processor.status().ID
}

func (c *cluster) Leader() uint64 {
	return c.processor.status().Lead
}
