package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	raft "github.com/shaj13/raftkit"
	"github.com/shaj13/raftkit/transport"
	raftgrpc "github.com/shaj13/raftkit/transport/grpc"
	"google.golang.org/grpc"
	// "google.golang.org/grpc"
)

var (
	addr string
	join string
	api  string
	dir  string
)

func init() {
	raftgrpc.Register(
		raftgrpc.WithDialOptions(grpc.WithInsecure()),
	)
	flag.StringVar(&addr, "raft", "", "raft server addr")
	flag.StringVar(&join, "join", "", "join cluster addr")
	flag.StringVar(&api, "api", "", "api addr")
	flag.StringVar(&dir, "dir", "", "join cluster addr")
	flag.Parse()
}

func main() {
	var opt raft.StartOption
	var opt2 raft.StartOption
	if join != "" {
		opt = raft.WithFallback(
			raft.WithJoin(join, time.Second),
			raft.WithRestart(),
		)
		opt2 = raft.WithMembers(raft.RawMember{
			ID:      3,
			Address: addr,
			Type:    raft.VoterMember,
		})
	} else {
		opt = raft.WithFallback(
			raft.WithInitCluster(),
			raft.WithRestart(),
		)
		opt2 = raft.WithAddress(addr)
	}
	fsm := new(stateMachine)
	fsm.kv = map[string]string{}
	node := raft.New(fsm, transport.GRPC, raft.WithStateDIR(dir), raft.WithSnapshotInterval(2))
	go func() {
		if join != "" {
			return
		}
		time.Sleep(time.Second * 5)
		err := node.DemoteMember(context.Background(), 3)
		fmt.Println("####", err)
	}()

	go startRaftServer(node.Handler())
	go httpListen(fsm, node)
	if err := node.Start(opt, opt2); err != nil {
		panic(err)
	}
}

func startRaftServer(h transport.Handler) {
	s := grpc.NewServer()
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	raftgrpc.RegisterHandler(s, h)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

	// h := rafthttp.Handler(srv)
	// if err := http.ListenAndServe(strings.TrimPrefix(addr, "http://"), h); err != nil {
	// 	log.Fatalf("failed to serve: %v", err)
	// }
}

func httpListen(fsm *stateMachine, n *raft.Node) {
	err := http.ListenAndServe(api, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			err := n.LinearizableRead(r.Context())
			if err != nil {
				rw.WriteHeader(400)
				rw.Write([]byte(err.Error()))
			}

			path := strings.ReplaceAll(r.RequestURI, "/", "")
			val := fsm.Reead(path)
			rw.Write([]byte(val))
			return
		}
		buf, err := ioutil.ReadAll(r.Body)
		if err != nil {
			rw.WriteHeader(400)
			rw.Write([]byte(err.Error()))
		}

		err = n.Replicate(r.Context(), buf)
		if err != nil {
			rw.WriteHeader(400)
			rw.Write([]byte(err.Error()))
		}
	}))

	if err != nil {
		panic(err)
	}
}

var _ raft.StateMachine = new(stateMachine)

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

func (s *stateMachine) Reead(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.kv[key]
}

type entry struct {
	Key   string
	Value string
}
