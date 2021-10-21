// Code generated by MockGen. DO NOT EDIT.
// Source: internal/rpc/types.go

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	raftpb "github.com/shaj13/raftkit/internal/raftpb"
	rpc "github.com/shaj13/raftkit/internal/rpc"
	storage "github.com/shaj13/raftkit/internal/storage"
	raftpb0 "go.etcd.io/etcd/raft/v3/raftpb"
)

// MockServerConfig is a mock of ServerConfig interface.
type MockServerConfig struct {
	ctrl     *gomock.Controller
	recorder *MockServerConfigMockRecorder
}

// MockServerConfigMockRecorder is the mock recorder for MockServerConfig.
type MockServerConfigMockRecorder struct {
	mock *MockServerConfig
}

// NewMockServerConfig creates a new mock instance.
func NewMockServerConfig(ctrl *gomock.Controller) *MockServerConfig {
	mock := &MockServerConfig{ctrl: ctrl}
	mock.recorder = &MockServerConfigMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockServerConfig) EXPECT() *MockServerConfigMockRecorder {
	return m.recorder
}

// Controller mocks base method.
func (m *MockServerConfig) Controller() rpc.Controller {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Controller")
	ret0, _ := ret[0].(rpc.Controller)
	return ret0
}

// Controller indicates an expected call of Controller.
func (mr *MockServerConfigMockRecorder) Controller() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Controller", reflect.TypeOf((*MockServerConfig)(nil).Controller))
}

// Snapshotter mocks base method.
func (m *MockServerConfig) Snapshotter() storage.Snapshotter {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Snapshotter")
	ret0, _ := ret[0].(storage.Snapshotter)
	return ret0
}

// Snapshotter indicates an expected call of Snapshotter.
func (mr *MockServerConfigMockRecorder) Snapshotter() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Snapshotter", reflect.TypeOf((*MockServerConfig)(nil).Snapshotter))
}

// MockDialerConfig is a mock of DialerConfig interface.
type MockDialerConfig struct {
	ctrl     *gomock.Controller
	recorder *MockDialerConfigMockRecorder
}

// MockDialerConfigMockRecorder is the mock recorder for MockDialerConfig.
type MockDialerConfigMockRecorder struct {
	mock *MockDialerConfig
}

// NewMockDialerConfig creates a new mock instance.
func NewMockDialerConfig(ctrl *gomock.Controller) *MockDialerConfig {
	mock := &MockDialerConfig{ctrl: ctrl}
	mock.recorder = &MockDialerConfigMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDialerConfig) EXPECT() *MockDialerConfigMockRecorder {
	return m.recorder
}

// Snapshotter mocks base method.
func (m *MockDialerConfig) Snapshotter() storage.Snapshotter {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Snapshotter")
	ret0, _ := ret[0].(storage.Snapshotter)
	return ret0
}

// Snapshotter indicates an expected call of Snapshotter.
func (mr *MockDialerConfigMockRecorder) Snapshotter() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Snapshotter", reflect.TypeOf((*MockDialerConfig)(nil).Snapshotter))
}

// MockServer is a mock of Server interface.
type MockServer struct {
	ctrl     *gomock.Controller
	recorder *MockServerMockRecorder
}

// MockServerMockRecorder is the mock recorder for MockServer.
type MockServerMockRecorder struct {
	mock *MockServer
}

// NewMockServer creates a new mock instance.
func NewMockServer(ctrl *gomock.Controller) *MockServer {
	mock := &MockServer{ctrl: ctrl}
	mock.recorder = &MockServerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockServer) EXPECT() *MockServerMockRecorder {
	return m.recorder
}

// MockClient is a mock of Client interface.
type MockClient struct {
	ctrl     *gomock.Controller
	recorder *MockClientMockRecorder
}

// MockClientMockRecorder is the mock recorder for MockClient.
type MockClientMockRecorder struct {
	mock *MockClient
}

// NewMockClient creates a new mock instance.
func NewMockClient(ctrl *gomock.Controller) *MockClient {
	mock := &MockClient{ctrl: ctrl}
	mock.recorder = &MockClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockClient) EXPECT() *MockClientMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockClient) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockClientMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockClient)(nil).Close))
}

// Join mocks base method.
func (m *MockClient) Join(arg0 context.Context, arg1 raftpb.Member) (uint64, []raftpb.Member, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Join", arg0, arg1)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].([]raftpb.Member)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Join indicates an expected call of Join.
func (mr *MockClientMockRecorder) Join(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Join", reflect.TypeOf((*MockClient)(nil).Join), arg0, arg1)
}

// Message mocks base method.
func (m *MockClient) Message(arg0 context.Context, arg1 raftpb0.Message) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Message", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Message indicates an expected call of Message.
func (mr *MockClientMockRecorder) Message(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Message", reflect.TypeOf((*MockClient)(nil).Message), arg0, arg1)
}

// MockController is a mock of Controller interface.
type MockController struct {
	ctrl     *gomock.Controller
	recorder *MockControllerMockRecorder
}

// MockControllerMockRecorder is the mock recorder for MockController.
type MockControllerMockRecorder struct {
	mock *MockController
}

// NewMockController creates a new mock instance.
func NewMockController(ctrl *gomock.Controller) *MockController {
	mock := &MockController{ctrl: ctrl}
	mock.recorder = &MockControllerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockController) EXPECT() *MockControllerMockRecorder {
	return m.recorder
}

// Join mocks base method.
func (m *MockController) Join(arg0 context.Context, arg1 *raftpb.Member) (uint64, []raftpb.Member, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Join", arg0, arg1)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].([]raftpb.Member)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Join indicates an expected call of Join.
func (mr *MockControllerMockRecorder) Join(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Join", reflect.TypeOf((*MockController)(nil).Join), arg0, arg1)
}

// Push mocks base method.
func (m *MockController) Push(arg0 context.Context, arg1 raftpb0.Message) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Push", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Push indicates an expected call of Push.
func (mr *MockControllerMockRecorder) Push(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Push", reflect.TypeOf((*MockController)(nil).Push), arg0, arg1)
}
