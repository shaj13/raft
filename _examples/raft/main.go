package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/shaj13/raft"
	"github.com/shaj13/raft/transport"
	"github.com/shaj13/raft/transport/raftgrpc"
	"google.golang.org/grpc"
)

var (
	raftServer *grpc.Server
	opts       []raft.Option
	startOpts  []raft.StartOption
	router     *mux.Router
	node       *raft.Node
	fsm        *stateMachine
	httpAddr   string
	raftAddr   string
)

func init() {
	addr := flag.String("raft", "", "raft server address")
	join := flag.String("join", "", "join cluster address")
	api := flag.String("api", "", "api server address")
	state := flag.String("state_dir", "", "raft state directory (WAL, Snapshots)")
	flag.Parse()

	startOpts = append(startOpts, raft.WithAddress(*addr))
	opts = append(opts, raft.WithStateDIR(*state))
	if *join != "" {
		opt := raft.WithFallback(
			raft.WithJoin(*join, time.Second),
			raft.WithRestart(),
		)
		startOpts = append(startOpts, opt)
	} else {
		opt := raft.WithFallback(
			raft.WithInitCluster(),
			raft.WithRestart(),
		)
		startOpts = append(startOpts, opt)
	}

	httpAddr = *api
	raftAddr = *addr
}

func init() {
	raftgrpc.Register(
		raftgrpc.WithDialOptions(grpc.WithInsecure()),
	)
	fsm = newstateMachine()
	node = raft.NewNode(fsm, transport.GRPC, opts...)
	raftServer = grpc.NewServer()
	raftgrpc.RegisterHandler(raftServer, node.Handler())
	router = mux.NewRouter()
	router.HandleFunc("/", http.HandlerFunc(save)).Methods("PUT", "POST")
	router.HandleFunc("/{key}", http.HandlerFunc(get)).Methods("GET")
	router.HandleFunc("/mgmt/nodes", http.HandlerFunc(nodes)).Methods("GET")
	router.HandleFunc("/mgmt/nodes/{id}", http.HandlerFunc(removeNode)).Methods("DELETE")
}

func main() {
	go func() {
		lis, err := net.Listen("tcp", raftAddr)
		if err != nil {
			log.Fatal(err)
		}

		err = raftServer.Serve(lis)
		if err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		err := node.Start(startOpts...)
		if err != nil && err != raft.ErrNodeStopped {
			log.Fatal(err)
		}
	}()

	go func() {
		err := http.ListenAndServe(httpAddr, router)
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c
	raftServer.GracefulStop()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	_ = node.Shutdown(ctx)
}

func get(w http.ResponseWriter, r *http.Request) {
	key := mux.Vars(r)["key"]

	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()

	if err := node.LinearizableRead(ctx); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	value := fsm.Read(key)
	w.Write([]byte(value))
	w.Write([]byte{'\n'})
}

func nodes(w http.ResponseWriter, r *http.Request) {
	raws := []raft.RawMember{}
	membs := node.Members()
	for _, m := range membs {
		raws = append(raws, m.Raw())
	}

	buf, err := json.Marshal(raws)
	if err != nil {
		panic(err)
	}

	w.Write(buf)
	w.Write([]byte{'\n'})
}

func removeNode(w http.ResponseWriter, r *http.Request) {
	sid := mux.Vars(r)["id"]
	id, err := strconv.ParseUint(sid, 0, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()

	if err := node.RemoveMember(ctx, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func save(w http.ResponseWriter, r *http.Request) {
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := json.Unmarshal(buf, new(entry)); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()

	if err := node.Replicate(ctx, buf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func newstateMachine() *stateMachine {
	return &stateMachine{
		kv: make(map[string]string),
	}
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

type entry struct {
	Key   string
	Value string
}
