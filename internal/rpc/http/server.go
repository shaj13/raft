package http

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/shaj13/raftkit/internal/log"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/rpc"
	"github.com/shaj13/raftkit/internal/storage"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

// var errSnapHeader = errors.New("raft/rpc/http: snapshot header missing")

func NewServerFunc(basePath string) rpc.NewServer {
	return func(c context.Context, cfg rpc.ServerConfig) (rpc.Server, error) {
		s := &server{
			ctrl: cfg.(ServerConfig).Controller(),
			snap: cfg.(ServerConfig).Snapshotter(),
		}

		mux := http.NewServeMux()
		mux.HandleFunc(join(basePath, messageURI), httpHandler(s.message))
		mux.HandleFunc(join(basePath, snapshotURI), httpHandler(s.snapshot))
		mux.HandleFunc(join(basePath, joinURI), httpHandler(s.join))
		return mux, nil
	}
}

// NewServer return an http Server.
//
// NewServer compatible with rpc.New.
// func NewServer(ctx context.Context, cfg rpc.ServerConfig) (rpc.Server, error) {

// }

type server struct {
	ctrl rpc.Controller
	snap storage.Snapshotter
}

func (s *server) message(w http.ResponseWriter, r *http.Request) (int, error) {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	msg := new(etcdraftpb.Message)
	if err := msg.Unmarshal(data); err != nil {
		return http.StatusBadRequest, err
	}

	if err := s.ctrl.Push(r.Context(), *msg); err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusNoContent, err
}

func (s *server) snapshot(w http.ResponseWriter, r *http.Request) (int, error) {
	vals := r.Header.Values(snapshotHeader)

	snapname := vals[0]

	to, err := strconv.ParseUint(vals[1], 0, 64)
	if err != nil {
		return http.StatusBadRequest, err
	}

	from, err := strconv.ParseUint(vals[2], 0, 64)
	if err != nil {
		return http.StatusBadRequest, err
	}

	log.Debugf(
		"raft.rpc.http: start downloading sanpshot %s file",
		snapname,
	)

	wr, peek, err := s.snap.Writer(r.Context(), snapname)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	defer wr.Close()

	_, err = io.Copy(wr, r.Body)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	snap, err := peek()
	if err != nil {
		return http.StatusInternalServerError, err
	}

	m := new(etcdraftpb.Message)
	m.Type = etcdraftpb.MsgSnap
	m.Snapshot = snap
	m.From = from
	m.To = to

	if err := s.ctrl.Push(r.Context(), *m); err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusNoContent, nil
}

func (s *server) join(w http.ResponseWriter, r *http.Request) (int, error) {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	m := new(raftpb.Member)
	if err := m.Unmarshal(data); err != nil {
		return http.StatusBadRequest, err
	}

	id, membs, err := s.ctrl.Join(r.Context(), m)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	w.Header().Set(memberIDHeader, strconv.FormatUint(id, 10))

	pool := &raftpb.Pool{
		Members: membs,
	}

	data, err = pool.Marshal()
	if err != nil {
		return http.StatusInternalServerError, err
	}

	w.Write(data)

	return http.StatusOK, nil
}

type handler func(w http.ResponseWriter, r *http.Request) (int, error)

func httpHandler(h handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			code := http.StatusMethodNotAllowed
			http.Error(w, http.StatusText(code), code)
			return
		}

		code, err := h(w, r)
		if err != nil {
			http.Error(w, err.Error(), code)
			return
		}

		w.WriteHeader(code)
	})
}
