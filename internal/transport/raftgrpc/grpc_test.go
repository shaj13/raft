//nolint:dupl
package raftgrpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	transportmock "github.com/rakoo/raft/internal/mocks/transport"
	"github.com/rakoo/raft/internal/raftpb"
	"github.com/rakoo/raft/internal/transport/raftgrpc/pb"
	"github.com/rakoo/raft/raftlog"
	"github.com/stretchr/testify/require"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

const testGroupID = uint64(1)

func TestMessage(t *testing.T) {
	ln, c, srv := testClientServer(t)
	defer ln.Close()
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
	ln, c, srv := testClientServer(t)
	defer ln.Close()
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
	ln, c, srv := testClientServer(t)
	defer ln.Close()
	defer c.Close()

	table := []struct {
		name string
		err  error
	}{
		{
			name: "it return nil error when snapshot uploaded",
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

func testClientServer(tb testing.TB) (*bufconn.Listener, *client, *handler) {
	ln := bufconn.Listen(1024)
	srv := new(handler)
	srv.logger = raftlog.DefaultLogger

	server := grpc.NewServer()
	pb.RegisterRaftServer(server, srv)

	go func() {
		server.Serve(ln)
	}()

	dial := func(context.Context, string) (net.Conn, error) {
		return ln.Dial()
	}

	dopts := func(context.Context) []grpc.DialOption {
		return []grpc.DialOption{
			grpc.WithInsecure(),
			grpc.WithContextDialer(dial),
		}
	}

	copts := func(c context.Context) []grpc.CallOption { return nil }

	ctx := context.TODO()
	ctrl := gomock.NewController(tb)
	cfg := transportmock.NewMockConfig(ctrl)
	cfg.EXPECT().GroupID().Return(testGroupID).AnyTimes()
	cfg.EXPECT().Controller()

	c, err := Dialer(dopts, copts)(cfg)(ctx, "")
	if err != nil {
		tb.Fatal(err)
	}

	return ln, c.(*client), srv
}

type writeCloser struct {
	io.Writer
}

func (writeCloser) Close() error {
	return nil
}
