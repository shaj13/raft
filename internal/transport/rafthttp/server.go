package rafthttp

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/franklee0817/raft/internal/raftpb"
	"github.com/franklee0817/raft/internal/transport"
	"github.com/franklee0817/raft/raftlog"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

// NewHandlerFunc retur'ns func that create an http transport Handler.
func NewHandlerFunc(basePath string) transport.NewHandler {
	return func(cfg transport.Config) transport.Handler {
		s := &Handler{
			ctrl:   cfg.Controller(),
			logger: cfg.Logger(),
		}
		return mux(s, basePath)
	}
}

type Handler struct {
	ctrl   transport.Controller
	logger raftlog.Logger
}

// WrapHttpHandlerFunc 将handler打包成http的handler func格式
func (h *Handler) WrapHttpHandlerFunc(fn handlerFunc) http.HandlerFunc {
	return httpHandler(fn, h.logger)
}

func (h *Handler) Message(w http.ResponseWriter, r *http.Request) (int, error) {
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

func (h *Handler) Snapshot(w http.ResponseWriter, r *http.Request) (int, error) {
	gid := groupID(r)

	vals := r.Header.Values(snapshotHeader)
	if len(vals) < 2 {
		return http.StatusBadRequest, errors.New("raft/http: Snapshot header missing")
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

func (h *Handler) Join(w http.ResponseWriter, r *http.Request) (int, error) {
	gid := groupID(r)
	m := new(raftpb.Member)
	if code, err := decode(r.Body, m); err != nil {
		return code, err
	}

	h.logger.V(2).Infof("raft.http: new member asks to Join the cluster on address %s", m.Address)

	resp, err := h.ctrl.Join(r.Context(), gid, m)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	data, err := resp.Marshal()
	if err != nil {
		return http.StatusInternalServerError, err
	}

	_, _ = w.Write(data)

	return http.StatusOK, nil
}

func (h *Handler) PromoteMember(w http.ResponseWriter, r *http.Request) (int, error) {
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

func mux(s *Handler, basePath string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(join(basePath, messageURI), httpHandler(s.Message, s.logger))
	mux.HandleFunc(join(basePath, snapshotURI), httpHandler(s.Snapshot, s.logger))
	mux.HandleFunc(join(basePath, joinURI), httpHandler(s.Join, s.logger))
	mux.HandleFunc(join(basePath, promoteURI), httpHandler(s.PromoteMember, s.logger))
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
