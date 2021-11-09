// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: internal/transport/grpc/pb/raft.proto

package pb

import (
	context "context"
	fmt "fmt"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
	empty "github.com/golang/protobuf/ptypes/empty"
	raftpb "github.com/shaj13/raftkit/internal/raftpb"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	io "io"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type Chunk struct {
	// Index specifies the chunk index.
	Index uint64 `protobuf:"varint,1,opt,name=index,proto3" json:"index,omitempty"`
	// Data specifies the raw chunk data.
	Data                 []byte   `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Chunk) Reset()         { *m = Chunk{} }
func (m *Chunk) String() string { return proto.CompactTextString(m) }
func (*Chunk) ProtoMessage()    {}
func (*Chunk) Descriptor() ([]byte, []int) {
	return fileDescriptor_3973619806d997ba, []int{0}
}
func (m *Chunk) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Chunk) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Chunk.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Chunk) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Chunk.Merge(m, src)
}
func (m *Chunk) XXX_Size() int {
	return m.Size()
}
func (m *Chunk) XXX_DiscardUnknown() {
	xxx_messageInfo_Chunk.DiscardUnknown(m)
}

var xxx_messageInfo_Chunk proto.InternalMessageInfo

func init() {
	proto.RegisterType((*Chunk)(nil), "pb.Chunk")
}

func init() {
	proto.RegisterFile("internal/transport/grpc/pb/raft.proto", fileDescriptor_3973619806d997ba)
}

var fileDescriptor_3973619806d997ba = []byte{
	// 293 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x8e, 0xcb, 0x4a, 0x03, 0x31,
	0x14, 0x86, 0x27, 0x65, 0xea, 0x25, 0xa8, 0x8b, 0x50, 0xa4, 0x8c, 0x30, 0x94, 0x82, 0x50, 0x5c,
	0x24, 0xad, 0x75, 0xe3, 0x56, 0x71, 0x23, 0x14, 0xa4, 0x3e, 0x41, 0xd2, 0xa6, 0x99, 0xb1, 0x9d,
	0x9c, 0x90, 0x39, 0x05, 0x7d, 0xc3, 0x2e, 0x8b, 0x4f, 0x60, 0xe7, 0x49, 0x64, 0x32, 0x2a, 0xa8,
	0x08, 0xee, 0xce, 0x77, 0x6e, 0xff, 0x47, 0xcf, 0x73, 0x8b, 0xda, 0x5b, 0xb9, 0x12, 0xe8, 0xa5,
	0x2d, 0x1d, 0x78, 0x14, 0xc6, 0xbb, 0x99, 0x70, 0x4a, 0x78, 0xb9, 0x40, 0xee, 0x3c, 0x20, 0xb0,
	0x96, 0x53, 0x49, 0xc7, 0x80, 0x81, 0x80, 0xa2, 0xae, 0x9a, 0x49, 0x72, 0x66, 0x00, 0xcc, 0x4a,
	0x8b, 0x40, 0x6a, 0xbd, 0x10, 0xba, 0x70, 0xf8, 0xf2, 0x31, 0xbc, 0x32, 0x39, 0x66, 0x6b, 0xc5,
	0x67, 0x50, 0x88, 0x32, 0x93, 0x4f, 0xa3, 0x71, 0x78, 0xba, 0xcc, 0x51, 0x7c, 0xe5, 0xd6, 0x8d,
	0x6f, 0x61, 0xfd, 0x11, 0x6d, 0xdf, 0x66, 0x6b, 0xbb, 0x64, 0x1d, 0xda, 0xce, 0xed, 0x5c, 0x3f,
	0x77, 0x49, 0x8f, 0x0c, 0xe2, 0x69, 0x03, 0x8c, 0xd1, 0x78, 0x2e, 0x51, 0x76, 0x5b, 0x3d, 0x32,
	0x38, 0x9a, 0x86, 0xfa, 0xf2, 0x95, 0xd0, 0x78, 0x2a, 0x17, 0xc8, 0x86, 0x74, 0x7f, 0xa2, 0xcb,
	0x52, 0x1a, 0xcd, 0x0e, 0xb9, 0x53, 0x3c, 0x3c, 0x4a, 0x4e, 0x79, 0x63, 0xc9, 0x3f, 0x2d, 0xf9,
	0x5d, 0x6d, 0xd9, 0x8f, 0x06, 0x84, 0x8d, 0xe8, 0xc1, 0xa3, 0x95, 0xae, 0xcc, 0x00, 0xff, 0x7b,
	0x72, 0x41, 0xe3, 0x7b, 0xc8, 0x2d, 0x3b, 0xe1, 0x8d, 0x3c, 0x9f, 0xe8, 0x42, 0x69, 0x9f, 0xfc,
	0xe0, 0x7e, 0x34, 0x24, 0xec, 0x9a, 0x1e, 0x3f, 0x78, 0x28, 0x00, 0x75, 0xd3, 0xfc, 0x75, 0xf4,
	0x67, 0xd0, 0x4d, 0x67, 0xb3, 0x4b, 0xa3, 0xed, 0x2e, 0x8d, 0x36, 0x55, 0x4a, 0xb6, 0x55, 0x4a,
	0xde, 0xaa, 0x94, 0xa8, 0xbd, 0xb0, 0x37, 0x7e, 0x0f, 0x00, 0x00, 0xff, 0xff, 0x48, 0x7a, 0x62,
	0x15, 0xba, 0x01, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// RaftClient is the client API for Raft service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type RaftClient interface {
	Message(ctx context.Context, opts ...grpc.CallOption) (Raft_MessageClient, error)
	Snapshot(ctx context.Context, opts ...grpc.CallOption) (Raft_SnapshotClient, error)
	Join(ctx context.Context, in *raftpb.Member, opts ...grpc.CallOption) (Raft_JoinClient, error)
	PromoteMember(ctx context.Context, in *raftpb.Member, opts ...grpc.CallOption) (*empty.Empty, error)
}

type raftClient struct {
	cc *grpc.ClientConn
}

func NewRaftClient(cc *grpc.ClientConn) RaftClient {
	return &raftClient{cc}
}

func (c *raftClient) Message(ctx context.Context, opts ...grpc.CallOption) (Raft_MessageClient, error) {
	stream, err := c.cc.NewStream(ctx, &_Raft_serviceDesc.Streams[0], "/pb.Raft/Message", opts...)
	if err != nil {
		return nil, err
	}
	x := &raftMessageClient{stream}
	return x, nil
}

type Raft_MessageClient interface {
	Send(*Chunk) error
	CloseAndRecv() (*empty.Empty, error)
	grpc.ClientStream
}

type raftMessageClient struct {
	grpc.ClientStream
}

func (x *raftMessageClient) Send(m *Chunk) error {
	return x.ClientStream.SendMsg(m)
}

func (x *raftMessageClient) CloseAndRecv() (*empty.Empty, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(empty.Empty)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *raftClient) Snapshot(ctx context.Context, opts ...grpc.CallOption) (Raft_SnapshotClient, error) {
	stream, err := c.cc.NewStream(ctx, &_Raft_serviceDesc.Streams[1], "/pb.Raft/Snapshot", opts...)
	if err != nil {
		return nil, err
	}
	x := &raftSnapshotClient{stream}
	return x, nil
}

type Raft_SnapshotClient interface {
	Send(*Chunk) error
	CloseAndRecv() (*empty.Empty, error)
	grpc.ClientStream
}

type raftSnapshotClient struct {
	grpc.ClientStream
}

func (x *raftSnapshotClient) Send(m *Chunk) error {
	return x.ClientStream.SendMsg(m)
}

func (x *raftSnapshotClient) CloseAndRecv() (*empty.Empty, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(empty.Empty)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *raftClient) Join(ctx context.Context, in *raftpb.Member, opts ...grpc.CallOption) (Raft_JoinClient, error) {
	stream, err := c.cc.NewStream(ctx, &_Raft_serviceDesc.Streams[2], "/pb.Raft/Join", opts...)
	if err != nil {
		return nil, err
	}
	x := &raftJoinClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Raft_JoinClient interface {
	Recv() (*raftpb.Member, error)
	grpc.ClientStream
}

type raftJoinClient struct {
	grpc.ClientStream
}

func (x *raftJoinClient) Recv() (*raftpb.Member, error) {
	m := new(raftpb.Member)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *raftClient) PromoteMember(ctx context.Context, in *raftpb.Member, opts ...grpc.CallOption) (*empty.Empty, error) {
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, "/pb.Raft/PromoteMember", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RaftServer is the server API for Raft service.
type RaftServer interface {
	Message(Raft_MessageServer) error
	Snapshot(Raft_SnapshotServer) error
	Join(*raftpb.Member, Raft_JoinServer) error
	PromoteMember(context.Context, *raftpb.Member) (*empty.Empty, error)
}

// UnimplementedRaftServer can be embedded to have forward compatible implementations.
type UnimplementedRaftServer struct {
}

func (*UnimplementedRaftServer) Message(srv Raft_MessageServer) error {
	return status.Errorf(codes.Unimplemented, "method Message not implemented")
}
func (*UnimplementedRaftServer) Snapshot(srv Raft_SnapshotServer) error {
	return status.Errorf(codes.Unimplemented, "method Snapshot not implemented")
}
func (*UnimplementedRaftServer) Join(req *raftpb.Member, srv Raft_JoinServer) error {
	return status.Errorf(codes.Unimplemented, "method Join not implemented")
}
func (*UnimplementedRaftServer) PromoteMember(ctx context.Context, req *raftpb.Member) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PromoteMember not implemented")
}

func RegisterRaftServer(s *grpc.Server, srv RaftServer) {
	s.RegisterService(&_Raft_serviceDesc, srv)
}

func _Raft_Message_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(RaftServer).Message(&raftMessageServer{stream})
}

type Raft_MessageServer interface {
	SendAndClose(*empty.Empty) error
	Recv() (*Chunk, error)
	grpc.ServerStream
}

type raftMessageServer struct {
	grpc.ServerStream
}

func (x *raftMessageServer) SendAndClose(m *empty.Empty) error {
	return x.ServerStream.SendMsg(m)
}

func (x *raftMessageServer) Recv() (*Chunk, error) {
	m := new(Chunk)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _Raft_Snapshot_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(RaftServer).Snapshot(&raftSnapshotServer{stream})
}

type Raft_SnapshotServer interface {
	SendAndClose(*empty.Empty) error
	Recv() (*Chunk, error)
	grpc.ServerStream
}

type raftSnapshotServer struct {
	grpc.ServerStream
}

func (x *raftSnapshotServer) SendAndClose(m *empty.Empty) error {
	return x.ServerStream.SendMsg(m)
}

func (x *raftSnapshotServer) Recv() (*Chunk, error) {
	m := new(Chunk)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _Raft_Join_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(raftpb.Member)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(RaftServer).Join(m, &raftJoinServer{stream})
}

type Raft_JoinServer interface {
	Send(*raftpb.Member) error
	grpc.ServerStream
}

type raftJoinServer struct {
	grpc.ServerStream
}

func (x *raftJoinServer) Send(m *raftpb.Member) error {
	return x.ServerStream.SendMsg(m)
}

func _Raft_PromoteMember_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(raftpb.Member)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RaftServer).PromoteMember(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.Raft/PromoteMember",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RaftServer).PromoteMember(ctx, req.(*raftpb.Member))
	}
	return interceptor(ctx, in, info, handler)
}

var _Raft_serviceDesc = grpc.ServiceDesc{
	ServiceName: "pb.Raft",
	HandlerType: (*RaftServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "PromoteMember",
			Handler:    _Raft_PromoteMember_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Message",
			Handler:       _Raft_Message_Handler,
			ClientStreams: true,
		},
		{
			StreamName:    "Snapshot",
			Handler:       _Raft_Snapshot_Handler,
			ClientStreams: true,
		},
		{
			StreamName:    "Join",
			Handler:       _Raft_Join_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "internal/transport/grpc/pb/raft.proto",
}

func (m *Chunk) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Chunk) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Chunk) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if len(m.Data) > 0 {
		i -= len(m.Data)
		copy(dAtA[i:], m.Data)
		i = encodeVarintRaft(dAtA, i, uint64(len(m.Data)))
		i--
		dAtA[i] = 0x12
	}
	if m.Index != 0 {
		i = encodeVarintRaft(dAtA, i, uint64(m.Index))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func encodeVarintRaft(dAtA []byte, offset int, v uint64) int {
	offset -= sovRaft(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *Chunk) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Index != 0 {
		n += 1 + sovRaft(uint64(m.Index))
	}
	l = len(m.Data)
	if l > 0 {
		n += 1 + l + sovRaft(uint64(l))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovRaft(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozRaft(x uint64) (n int) {
	return sovRaft(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *Chunk) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowRaft
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Chunk: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Chunk: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Index", wireType)
			}
			m.Index = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowRaft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Index |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Data", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowRaft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthRaft
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthRaft
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Data = append(m.Data[:0], dAtA[iNdEx:postIndex]...)
			if m.Data == nil {
				m.Data = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipRaft(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthRaft
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipRaft(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowRaft
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowRaft
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowRaft
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthRaft
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupRaft
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthRaft
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthRaft        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowRaft          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupRaft = fmt.Errorf("proto: unexpected end of group")
)