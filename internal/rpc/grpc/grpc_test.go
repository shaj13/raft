package grpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/shaj13/raftkit/internal/mocks"
	"github.com/shaj13/raftkit/internal/raftpb"
	"github.com/shaj13/raftkit/internal/storage"
	"github.com/stretchr/testify/assert"
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
			rpcCtrl := mocks.NewMockController(ctrl)
			rpcCtrl.EXPECT().Push(gomock.Any(), gomock.Any()).Return(tt.err)
			srv.ctrl = rpcCtrl
			err := c.Message(context.Background(), etcdraftpb.Message{})
			if tt.err != nil {
				assert.Contains(t, err.Error(), tt.err.Error())
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
			rpcCtrl := mocks.NewMockController(ctrl)
			rpcCtrl.EXPECT().Join(gomock.Any(), gomock.Any()).Return(tt.id, tt.membs, tt.err)
			srv.ctrl = rpcCtrl
			id, pool, err := c.Join(context.Background(), raftpb.Member{})
			assert.Equal(t, tt.id, id)
			assert.Equal(t, tt.membs, pool)
			if tt.err != nil {
				assert.Contains(t, err.Error(), tt.err.Error())
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
			ctrl := gomock.NewController(t)
			rpcCtrl := mocks.NewMockController(ctrl)
			rpcCtrl.EXPECT().Push(gomock.Any(), gomock.Any()).Return(tt.err)
			snap := newTestSnapshoter("snap data")
			srv.ctrl = rpcCtrl
			srv.snap = snap
			c.shotter = snap
			err := c.snapshot(context.Background(), etcdraftpb.Message{})
			if tt.err != nil {
				assert.Contains(t, err.Error(), tt.err.Error())
			} else {
				snap.Assert(t)
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
	cfg := new(testConfig)

	ctx := context.TODO()
	c, err := Dialer(dopts, copts)(ctx, cfg)(ctx, "")
	if err != nil {
		tb.Fatal(err)
	}

	return ln, c.(*client), srv
}

func newTestSnapshoter(str string) *testSnapshoter {
	return &testSnapshoter{
		data:    []byte(str),
		buf:     new(bytes.Buffer),
		expname: "file.snap",
	}
}

type testSnapshoter struct {
	data    []byte
	buf     *bytes.Buffer
	expname string
	gotname string
}

func (t *testSnapshoter) Reader(context.Context, etcdraftpb.Snapshot) (string, io.ReadCloser, error) {
	return t.expname, ioutil.NopCloser(bytes.NewBuffer(t.data)), nil
}

func (t *testSnapshoter) Writer(_ context.Context, n string) (io.WriteCloser, func() (etcdraftpb.Snapshot, error), error) {
	t.gotname = n
	peek := func() (etcdraftpb.Snapshot, error) {
		return etcdraftpb.Snapshot{}, nil
	}
	return writeCloser{t.buf}, peek, nil
}

func (t *testSnapshoter) Write(sf *storage.SnapshotFile) error {
	return nil
}

func (t *testSnapshoter) Read(snap etcdraftpb.Snapshot) (*storage.SnapshotFile, error) {
	return nil, nil
}

func (t *testSnapshoter) ReadFromPath(string) (*storage.SnapshotFile, error) {
	return nil, nil
}

func (t *testSnapshoter) Assert(tb testing.TB) {
	assert.Equal(tb, t.expname, t.gotname)
	assert.Equal(tb, t.data, t.buf.Bytes())
}

type testConfig struct {
	dialopts []grpc.DialOption
}

func (t testConfig) CallOption() []grpc.CallOption {
	return []grpc.CallOption{}
}

func (t testConfig) DialOption() []grpc.DialOption {
	return t.dialopts
}

func (t testConfig) Snapshotter() storage.Snapshotter {
	return nil
}

type writeCloser struct {
	io.Writer
}

func (writeCloser) Close() error {
	return nil
}
