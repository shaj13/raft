package rafthttp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/shaj13/raft/internal/raftpb"
	"github.com/shaj13/raft/internal/transport"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

const (
	snapshotHeader = "X-Raft-Snapshot"
	groupIDHeader  = "X-Raft-Group-ID"
	messageURI     = "/message"
	snapshotURI    = "/snapshot"
	joinURI        = "/join"
	promoteURI     = "/promote"
)

var bufferPool = sync.Pool{
	New: func() interface{} { return new(bytes.Buffer) },
}

// Dialer return's grpc dialer.
func Dialer(tr func(context.Context) http.RoundTripper, basePath string) transport.Dialer {
	return func(cfg transport.Config) transport.Dial {
		return func(ctx context.Context, addr string) (transport.Client, error) {
			if !strings.HasPrefix("http://", addr) && !strings.HasPrefix("https://", addr) {
				addr = "http://" + addr
			}
			return &client{
				transport: tr,
				gid:       cfg.GroupID(),
				url:       join(addr, basePath),
				ctrl:      cfg.Controller(),
			}, nil
		}
	}
}

type client struct {
	transport func(context.Context) http.RoundTripper
	gid       uint64
	url       string
	ctrl      transport.Controller
}

func (c *client) Close() (err error) { return }

func (c *client) Message(ctx context.Context, m etcdraftpb.Message) error {
	fn := c.message
	if m.Type == etcdraftpb.MsgSnap {
		fn = c.snapshot
	}

	err := fn(ctx, m)
	if err == io.EOF {
		return nil
	}

	return err
}

func (c *client) Join(ctx context.Context, m raftpb.Member) (*raftpb.JoinResponse, error) {
	resp := new(raftpb.JoinResponse)
	// nolint:bodyclose
	_, err := c.requestProto(ctx, joinURI, &m, resp)
	return resp, err
}

func (c *client) PromoteMember(ctx context.Context, msg raftpb.Member) error {
	// nolint:bodyclose
	_, err := c.requestProto(ctx, promoteURI, &msg, nil)
	return err
}

func (c *client) message(ctx context.Context, msg etcdraftpb.Message) error {
	// nolint:bodyclose
	_, err := c.requestProto(ctx, messageURI, &msg, nil)
	return err
}

func (c *client) snapshot(ctx context.Context, msg etcdraftpb.Message) error {
	meta := msg.Snapshot.Metadata
	r, err := c.ctrl.SnapshotReader(c.gid, meta.Term, meta.Index)
	if err != nil {
		return err
	}

	defer r.Close()

	u := join(c.url, snapshotURI)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, r)
	if err != nil {
		return err
	}

	req.Header.Add(snapshotHeader, strconv.FormatUint(meta.Term, 10))
	req.Header.Add(snapshotHeader, strconv.FormatUint(meta.Index, 10))

	// nolint:bodyclose
	if _, err := c.roundTrip(ctx, req, nil); err != nil {
		return err
	}

	return c.message(ctx, msg)
}

func (c *client) requestProto(
	ctx context.Context,
	uri string,
	in pbutil.Marshaler,
	out pbutil.Unmarshaler,
) (*http.Response, error) {

	data, err := in.Marshal()
	if err != nil {
		return nil, err
	}

	b := bufferPool.Get().(*bytes.Buffer)
	b.Reset()
	_, _ = b.Write(data)
	defer bufferPool.Put(b)

	u := join(c.url, uri)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, b)
	if err != nil {
		return nil, err
	}

	return c.roundTrip(ctx, req, out)
}

func (c *client) roundTrip(ctx context.Context, req *http.Request, out pbutil.Unmarshaler) (*http.Response, error) {
	gid := strconv.FormatUint(c.gid, 10)
	req.Header.Set(groupIDHeader, gid)

	res, err := c.transport(ctx).RoundTrip(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	// return if rpc does not return response.
	if res.StatusCode == http.StatusNoContent && out == nil {
		return res, nil
	}

	b := bufferPool.Get().(*bytes.Buffer)
	b.Reset()
	defer bufferPool.Put(b)

	_, err = io.Copy(b, res.Body)
	if err != nil {
		return nil, fmt.Errorf("raft/http: reading response body: %v", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("raft/http: server returned: %v : %v", res.Status, b.String())
	}

	err = out.Unmarshal(b.Bytes())
	if err != nil {
		return nil, fmt.Errorf("raft/http: decoding response body: %v", err)
	}

	return res, nil
}

func join(u, p string) string {
	u = strings.TrimSuffix(u, "/")
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")
	return u + "/" + p
}
