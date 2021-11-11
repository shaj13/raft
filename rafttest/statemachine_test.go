package rafttest_test

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"strings"
	"sync"
)

func newStateMachine() *stateMachine {
	return &stateMachine{
		kv: map[int]int{},
	}
}

func newBytesEntry(key, value int) []byte {
	ent := &entry{
		Key:   key,
		Value: value,
	}

	buf, err := json.Marshal(ent)
	if err != nil {
		panic(err)
	}

	return buf
}

type entry struct {
	Key   int
	Value int
}

type stateMachine struct {
	mu sync.Mutex
	kv map[int]int
}

func (s *stateMachine) Apply(data []byte) {
	var e entry
	if err := json.Unmarshal(data, &e); err != nil {
		log.Println("unable to Unmarshal entry", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.kv[e.Key] = e.Value
}

func (s *stateMachine) Snapshot() (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	buf, err := json.Marshal(&s.kv)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(strings.NewReader(string(buf))), nil
}

func (s *stateMachine) Restore(r io.ReadCloser) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	buf, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	err = json.Unmarshal(buf, &s.kv)
	if err != nil {
		return err
	}

	return r.Close()
}

func (s *stateMachine) Read(key int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.kv[key]
}
