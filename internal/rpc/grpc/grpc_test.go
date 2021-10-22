package grpc

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
	"github.com/shaj13/raftkit/internal/mocks"
	rpcmock "github.com/shaj13/raftkit/internal/mocks/rpc"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/stretchr/testify/require"
	etcdraftpb "go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

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
			rpcCtrl := rpcmock.NewMockController(ctrl)
			rpcCtrl.EXPECT().Push(gomock.Any(), gomock.Any()).Return(tt.err)
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
		name  string
		id    uint64
		membs []raftpb.Member
		err   error
	}{
		{
			name:  "it return id and pool when joined",
			id:    11,
			membs: []raftpb.Member{{ID: 12}},
			err:   nil,
		},
		{
			name: "it return error when server return error",
			err:  fmt.Errorf("TestJoin Error"),
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			rpcCtrl := rpcmock.NewMockController(ctrl)
			rpcCtrl.EXPECT().Join(gomock.Any(), gomock.Any()).Return(tt.id, tt.membs, tt.err)
			srv.ctrl = rpcCtrl
			id, pool, err := c.Join(context.Background(), raftpb.Member{})
			require.Equal(t, tt.id, id)
			require.Equal(t, tt.membs, pool)
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
			expName := "file.snap"
			gotName := ""
			ctrl := gomock.NewController(t)

			rpcCtrl := rpcmock.NewMockController(ctrl)
			rpcCtrl.EXPECT().Push(gomock.Any(), gomock.Any()).Return(tt.err)

			shotter := mocks.NewMockSnapshotter(ctrl)
			shotter.
				EXPECT().
				Reader(gomock.Any()).
				Return(expName, ioutil.NopCloser(strings.NewReader(snapData)), nil)
			shotter.
				EXPECT().
				Writer(gomock.Any()).
				DoAndReturn(func(name string) (io.WriteCloser, func() (etcdraftpb.Snapshot, error), error) {
					gotName = name
					peek := func() (etcdraftpb.Snapshot, error) {
						return etcdraftpb.Snapshot{}, nil
					}
					return writeCloser{buf}, peek, nil
				})

			srv.ctrl = rpcCtrl
			srv.snap = shotter
			c.shotter = shotter
			err := c.snapshot(context.Background(), etcdraftpb.Message{})
			if tt.err != nil {
				require.Contains(t, err.Error(), tt.err.Error())
			} else {
				require.Equal(t, expName, gotName)
				require.Equal(t, snapData, buf.String())
			}
		})
	}
}

func testClientServer(tb testing.TB) (*bufconn.Listener, *client, *server) {
	ln := bufconn.Listen(1024)
	srv := new(server)

	server := grpc.NewServer()
	raftpb.RegisterRaftServer(server, srv)

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
	cfg := rpcmock.NewMockDialerConfig(ctrl)
	cfg.EXPECT().Snapshotter().Return(nil)

	c, err := Dialer(dopts, copts)(ctx, cfg)(ctx, "")
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
