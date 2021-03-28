package grpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"testing"

	"github.com/shaj13/raftkit/api"
	"github.com/shaj13/raftkit/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

func TestMessage(t *testing.T) {
	method := "Push"
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
			ctrl := &mockController{mock.Mock{}}
			ctrl.On(method).Return(tt.err)
			srv.ctrl = ctrl
			err := c.Message(context.Background(), raftpb.Message{})
			ctrl.AssertCalled(t, method)
			if tt.err != nil {
				assert.Contains(t, err.Error(), tt.err.Error())
			}
		})
	}
}

func TestJoin(t *testing.T) {
	method := "Join"
	ln, c, srv := testClientServer(t)
	defer ln.Close()
	defer c.Close()

	table := []struct {
		name  string
		id    uint64
		membs []api.Member
		err   error
	}{
		{
			name:  "it return id and pool when joined",
			id:    11,
			membs: []api.Member{{ID: 12}},
			err:   nil,
		},
		{
			name: "it return error when server return error",
			id:   0,
			// membs: []api.Member{},
			err: fmt.Errorf("TestJoin Error"),
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := &mockController{mock.Mock{}}
			ctrl.On(method).Return(tt.id, tt.membs, tt.err)
			srv.ctrl = ctrl
			id, pool, err := c.Join(context.Background(), api.Member{})
			ctrl.AssertCalled(t, method)
			assert.Equal(t, tt.id, id)
			assert.Equal(t, tt.membs, pool.Members)
			if tt.err != nil {
				assert.Contains(t, err.Error(), tt.err.Error())
			}
		})
	}
}

func TestSnapshot(t *testing.T) {
	method := "Push"
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
			ctrl := &mockController{mock.Mock{}}
			ctrl.On(method).Return(tt.err)
			snap := newTestSnapshoter("snap data")
			srv.ctrl = ctrl
			srv.snap = snap
			c.snapshoter = snap
			err := c.snapshot(context.Background(), raftpb.Message{})
			ctrl.AssertCalled(t, method)
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
	api.RegisterRaftServer(server, srv)

	go func() {
		server.Serve(ln)
	}()

	dial := func(context.Context, string) (net.Conn, error) {
		return ln.Dial()
	}

	cfg := testConfig{
		dialopts: []grpc.DialOption{
			grpc.WithInsecure(),
			grpc.WithContextDialer(dial),
		},
	}

	c, err := Dial(context.TODO(), cfg, "")
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

type mockController struct {
	mock.Mock
}

func (m *mockController) Push(context.Context, raftpb.Message) error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockController) Join(context.Context, *api.Member) (uint64, []api.Member, error) {
	args := m.Called()
	return args.Get(0).(uint64), args.Get(1).([]api.Member), args.Error(2)
}

type testSnapshoter struct {
	data    []byte
	buf     *bytes.Buffer
	expname string
	gotname string
}

func (t *testSnapshoter) Reader(context.Context, raftpb.Message) (string, io.ReadCloser, error) {
	return t.expname, ioutil.NopCloser(bytes.NewBuffer(t.data)), nil
}

func (t *testSnapshoter) Writer(_ context.Context, n string) (io.WriteCloser, func() (raftpb.Snapshot, error), error) {
	t.gotname = n
	peek := func() (raftpb.Snapshot, error) {
		return raftpb.Snapshot{}, nil
	}
	return writeCloser{t.buf}, peek, nil
}

func (t *testSnapshoter) Write(sf *storage.SnapshotFile) error {
	return nil
}

func (t *testSnapshoter) Read(snap raftpb.Snapshot) (*storage.SnapshotFile, error) {
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

func (t testConfig) Snapshoter() storage.Snapshoter {
	return nil
}

type writeCloser struct {
	io.Writer
}

func (writeCloser) Close() error {
	return nil
}
