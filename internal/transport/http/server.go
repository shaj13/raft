package http

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/shaj13/raftkit/internal/log"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/storage"
	"github.com/shaj13/raftkit/internal/transport"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

// NewServerFunc retur'ns func that create an http server.
func NewServerFunc(basePath string) transport.NewServer {
	return func(c context.Context, cfg transport.ServerConfig) (transport.Server, error) {
		s := &server{
			ctrl: cfg.Controller(),
			snap: cfg.Snapshotter(),
		}
		return mux(s, basePath), nil
	}
}

type server struct {
	ctrl transport.Controller
	snap storage.Snapshotter
}

func (s *server) message(w http.ResponseWriter, r *http.Request) (int, error) {
	msg := new(etcdraftpb.Message)
	if code, err := decode(r.Body, msg); err != nil {
		return code, err
	}

	if err := s.ctrl.Push(r.Context(), *msg); err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusNoContent, nil
}

func (s *server) snapshot(w http.ResponseWriter, r *http.Request) (int, error) {
	vals := r.Header.Values(snapshotHeader)
	if len(vals) < 3 {
		return http.StatusBadRequest, errors.New("raft/http: snapshot header missing")
	}

	snapname := vals[0]

	to, err := strconv.ParseUint(vals[1], 0, 64)
	if err != nil {
		return http.StatusBadRequest, err
	}

	from, err := strconv.ParseUint(vals[2], 0, 64)
	if err != nil {
		return http.StatusBadRequest, err
	}

	log.Debugf("raft.http: downloading sanpshot %s file", snapname)

	wr, peek, err := s.snap.Writer(snapname)
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
	m := new(raftpb.Member)
	if code, err := decode(r.Body, m); err != nil {
		return code, err
	}

	log.Debugf("raft.http: new member asks to join the cluster on address %s", m.Address)

	id, membs, err := s.ctrl.Join(r.Context(), m)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	w.Header().Set(memberIDHeader, strconv.FormatUint(id, 10))

	pool := &raftpb.Pool{
		Members: membs,
	}

	data, err := pool.Marshal()
	if err != nil {
		return http.StatusInternalServerError, err
	}

	w.Write(data)

	return http.StatusOK, nil
}

func (s *server) promoteMember(w http.ResponseWriter, r *http.Request) (int, error) {
	m := new(raftpb.Member)
	if code, err := decode(r.Body, m); err != nil {
		return code, err
	}

	if err := s.ctrl.PromoteMember(r.Context(), *m); err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusNoContent, nil
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
			log.Warnf("raft.http: handle %s: %v", r.URL.Path, err)
			http.Error(w, err.Error(), code)
			return
		}

		// Write calls WriteHeader(http.StatusOK)
		if code != http.StatusOK {
			w.WriteHeader(code)
		}
	})
}

func mux(s *server, basePath string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(join(basePath, messageURI), httpHandler(s.message))
	mux.HandleFunc(join(basePath, snapshotURI), httpHandler(s.snapshot))
	mux.HandleFunc(join(basePath, joinURI), httpHandler(s.join))
	mux.HandleFunc(join(basePath, promoteURI), httpHandler(s.promoteMember))
	return mux
}

func decode(r io.Reader, u pbutil.Unmarshaler) (int, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return http.StatusPreconditionFailed, err
	}

	if err := u.Unmarshal(data); err != nil {
		return http.StatusBadRequest, err
	}

	return 0, nil
}
