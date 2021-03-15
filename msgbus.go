package raft

import (
	"sync"
)

type msgbus struct {
	mu    sync.Mutex
	chans map[uint64][]chan interface{}
}

func (m *msgbus) subscribe(id uint64) <-chan interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan interface{})
	chans, ok := m.chans[id]
	if !ok {
		chans = make([]chan interface{}, 0)
	}
	chans = append(chans, ch)
	m.chans[id] = chans

	return ch
}

func (m *msgbus) trigger(id uint64, v interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	chans := m.chans[id]
	delete(m.chans, id)
	go m.process(chans, v)
}

func (m *msgbus) cancel(id uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	chans := m.chans[id]
	delete(m.chans, id)
	for _, ch := range chans {
		close(ch)
	}
}

func (m *msgbus) broadcast(id uint64, v interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	chans := m.chans[id]
	go m.process(chans, v)
}

func (m *msgbus) process(chans []chan interface{}, v interface{}) {
	for _, ch := range chans {
		ch <- v
	}
}

func (m *msgbus) clsoe() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, chans := range m.chans {
		delete(m.chans, id)
		for _, ch := range chans {
			close(ch)
		}
	}
}
