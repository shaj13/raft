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
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/shaj13/raft"
	"github.com/shaj13/raft/raftlog"
	"github.com/shaj13/raft/transport"
	"github.com/shaj13/raft/transport/raftgrpc"
	"google.golang.org/grpc"
)

var (
	raftServer  *grpc.Server
	router      *mux.Router
	raftgroups  *groups
	httpAddr    string
	raftAddr    string
	joinAddr    string
	stateDIR    string
	initGroupID uint64
	nodeID      uint64
)

func init() {
	flag.StringVar(&raftAddr, "raft", "", "raft server address")
	flag.StringVar(&joinAddr, "join", "", "join cluster address")
	flag.StringVar(&httpAddr, "api", "", "api server address")
	flag.StringVar(&stateDIR, "state_dir", "", "raft state directory (WAL, Snapshots)")
	flag.Uint64Var(&initGroupID, "initial_group_id", 1, "initial group id this node will join")
	flag.Uint64Var(&nodeID, "id", 0, "raft node id")
	flag.Parse()
}

func init() {
	raftgrpc.Register(
		raftgrpc.WithDialOptions(grpc.WithInsecure()),
	)
	raftgroups = &groups{
		NodeGroup: raft.NewNodeGroup(transport.GRPC),
		nodes:     make(map[uint64]*localNode),
	}
	raftServer = grpc.NewServer()
	raftgrpc.RegisterHandler(raftServer, raftgroups.Handler())
	router = mux.NewRouter()
	router.HandleFunc("/{groupID}", http.HandlerFunc(save)).Methods("PUT", "POST")
	router.HandleFunc("/{groupID}/{key}", http.HandlerFunc(get)).Methods("GET")
	router.HandleFunc("/{groupID}/mgmt/nodes", http.HandlerFunc(nodes)).Methods("GET")
	router.HandleFunc("/{groupID}/mgmt/nodes/{id}", http.HandlerFunc(removeNode)).Methods("DELETE")
	router.HandleFunc("/mgmt/groups", http.HandlerFunc(newGroup)).Methods("PUT", "POST")
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

	go raftgroups.Start()

	go func() {
		err := http.ListenAndServe(httpAddr, router)
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	raftgroups.createAndStart(initGroupID, joinAddr)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c
	raftServer.GracefulStop()
	// TODO(Shaj13) stop all nodes.
}

func get(w http.ResponseWriter, r *http.Request) {
	lnode, err := getNodeFromgroup(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	key := mux.Vars(r)["key"]

	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()

	if err := lnode.raftnode.LinearizableRead(ctx); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	value := lnode.fsm.Read(key)
	w.Write([]byte(value))
	w.Write([]byte{'\n'})
}

func nodes(w http.ResponseWriter, r *http.Request) {
	lnode, err := getNodeFromgroup(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	raws := []raft.RawMember{}
	membs := lnode.raftnode.Members()
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
	lnode, err := getNodeFromgroup(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sid := mux.Vars(r)["id"]
	id, err := strconv.ParseUint(sid, 0, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()

	if err := lnode.raftnode.RemoveMember(ctx, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func newGroup(w http.ResponseWriter, r *http.Request) {
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := json.Unmarshal(buf, new(createGroup)); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()

	rep := replicate{
		CMD:  "group",
		Data: buf,
	}

	buf, err = json.Marshal(&rep)
	if err != nil {
		panic(err)
	}

	if err := raftgroups.getNode(initGroupID).raftnode.Replicate(ctx, buf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func save(w http.ResponseWriter, r *http.Request) {
	lnode, err := getNodeFromgroup(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

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

	rep := replicate{
		CMD:  "kv",
		Data: buf,
	}

	buf, err = json.Marshal(&rep)
	if err != nil {
		panic(err)
	}

	if err := lnode.raftnode.Replicate(ctx, buf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func getNodeFromgroup(r *http.Request) (*localNode, error) {
	sid := mux.Vars(r)["groupID"]
	gid, err := strconv.ParseUint(sid, 0, 64)
	if err != nil {
		return nil, err
	}

	lnode := raftgroups.getNode(gid)
	if lnode == nil {
		return nil, fmt.Errorf("group %s does not exist", sid)
	}

	return lnode, nil
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
	var rep replicate
	if err := json.Unmarshal(data, &rep); err != nil {
		log.Println("unable to Unmarshal replicate", err)
		return
	}

	switch rep.CMD {
	case "kv":
		var e entry
		if err := json.Unmarshal(rep.Data, &e); err != nil {
			log.Println("unable to Unmarshal entry", err)
			return
		}

		s.mu.Lock()
		defer s.mu.Unlock()
		s.kv[e.Key] = e.Value
	case "group":
		var c createGroup
		if err := json.Unmarshal(rep.Data, &c); err != nil {
			log.Println("unable to Unmarshal createGroup", err)
			return
		}

		myid := raftgroups.getNode(initGroupID).raftnode.Whoami()
		found := false
		for _, id := range c.IDs {
			if id == myid {
				found = true
				break
			}
		}

		if !found {
			raftlog.Info("ignore create group cmd; this node not part of it.")
			return
		}

		addr := c.JoinAddr
		if c.JoinAddr == raftAddr {
			addr = ""
		} else {
			// workaround to wait until first group node started before sending join.
			time.Sleep(time.Second * 5)
		}

		raftgroups.createAndStart(c.GroupID, addr)
	}
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

type groups struct {
	*raft.NodeGroup
	mu    sync.Mutex
	nodes map[uint64]*localNode
}

func (g *groups) createAndStart(groupID uint64, joinAddr string) {
	var fallback raft.StartOption
	lg := raftlog.New(0, fmt.Sprintf("[GROUP %d]", groupID), os.Stderr, io.Discard)
	logger := raft.WithLogger(lg)
	raw := raft.WithMembers(raft.RawMember{
		Address: raftAddr,
		ID:      nodeID,
	})
	state := raft.WithStateDIR(filepath.Join(stateDIR, fmt.Sprintf("%d", groupID)))
	if joinAddr != "" {
		fallback = raft.WithFallback(
			raft.WithJoin(joinAddr, time.Second),
			raft.WithRestart(),
		)
	} else {
		fallback = raft.WithFallback(
			raft.WithInitCluster(),
			raft.WithRestart(),
		)
	}

	fsm := newstateMachine()
	node := g.Create(groupID, fsm, state, logger)
	g.mu.Lock()

	g.nodes[groupID] = &localNode{
		fsm:      fsm,
		raftnode: node,
	}
	g.mu.Unlock()

	go func() {
		err := node.Start(fallback, raw)
		if err != nil && err != raft.ErrNodeStopped {
			log.Fatal(err)
		}
	}()
}

func (g *groups) getNode(groupID uint64) *localNode {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.nodes[groupID]
}

type localNode struct {
	raftnode *raft.Node
	fsm      *stateMachine
}

type replicate struct {
	CMD  string
	Data json.RawMessage
}

type createGroup struct {
	GroupID  uint64
	IDs      []uint64
	JoinAddr string
}

type entry struct {
	Key   string
	Value string
}
