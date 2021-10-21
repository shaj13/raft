package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/rpc"
	"github.com/shaj13/raftkit/internal/storage"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

const (
	snapshotHeader = "X-Raft-Snapshot"
	memberIDHeader = "X-Raft-Member-ID"
	messageURI     = "/message"
	snapshotURI    = "/snapshot"
	joinURI        = "/join"
)

var bufferPool = sync.Pool{
	New: func() interface{} { return new(bytes.Buffer) },
}

// Dialer return's grpc dialer.
func Dialer(tr func(context.Context) http.RoundTripper, basePath string) rpc.Dialer {
	return func(_ context.Context, dc rpc.DialerConfig) rpc.Dial {
		return func(ctx context.Context, addr string) (rpc.Client, error) {
			return &client{
				transport: tr,
				url:       join(addr, basePath),
				shotter:   dc.Snapshotter(),
			}, nil
		}
	}
}

type client struct {
	transport func(context.Context) http.RoundTripper
	url       string
	shotter   storage.Snapshotter
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

func (c *client) Join(ctx context.Context, m raftpb.Member) (uint64, []raftpb.Member, error) {
	pool := new(raftpb.Pool)
	h, err := c.requestProto(ctx, joinURI, &m, pool)
	if err != nil {
		return 0, nil, err
	}

	id, err := strconv.ParseUint(h.Get(memberIDHeader), 0, 64)
	if err != nil {
		return 0, nil, fmt.Errorf("raft/http: parse member id: %v", err)
	}

	return id, pool.Members, nil
}

func (c *client) message(ctx context.Context, m etcdraftpb.Message) (err error) {
	_, err = c.requestProto(ctx, messageURI, &m, nil)
	return
}

func (c *client) snapshot(ctx context.Context, m etcdraftpb.Message) (err error) {
	name, r, err := c.shotter.Reader(ctx, m.Snapshot)
	if err != nil {
		return err
	}

	defer r.Close()

	h := make(http.Header, 1)
	h.Add(snapshotHeader, name)
	h.Add(snapshotHeader, strconv.FormatUint(m.To, 10))
	h.Add(snapshotHeader, strconv.FormatUint(m.From, 10))

	_, err = c.request(ctx, snapshotURI, h, r, nil)
	return
}

func (c *client) requestProto(ctx context.Context, uri string, in pbutil.Marshaler, out pbutil.Unmarshaler) (http.Header, error) {
	data, err := in.Marshal()
	if err != nil {
		return nil, err
	}

	b := bufferPool.Get().(*bytes.Buffer)
	b.Reset()
	b.Write(data)
	defer bufferPool.Put(b)

	return c.request(ctx, uri, nil, b, out)
}

func (c *client) request(ctx context.Context, uri string, h http.Header, body io.Reader, out pbutil.Unmarshaler) (http.Header, error) {
	u := join(c.url, uri)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, body)
	if err != nil {
		return nil, err
	}

	if h != nil {
		req.Header = h
	}

	tr := c.transport(ctx)
	res, err := tr.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	// return if rpc does not return responce.
	if res.StatusCode == http.StatusNoContent && out == nil {
		return res.Header, nil
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

	return res.Header, nil
}

func join(u, p string) string {
	u = strings.TrimSuffix(u, "/")
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")
	return u + "/" + p
}
