package http

import (
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

// NewHandlerFunc retur'ns func that create an http transport handler.
func NewHandlerFunc(basePath string) transport.NewHandler {
	return func(cfg transport.HandlerConfig) transport.Handler {
		s := &handler{
			ctrl: cfg.Controller(),
			snap: cfg.Snapshotter(),
		}
		return mux(s, basePath)
	}
}

type handler struct {
	ctrl transport.Controller
	snap storage.Snapshotter
}

func (h *handler) message(w http.ResponseWriter, r *http.Request) (int, error) {
	msg := new(etcdraftpb.Message)
	if code, err := decode(r.Body, msg); err != nil {
		return code, err
	}

	if err := h.ctrl.Push(r.Context(), *msg); err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusNoContent, nil
}

func (h *handler) snapshot(w http.ResponseWriter, r *http.Request) (int, error) {
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

	log.Debugf("raft.http: downloading sanpshot file [term: %d, index: %d]", term, index)

	wr, err := h.snap.Writer(term, index)
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
	m := new(raftpb.Member)
	if code, err := decode(r.Body, m); err != nil {
		return code, err
	}

	log.Debugf("raft.http: new member asks to join the cluster on address %s", m.Address)

	resp, err := h.ctrl.Join(r.Context(), m)
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
	m := new(raftpb.Member)
	if code, err := decode(r.Body, m); err != nil {
		return code, err
	}

	if err := h.ctrl.PromoteMember(r.Context(), *m); err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusNoContent, nil
}

type handlerFunc func(w http.ResponseWriter, r *http.Request) (int, error)

func httpHandler(h handlerFunc) http.HandlerFunc {
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

func mux(s *handler, basePath string) http.Handler {
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
