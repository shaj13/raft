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
		kv: map[string]string{},
	}
}

type entry struct {
	Key   string
	Value string
}

type stateMachine struct {
	mu sync.Mutex
	kv map[string]string
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

func (s *stateMachine) Read(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.kv[key]
}
