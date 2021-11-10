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
	"github.com/shaj13/raftkit/internal/storage"
	"github.com/shaj13/raftkit/internal/transport"
	"go.etcd.io/etcd/pkg/v3/pbutil"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

const (
	snapshotHeader = "X-Raft-Snapshot"
	memberIDHeader = "X-Raft-Member-ID"
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
	return func(dc transport.DialerConfig) transport.Dial {
		return func(ctx context.Context, addr string) (transport.Client, error) {
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
	resp, err := c.requestProto(ctx, joinURI, &m, pool)
	if err != nil {
		return 0, nil, err
	}

	id, err := strconv.ParseUint(resp.Header.Get(memberIDHeader), 0, 64)
	if err != nil {
		return 0, nil, fmt.Errorf("raft/http: parse member id: %v", err)
	}

	return id, pool.Members, nil
}

func (c *client) PromoteMember(ctx context.Context, msg raftpb.Member) error {
	_, err := c.requestProto(ctx, promoteURI, &msg, nil)
	return err
}

func (c *client) message(ctx context.Context, msg etcdraftpb.Message) (err error) {
	_, err = c.requestProto(ctx, messageURI, &msg, nil)
	return
}

func (c *client) snapshot(ctx context.Context, msg etcdraftpb.Message) (err error) {
	meta := msg.Snapshot.Metadata
	r, err := c.shotter.Reader(meta.Term, meta.Index)
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

	if _, err := c.roundTrip(ctx, req, nil); err != nil {
		return err
	}

	return c.message(ctx, msg)
}

func (c *client) requestProto(ctx context.Context, uri string, in pbutil.Marshaler, out pbutil.Unmarshaler) (*http.Response, error) {
	data, err := in.Marshal()
	if err != nil {
		return nil, err
	}

	b := bufferPool.Get().(*bytes.Buffer)
	b.Reset()
	b.Write(data)
	defer bufferPool.Put(b)

	u := join(c.url, uri)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, b)
	if err != nil {
		return nil, err
	}

	return c.roundTrip(ctx, req, out)
}

func (c *client) roundTrip(ctx context.Context, req *http.Request, out pbutil.Unmarshaler) (*http.Response, error) {
	tr := c.transport(ctx)
	res, err := tr.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	// return if rpc does not return responce.
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
