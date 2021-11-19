package msgbus

import (
	"math/rand"
	"sync"
	"time"

	"github.com/shaj13/raft/internal/atomic"
	"go.etcd.io/etcd/pkg/v3/idutil"
)

// New create a new msgbus.
func New() *MsgBus {
	id := rand.Int63() + 1
	idgen := idutil.NewGenerator(uint16(id), time.Now())
	return &MsgBus{
		subid:  idgen,
		events: map[uint64]map[uint64]*Subscription{},
	}
}

// MsgBus is a 1-to-n event distribution.
type MsgBus struct {
	subid  *idutil.Generator // generate subscription id
	mu     sync.Mutex
	events map[uint64]map[uint64]*Subscription
}

// Subscribe creates an async subscription for event.
func (m *MsgBus) Subscribe(id uint64) *Subscription {
	return m.subscribe(id, 1, false)
}

// SubscribeBuffered creates an async buffered subscription for event.
func (m *MsgBus) SubscribeBuffered(id uint64, n int) *Subscription {
	return m.subscribe(id, n, false)
}

// Subscribe creates an async one time subscription for event.
func (m *MsgBus) SubscribeOnce(id uint64) *Subscription {
	return m.subscribe(id, 1, true)
}

// BroadcastToAll sends v to all events subscribers.
func (m *MsgBus) BroadcastToAll(v interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id := range m.events {
		m.broadcast(id, v)
	}
}

// Broadcast sends event to subscribers.
func (m *MsgBus) Broadcast(id uint64, v interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.broadcast(id, v)
}

// Close msgbus and remove all subscription.
func (m *MsgBus) Clsoe() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, subs := range m.events {
		for _, s := range subs {
			s.close()
		}
		delete(m.events, id)
	}

	return nil
}

func (m *MsgBus) broadcast(id uint64, v interface{}) {
	subs, ok := m.events[id]
	if !ok {
		return
	}

	sv := make([]*Subscription, len(subs)) // subscription value
	index := 0

	for _, v := range subs {
		sv[index] = v
		index++
	}

	go func(subs []*Subscription, v interface{}) {
		for _, s := range subs {
			s.publish(v)
		}
	}(sv, v)
}

func (m *MsgBus) subscribe(id uint64, n int, once bool) *Subscription {
	m.mu.Lock()
	defer m.mu.Unlock()

	s := &Subscription{
		id:     m.subid.Next(),
		closed: atomic.NewBool(),
		eid:    id,
		once:   once,
		c:      make(chan interface{}, n),
		delete: m.delete,
	}

	subs, ok := m.events[s.eid]
	if !ok {
		subs = make(map[uint64]*Subscription, 1)
	}

	subs[s.id] = s
	m.events[s.eid] = subs
	return s
}

func (m *MsgBus) delete(id, sid uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	subs, ok := m.events[id]
	if !ok {
		return
	}

	if len(subs) == 1 {
		delete(m.events, id)
	}

	delete(subs, sid)
}

// Subscription represents interest in a given event.
type Subscription struct {
	id     uint64 // subscription id
	eid    uint64 // event id
	once   bool
	closed *atomic.Bool
	c      chan interface{}
	delete func(id, sid uint64)
}

// Chan returns subscription channel.
func (s *Subscription) Chan() <-chan interface{} {
	return s.c
}

// Unsubscribe will remove interest in the given event.
func (s *Subscription) Unsubscribe() {
	s.close()
	s.delete(s.eid, s.id)
}

func (s *Subscription) publish(v interface{}) {
	if s.closed.True() {
		return
	}

	s.c <- v

	if s.once {
		s.Unsubscribe()
	}
}

func (s *Subscription) close() {
	if s.closed.True() {
		return
	}
	s.closed.Set()
	close(s.c)
}
