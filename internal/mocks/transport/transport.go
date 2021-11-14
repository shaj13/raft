// Code generated by MockGen. DO NOT EDIT.
// Source: internal/transport/types.go

// Package transportmock is a generated GoMock package.
package transportmock

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	raftpb "github.com/shaj13/raftkit/internal/raftpb"
	storage "github.com/shaj13/raftkit/internal/storage"
	transport "github.com/shaj13/raftkit/internal/transport"
	raftpb0 "go.etcd.io/etcd/raft/v3/raftpb"
)

// MockHandlerConfig is a mock of HandlerConfig interface.
type MockHandlerConfig struct {
	ctrl     *gomock.Controller
	recorder *MockHandlerConfigMockRecorder
}

// MockHandlerConfigMockRecorder is the mock recorder for MockHandlerConfig.
type MockHandlerConfigMockRecorder struct {
	mock *MockHandlerConfig
}

// NewMockHandlerConfig creates a new mock instance.
func NewMockHandlerConfig(ctrl *gomock.Controller) *MockHandlerConfig {
	mock := &MockHandlerConfig{ctrl: ctrl}
	mock.recorder = &MockHandlerConfigMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockHandlerConfig) EXPECT() *MockHandlerConfigMockRecorder {
	return m.recorder
}

// Controller mocks base method.
func (m *MockHandlerConfig) Controller() transport.Controller {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Controller")
	ret0, _ := ret[0].(transport.Controller)
	return ret0
}

// Controller indicates an expected call of Controller.
func (mr *MockHandlerConfigMockRecorder) Controller() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Controller", reflect.TypeOf((*MockHandlerConfig)(nil).Controller))
}

// GroupID mocks base method.
func (m *MockHandlerConfig) GroupID() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GroupID")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// GroupID indicates an expected call of GroupID.
func (mr *MockHandlerConfigMockRecorder) GroupID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GroupID", reflect.TypeOf((*MockHandlerConfig)(nil).GroupID))
}

// Snapshotter mocks base method.
func (m *MockHandlerConfig) Snapshotter() storage.Snapshotter {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Snapshotter")
	ret0, _ := ret[0].(storage.Snapshotter)
	return ret0
}

// Snapshotter indicates an expected call of Snapshotter.
func (mr *MockHandlerConfigMockRecorder) Snapshotter() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Snapshotter", reflect.TypeOf((*MockHandlerConfig)(nil).Snapshotter))
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

// GroupID mocks base method.
func (m *MockDialerConfig) GroupID() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GroupID")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// GroupID indicates an expected call of GroupID.
func (mr *MockDialerConfigMockRecorder) GroupID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GroupID", reflect.TypeOf((*MockDialerConfig)(nil).GroupID))
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

// MockHandler is a mock of Handler interface.
type MockHandler struct {
	ctrl     *gomock.Controller
	recorder *MockHandlerMockRecorder
}

// MockHandlerMockRecorder is the mock recorder for MockHandler.
type MockHandlerMockRecorder struct {
	mock *MockHandler
}

// NewMockHandler creates a new mock instance.
func NewMockHandler(ctrl *gomock.Controller) *MockHandler {
	mock := &MockHandler{ctrl: ctrl}
	mock.recorder = &MockHandlerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockHandler) EXPECT() *MockHandlerMockRecorder {
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
func (m *MockClient) Join(arg0 context.Context, arg1 raftpb.Member) (*raftpb.JoinResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Join", arg0, arg1)
	ret0, _ := ret[0].(*raftpb.JoinResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
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

// PromoteMember mocks base method.
func (m_2 *MockClient) PromoteMember(ctx context.Context, m raftpb.Member) error {
	m_2.ctrl.T.Helper()
	ret := m_2.ctrl.Call(m_2, "PromoteMember", ctx, m)
	ret0, _ := ret[0].(error)
	return ret0
}

// PromoteMember indicates an expected call of PromoteMember.
func (mr *MockClientMockRecorder) PromoteMember(ctx, m interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PromoteMember", reflect.TypeOf((*MockClient)(nil).PromoteMember), ctx, m)
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
func (m *MockController) Join(arg0 context.Context, arg1 uint64, arg2 *raftpb.Member) (*raftpb.JoinResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Join", arg0, arg1, arg2)
	ret0, _ := ret[0].(*raftpb.JoinResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Join indicates an expected call of Join.
func (mr *MockControllerMockRecorder) Join(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Join", reflect.TypeOf((*MockController)(nil).Join), arg0, arg1, arg2)
}

// PromoteMember mocks base method.
func (m *MockController) PromoteMember(arg0 context.Context, arg1 uint64, arg2 raftpb.Member) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PromoteMember", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// PromoteMember indicates an expected call of PromoteMember.
func (mr *MockControllerMockRecorder) PromoteMember(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PromoteMember", reflect.TypeOf((*MockController)(nil).PromoteMember), arg0, arg1, arg2)
}

// Push mocks base method.
func (m *MockController) Push(arg0 context.Context, arg1 uint64, arg2 raftpb0.Message) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Push", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Push indicates an expected call of Push.
func (mr *MockControllerMockRecorder) Push(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Push", reflect.TypeOf((*MockController)(nil).Push), arg0, arg1, arg2)
}
