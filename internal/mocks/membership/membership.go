// Code generated by MockGen. DO NOT EDIT.
// Source: internal/membership/types.go

// Package membershipmock is a generated GoMock package.
package membershipmock

import (
	context "context"
	reflect "reflect"
	time "time"

	gomock "github.com/golang/mock/gomock"
	membership "github.com/shaj13/raft/internal/membership"
	raftpb "github.com/shaj13/raft/internal/raftpb"
	transport "github.com/shaj13/raft/internal/transport"
	raftlog "github.com/shaj13/raft/raftlog"
	raft "go.etcd.io/etcd/raft/v3"
	raftpb0 "go.etcd.io/etcd/raft/v3/raftpb"
)

// MockMember is a mock of Member interface.
type MockMember struct {
	ctrl     *gomock.Controller
	recorder *MockMemberMockRecorder
}

// MockMemberMockRecorder is the mock recorder for MockMember.
type MockMemberMockRecorder struct {
	mock *MockMember
}

// NewMockMember creates a new mock instance.
func NewMockMember(ctrl *gomock.Controller) *MockMember {
	mock := &MockMember{ctrl: ctrl}
	mock.recorder = &MockMemberMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMember) EXPECT() *MockMemberMockRecorder {
	return m.recorder
}

// ActiveSince mocks base method.
func (m *MockMember) ActiveSince() time.Time {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ActiveSince")
	ret0, _ := ret[0].(time.Time)
	return ret0
}

// ActiveSince indicates an expected call of ActiveSince.
func (mr *MockMemberMockRecorder) ActiveSince() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ActiveSince", reflect.TypeOf((*MockMember)(nil).ActiveSince))
}

// Address mocks base method.
func (m *MockMember) Address() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Address")
	ret0, _ := ret[0].(string)
	return ret0
}

// Address indicates an expected call of Address.
func (mr *MockMemberMockRecorder) Address() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Address", reflect.TypeOf((*MockMember)(nil).Address))
}

// Close mocks base method.
func (m *MockMember) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockMemberMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockMember)(nil).Close))
}

// ID mocks base method.
func (m *MockMember) ID() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ID")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// ID indicates an expected call of ID.
func (mr *MockMemberMockRecorder) ID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ID", reflect.TypeOf((*MockMember)(nil).ID))
}

// IsActive mocks base method.
func (m *MockMember) IsActive() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsActive")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsActive indicates an expected call of IsActive.
func (mr *MockMemberMockRecorder) IsActive() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsActive", reflect.TypeOf((*MockMember)(nil).IsActive))
}

// Raw mocks base method.
func (m *MockMember) Raw() raftpb.Member {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Raw")
	ret0, _ := ret[0].(raftpb.Member)
	return ret0
}

// Raw indicates an expected call of Raw.
func (mr *MockMemberMockRecorder) Raw() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Raw", reflect.TypeOf((*MockMember)(nil).Raw))
}

// Send mocks base method.
func (m *MockMember) Send(arg0 raftpb0.Message) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Send", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Send indicates an expected call of Send.
func (mr *MockMemberMockRecorder) Send(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Send", reflect.TypeOf((*MockMember)(nil).Send), arg0)
}

// TearDown mocks base method.
func (m *MockMember) TearDown(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TearDown", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// TearDown indicates an expected call of TearDown.
func (mr *MockMemberMockRecorder) TearDown(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TearDown", reflect.TypeOf((*MockMember)(nil).TearDown), ctx)
}

// Type mocks base method.
func (m *MockMember) Type() raftpb.MemberType {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Type")
	ret0, _ := ret[0].(raftpb.MemberType)
	return ret0
}

// Type indicates an expected call of Type.
func (mr *MockMemberMockRecorder) Type() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Type", reflect.TypeOf((*MockMember)(nil).Type))
}

// Update mocks base method.
func (m_2 *MockMember) Update(m raftpb.Member) error {
	m_2.ctrl.T.Helper()
	ret := m_2.ctrl.Call(m_2, "Update", m)
	ret0, _ := ret[0].(error)
	return ret0
}

// Update indicates an expected call of Update.
func (mr *MockMemberMockRecorder) Update(m interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockMember)(nil).Update), m)
}

// MockReporter is a mock of Reporter interface.
type MockReporter struct {
	ctrl     *gomock.Controller
	recorder *MockReporterMockRecorder
}

// MockReporterMockRecorder is the mock recorder for MockReporter.
type MockReporterMockRecorder struct {
	mock *MockReporter
}

// NewMockReporter creates a new mock instance.
func NewMockReporter(ctrl *gomock.Controller) *MockReporter {
	mock := &MockReporter{ctrl: ctrl}
	mock.recorder = &MockReporterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockReporter) EXPECT() *MockReporterMockRecorder {
	return m.recorder
}

// ReportShutdown mocks base method.
func (m *MockReporter) ReportShutdown(id uint64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ReportShutdown", id)
}

// ReportShutdown indicates an expected call of ReportShutdown.
func (mr *MockReporterMockRecorder) ReportShutdown(id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReportShutdown", reflect.TypeOf((*MockReporter)(nil).ReportShutdown), id)
}

// ReportSnapshot mocks base method.
func (m *MockReporter) ReportSnapshot(id uint64, status raft.SnapshotStatus) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ReportSnapshot", id, status)
}

// ReportSnapshot indicates an expected call of ReportSnapshot.
func (mr *MockReporterMockRecorder) ReportSnapshot(id, status interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReportSnapshot", reflect.TypeOf((*MockReporter)(nil).ReportSnapshot), id, status)
}

// ReportUnreachable mocks base method.
func (m *MockReporter) ReportUnreachable(id uint64) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ReportUnreachable", id)
}

// ReportUnreachable indicates an expected call of ReportUnreachable.
func (mr *MockReporterMockRecorder) ReportUnreachable(id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReportUnreachable", reflect.TypeOf((*MockReporter)(nil).ReportUnreachable), id)
}

// MockConfig is a mock of Config interface.
type MockConfig struct {
	ctrl     *gomock.Controller
	recorder *MockConfigMockRecorder
}

// MockConfigMockRecorder is the mock recorder for MockConfig.
type MockConfigMockRecorder struct {
	mock *MockConfig
}

// NewMockConfig creates a new mock instance.
func NewMockConfig(ctrl *gomock.Controller) *MockConfig {
	mock := &MockConfig{ctrl: ctrl}
	mock.recorder = &MockConfigMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockConfig) EXPECT() *MockConfigMockRecorder {
	return m.recorder
}

// Context mocks base method.
func (m *MockConfig) Context() context.Context {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Context")
	ret0, _ := ret[0].(context.Context)
	return ret0
}

// Context indicates an expected call of Context.
func (mr *MockConfigMockRecorder) Context() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Context", reflect.TypeOf((*MockConfig)(nil).Context))
}

// Dial mocks base method.
func (m *MockConfig) Dial() transport.Dial {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Dial")
	ret0, _ := ret[0].(transport.Dial)
	return ret0
}

// Dial indicates an expected call of Dial.
func (mr *MockConfigMockRecorder) Dial() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Dial", reflect.TypeOf((*MockConfig)(nil).Dial))
}

// DrainTimeout mocks base method.
func (m *MockConfig) DrainTimeout() time.Duration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DrainTimeout")
	ret0, _ := ret[0].(time.Duration)
	return ret0
}

// DrainTimeout indicates an expected call of DrainTimeout.
func (mr *MockConfigMockRecorder) DrainTimeout() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DrainTimeout", reflect.TypeOf((*MockConfig)(nil).DrainTimeout))
}

// Logger mocks base method.
func (m *MockConfig) Logger() raftlog.Logger {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Logger")
	ret0, _ := ret[0].(raftlog.Logger)
	return ret0
}

// Logger indicates an expected call of Logger.
func (mr *MockConfigMockRecorder) Logger() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Logger", reflect.TypeOf((*MockConfig)(nil).Logger))
}

// Reporter mocks base method.
func (m *MockConfig) Reporter() membership.Reporter {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Reporter")
	ret0, _ := ret[0].(membership.Reporter)
	return ret0
}

// Reporter indicates an expected call of Reporter.
func (mr *MockConfigMockRecorder) Reporter() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Reporter", reflect.TypeOf((*MockConfig)(nil).Reporter))
}

// StreamTimeout mocks base method.
func (m *MockConfig) StreamTimeout() time.Duration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StreamTimeout")
	ret0, _ := ret[0].(time.Duration)
	return ret0
}

// StreamTimeout indicates an expected call of StreamTimeout.
func (mr *MockConfigMockRecorder) StreamTimeout() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StreamTimeout", reflect.TypeOf((*MockConfig)(nil).StreamTimeout))
}

// MockPool is a mock of Pool interface.
type MockPool struct {
	ctrl     *gomock.Controller
	recorder *MockPoolMockRecorder
}

// MockPoolMockRecorder is the mock recorder for MockPool.
type MockPoolMockRecorder struct {
	mock *MockPool
}

// NewMockPool creates a new mock instance.
func NewMockPool(ctrl *gomock.Controller) *MockPool {
	mock := &MockPool{ctrl: ctrl}
	mock.recorder = &MockPoolMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPool) EXPECT() *MockPoolMockRecorder {
	return m.recorder
}

// Add mocks base method.
func (m *MockPool) Add(arg0 raftpb.Member) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Add", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Add indicates an expected call of Add.
func (mr *MockPoolMockRecorder) Add(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Add", reflect.TypeOf((*MockPool)(nil).Add), arg0)
}

// Get mocks base method.
func (m *MockPool) Get(arg0 uint64) (membership.Member, bool) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0)
	ret0, _ := ret[0].(membership.Member)
	ret1, _ := ret[1].(bool)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockPoolMockRecorder) Get(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockPool)(nil).Get), arg0)
}

// Members mocks base method.
func (m *MockPool) Members() []membership.Member {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Members")
	ret0, _ := ret[0].([]membership.Member)
	return ret0
}

// Members indicates an expected call of Members.
func (mr *MockPoolMockRecorder) Members() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Members", reflect.TypeOf((*MockPool)(nil).Members))
}

// NextID mocks base method.
func (m *MockPool) NextID() uint64 {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NextID")
	ret0, _ := ret[0].(uint64)
	return ret0
}

// NextID indicates an expected call of NextID.
func (mr *MockPoolMockRecorder) NextID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NextID", reflect.TypeOf((*MockPool)(nil).NextID))
}

// RegisterTypeMatcher mocks base method.
func (m *MockPool) RegisterTypeMatcher(arg0 func(raftpb.Member) raftpb.MemberType) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "RegisterTypeMatcher", arg0)
}

// RegisterTypeMatcher indicates an expected call of RegisterTypeMatcher.
func (mr *MockPoolMockRecorder) RegisterTypeMatcher(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RegisterTypeMatcher", reflect.TypeOf((*MockPool)(nil).RegisterTypeMatcher), arg0)
}

// Remove mocks base method.
func (m *MockPool) Remove(arg0 raftpb.Member) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Remove", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Remove indicates an expected call of Remove.
func (mr *MockPoolMockRecorder) Remove(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Remove", reflect.TypeOf((*MockPool)(nil).Remove), arg0)
}

// Restore mocks base method.
func (m *MockPool) Restore(arg0 []raftpb.Member) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Restore", arg0)
}

// Restore indicates an expected call of Restore.
func (mr *MockPoolMockRecorder) Restore(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Restore", reflect.TypeOf((*MockPool)(nil).Restore), arg0)
}

// Snapshot mocks base method.
func (m *MockPool) Snapshot() []raftpb.Member {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Snapshot")
	ret0, _ := ret[0].([]raftpb.Member)
	return ret0
}

// Snapshot indicates an expected call of Snapshot.
func (mr *MockPoolMockRecorder) Snapshot() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Snapshot", reflect.TypeOf((*MockPool)(nil).Snapshot))
}

// TearDown mocks base method.
func (m *MockPool) TearDown(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TearDown", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// TearDown indicates an expected call of TearDown.
func (mr *MockPoolMockRecorder) TearDown(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TearDown", reflect.TypeOf((*MockPool)(nil).TearDown), arg0)
}

// Update mocks base method.
func (m *MockPool) Update(arg0 raftpb.Member) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Update", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Update indicates an expected call of Update.
func (mr *MockPoolMockRecorder) Update(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockPool)(nil).Update), arg0)
}
