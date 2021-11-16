package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	transportmock "github.com/shaj13/raftkit/internal/mocks/transport"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/stretchr/testify/require"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
)

const testGroupID = uint64(1)

func TestMessage(t *testing.T) {
	ts, c, srv := testClientServer(t)
	defer ts.Close()
	defer c.Close()

	table := []struct {
		name string
		err  error
	}{
		{
			name: "it return nil error when server process msg",
			err:  nil,
		},
		{
			name: "it return error when server return error",
			err:  fmt.Errorf("TestMessage Error"),
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			rpcCtrl := transportmock.NewMockController(ctrl)
			rpcCtrl.EXPECT().Push(gomock.Any(), gomock.Eq(testGroupID), gomock.Any()).Return(tt.err)
			srv.ctrl = rpcCtrl
			err := c.Message(context.Background(), etcdraftpb.Message{})
			if tt.err != nil {
				require.Contains(t, err.Error(), tt.err.Error())
			}
		})
	}
}

func TestJoin(t *testing.T) {
	ts, c, srv := testClientServer(t)
	defer ts.Close()
	defer c.Close()

	table := []struct {
		name string
		resp *raftpb.JoinResponse
		err  error
	}{
		{
			name: "it return join resp when joined",
			resp: &raftpb.JoinResponse{
				ID:      11,
				Members: []raftpb.Member{{ID: 12}},
			},
			err: nil,
		},
		{
			name: "it return error when server return error",
			resp: new(raftpb.JoinResponse),
			err:  fmt.Errorf("TestJoin Error"),
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			rpcCtrl := transportmock.NewMockController(ctrl)
			rpcCtrl.EXPECT().Join(gomock.Any(), gomock.Eq(testGroupID), gomock.Any()).Return(tt.resp, tt.err)
			srv.ctrl = rpcCtrl
			resp, err := c.Join(context.Background(), raftpb.Member{})
			require.Equal(t, tt.resp, resp)
			if tt.err != nil {
				require.Contains(t, err.Error(), tt.err.Error())
			}
		})
	}
}

func TestSnapshot(t *testing.T) {
	ts, c, srv := testClientServer(t)
	defer ts.Close()
	defer c.Close()

	table := []struct {
		name string
		err  error
	}{
		{
			name: "it return id and pool when joined",
			err:  nil,
		},
		{
			name: "it return error when server return error",
			err:  fmt.Errorf("TestSnapshot Error"),
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			snapData := "some snap data"
			ctrl := gomock.NewController(t)

			rpcCtrl := transportmock.NewMockController(ctrl)
			rpcCtrl.EXPECT().Push(gomock.Any(), gomock.Eq(testGroupID), gomock.Any()).Return(tt.err)
			rpcCtrl.
				EXPECT().
				SnapshotReader(gomock.Eq(testGroupID), gomock.Any(), gomock.Any()).
				Return(ioutil.NopCloser(strings.NewReader(snapData)), nil)
			rpcCtrl.
				EXPECT().
				SnapshotWriter(gomock.Eq(testGroupID), gomock.Any(), gomock.Any()).
				Return(writeCloser{buf}, nil)

			srv.ctrl = rpcCtrl
			c.ctrl = rpcCtrl
			err := c.snapshot(context.Background(), etcdraftpb.Message{})
			if tt.err != nil {
				require.Contains(t, err.Error(), tt.err.Error())
			} else {
				require.Equal(t, snapData, buf.String())
			}
		})
	}
}

func TestPromoteMember(t *testing.T) {
	ts, c, srv := testClientServer(t)
	defer ts.Close()
	defer c.Close()

	table := []struct {
		name string
		err  error
	}{
		{
			name: "it return nil error when server process promote",
			err:  nil,
		},
		{
			name: "it return error when server return error",
			err:  fmt.Errorf("TestPromoteMember Error"),
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			rpcCtrl := transportmock.NewMockController(ctrl)
			rpcCtrl.EXPECT().PromoteMember(gomock.Any(), gomock.Eq(testGroupID), gomock.Any()).Return(tt.err)
			srv.ctrl = rpcCtrl
			err := c.PromoteMember(context.Background(), raftpb.Member{})
			if tt.err != nil {
				require.Contains(t, err.Error(), tt.err.Error())
			}
		})
	}
}

func testClientServer(tb testing.TB) (*httptest.Server, *client, *handler) {
	srv := new(handler)
	ts := httptest.NewServer(mux(srv, ""))

	ctx := context.TODO()
	ctrl := gomock.NewController(tb)
	cfg := transportmock.NewMockConfig(ctrl)
	cfg.EXPECT().Controller()
	cfg.EXPECT().GroupID().Return(testGroupID).AnyTimes()

	tr := func(context.Context) http.RoundTripper {
		return testRoundTripper{ts.Client()}
	}

	c, err := Dialer(tr, "")(cfg)(ctx, ts.URL)
	if err != nil {
		tb.Fatal(err)
	}

	return ts, c.(*client), srv
}

type writeCloser struct {
	io.Writer
}

func (writeCloser) Close() error {
	return nil
}

type testRoundTripper struct {
	c *http.Client
}

func (trt testRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return trt.c.Do(r)
}
