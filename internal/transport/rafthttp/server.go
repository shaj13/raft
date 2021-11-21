package rafthttp

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/shaj13/raft/internal/raftpb"
	"github.com/shaj13/raft/internal/transport"
	"github.com/shaj13/raft/raftlog"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

// NewHandlerFunc retur'ns func that create an http transport handler.
func NewHandlerFunc(basePath string) transport.NewHandler {
	return func(cfg transport.Config) transport.Handler {
		s := &handler{
			ctrl: cfg.Controller(),
		}
		return mux(s, basePath)
	}
}

type handler struct {
	ctrl   transport.Controller
	logger raftlog.Logger
}

func (h *handler) message(w http.ResponseWriter, r *http.Request) (int, error) {
	gid := groupID(r)
	msg := new(etcdraftpb.Message)
	if code, err := decode(r.Body, msg); err != nil {
		return code, err
	}

	if err := h.ctrl.Push(r.Context(), gid, *msg); err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusNoContent, nil
}

func (h *handler) snapshot(w http.ResponseWriter, r *http.Request) (int, error) {
	gid := groupID(r)

	vals := r.Header.Values(snapshotHeader)
	if len(vals) < 2 {
		return http.StatusBadRequest, errors.New("raft/http: snapshot header missing")
	}

	term, err := strconv.ParseUint(vals[0], 0, 64)
	if err != nil {
		return http.StatusBadRequest, err
	}

	index, err := strconv.ParseUint(vals[1], 0, 64)
	if err != nil {
		return http.StatusBadRequest, err
	}

	h.logger.V(2).Infof("raft.http: downloading sanpshot file [term: %d, index: %d]", term, index)

	wr, err := h.ctrl.SnapshotWriter(gid, term, index)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	defer wr.Close()

	_, err = io.Copy(wr, r.Body)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusNoContent, nil
}

func (h *handler) join(w http.ResponseWriter, r *http.Request) (int, error) {
	gid := groupID(r)
	m := new(raftpb.Member)
	if code, err := decode(r.Body, m); err != nil {
		return code, err
	}

	h.logger.V(2).Infof("raft.http: new member asks to join the cluster on address %s", m.Address)

	resp, err := h.ctrl.Join(r.Context(), gid, m)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	data, err := resp.Marshal()
	if err != nil {
		return http.StatusInternalServerError, err
	}

	w.Write(data)

	return http.StatusOK, nil
}

func (h *handler) promoteMember(w http.ResponseWriter, r *http.Request) (int, error) {
	gid := groupID(r)
	m := new(raftpb.Member)
	if code, err := decode(r.Body, m); err != nil {
		return code, err
	}

	if err := h.ctrl.PromoteMember(r.Context(), gid, *m); err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusNoContent, nil
}

type handlerFunc func(w http.ResponseWriter, r *http.Request) (int, error)

func httpHandler(h handlerFunc, logger raftlog.Logger) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			code := http.StatusMethodNotAllowed
			http.Error(w, http.StatusText(code), code)
			return
		}

		code, err := h(w, r)
		if err != nil {
			logger.Infof("raft.http: handle %s: %v", r.URL.Path, err)
			http.Error(w, err.Error(), code)
			return
		}

		// Write calls WriteHeader(http.StatusOK)
		if code != http.StatusOK {
			w.WriteHeader(code)
		}
	})
}

func mux(s *handler, basePath string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(join(basePath, messageURI), httpHandler(s.message, s.logger))
	mux.HandleFunc(join(basePath, snapshotURI), httpHandler(s.snapshot, s.logger))
	mux.HandleFunc(join(basePath, joinURI), httpHandler(s.join, s.logger))
	mux.HandleFunc(join(basePath, promoteURI), httpHandler(s.promoteMember, s.logger))
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

func groupID(r *http.Request) uint64 {
	str := r.Header.Get(groupIDHeader)
	gid, _ := strconv.ParseUint(str, 0, 64)
	return gid
}
