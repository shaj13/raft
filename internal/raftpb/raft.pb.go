// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: internal/raftpb/raft.proto

package raftpb

import (
	fmt "fmt"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
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

type MemberType int32

const (
	VoterMember   MemberType = 0
	RemovedMember MemberType = 1
	LearnerMember MemberType = 2
	StagingMember MemberType = 3
	LocalMember   MemberType = 4
)

var MemberType_name = map[int32]string{
	0: "voter",
	1: "removed",
	2: "learner",
	3: "staging",
	4: "local",
}

var MemberType_value = map[string]int32{
	"voter":   0,
	"removed": 1,
	"learner": 2,
	"staging": 3,
	"local":   4,
}

func (x MemberType) String() string {
	return proto.EnumName(MemberType_name, int32(x))
}

func (MemberType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_dbd5440484cc1d7f, []int{0}
}

// Version represents the snapshot file version.
type SnapshotTrailer_Version int32

const (
	// V0 is the initial version of the snapshot file.
	V0 SnapshotTrailer_Version = 0
)

var SnapshotTrailer_Version_name = map[int32]string{
	0: "V0",
}

var SnapshotTrailer_Version_value = map[string]int32{
	"V0": 0,
}

func (x SnapshotTrailer_Version) String() string {
	return proto.EnumName(SnapshotTrailer_Version_name, int32(x))
}

func (SnapshotTrailer_Version) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_dbd5440484cc1d7f, []int{3, 0}
}

type Member struct {
	// ID specifies the cluster member id.
	ID uint64 `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	// Address specifies the address of the cluster member.
	Address string `protobuf:"bytes,2,opt,name=addr,proto3" json:"addr,omitempty"`
	// Type used to distinguish members (local, remote, etc).
	Type MemberType `protobuf:"varint,3,opt,name=type,proto3,enum=raftpb.MemberType" json:"type,omitempty"`
	// Context is treated as an opaque payload and can be used to
	// attach an extra info/data on member.
	Context              []byte   `protobuf:"bytes,4,opt,name=context,proto3" json:"context,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Member) Reset()         { *m = Member{} }
func (m *Member) String() string { return proto.CompactTextString(m) }
func (*Member) ProtoMessage()    {}
func (*Member) Descriptor() ([]byte, []int) {
	return fileDescriptor_dbd5440484cc1d7f, []int{0}
}
func (m *Member) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Member) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Member.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Member) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Member.Merge(m, src)
}
func (m *Member) XXX_Size() int {
	return m.Size()
}
func (m *Member) XXX_DiscardUnknown() {
	xxx_messageInfo_Member.DiscardUnknown(m)
}

var xxx_messageInfo_Member proto.InternalMessageInfo

type Replicate struct {
	// CID specifies the transaction change id.
	CID uint64 `protobuf:"varint,1,opt,name=cid,proto3" json:"cid,omitempty"`
	// Data specifies the raw replicate data.
	Data                 []byte   `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Replicate) Reset()         { *m = Replicate{} }
func (m *Replicate) String() string { return proto.CompactTextString(m) }
func (*Replicate) ProtoMessage()    {}
func (*Replicate) Descriptor() ([]byte, []int) {
	return fileDescriptor_dbd5440484cc1d7f, []int{1}
}
func (m *Replicate) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Replicate) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Replicate.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Replicate) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Replicate.Merge(m, src)
}
func (m *Replicate) XXX_Size() int {
	return m.Size()
}
func (m *Replicate) XXX_DiscardUnknown() {
	xxx_messageInfo_Replicate.DiscardUnknown(m)
}

var xxx_messageInfo_Replicate proto.InternalMessageInfo

// Pool specifies the the cluster pool members.
type Pool struct {
	Members              []Member `protobuf:"bytes,1,rep,name=members,proto3" json:"members"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Pool) Reset()         { *m = Pool{} }
func (m *Pool) String() string { return proto.CompactTextString(m) }
func (*Pool) ProtoMessage()    {}
func (*Pool) Descriptor() ([]byte, []int) {
	return fileDescriptor_dbd5440484cc1d7f, []int{2}
}
func (m *Pool) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Pool) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Pool.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Pool) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Pool.Merge(m, src)
}
func (m *Pool) XXX_Size() int {
	return m.Size()
}
func (m *Pool) XXX_DiscardUnknown() {
	xxx_messageInfo_Pool.DiscardUnknown(m)
}

var xxx_messageInfo_Pool proto.InternalMessageInfo

type SnapshotTrailer struct {
	// CRC specifies the snapshot crc sum.
	CRC []byte `protobuf:"bytes,1,opt,name=CRC,proto3" json:"CRC,omitempty"`
	// Version specifies the snapshot file version.
	Version SnapshotTrailer_Version `protobuf:"varint,2,opt,name=version,proto3,enum=raftpb.SnapshotTrailer_Version" json:"version,omitempty"`
	// members specifies the the cluster pool members.
	Members              []Member `protobuf:"bytes,3,rep,name=members,proto3" json:"members"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *SnapshotTrailer) Reset()         { *m = SnapshotTrailer{} }
func (m *SnapshotTrailer) String() string { return proto.CompactTextString(m) }
func (*SnapshotTrailer) ProtoMessage()    {}
func (*SnapshotTrailer) Descriptor() ([]byte, []int) {
	return fileDescriptor_dbd5440484cc1d7f, []int{3}
}
func (m *SnapshotTrailer) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *SnapshotTrailer) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_SnapshotTrailer.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *SnapshotTrailer) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SnapshotTrailer.Merge(m, src)
}
func (m *SnapshotTrailer) XXX_Size() int {
	return m.Size()
}
func (m *SnapshotTrailer) XXX_DiscardUnknown() {
	xxx_messageInfo_SnapshotTrailer.DiscardUnknown(m)
}

var xxx_messageInfo_SnapshotTrailer proto.InternalMessageInfo

func init() {
	proto.RegisterEnum("raftpb.MemberType", MemberType_name, MemberType_value)
	proto.RegisterEnum("raftpb.SnapshotTrailer_Version", SnapshotTrailer_Version_name, SnapshotTrailer_Version_value)
	proto.RegisterType((*Member)(nil), "raftpb.Member")
	proto.RegisterType((*Replicate)(nil), "raftpb.Replicate")
	proto.RegisterType((*Pool)(nil), "raftpb.Pool")
	proto.RegisterType((*SnapshotTrailer)(nil), "raftpb.SnapshotTrailer")
}

func init() { proto.RegisterFile("internal/raftpb/raft.proto", fileDescriptor_dbd5440484cc1d7f) }

var fileDescriptor_dbd5440484cc1d7f = []byte{
	// 451 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x92, 0xc1, 0x6a, 0xdb, 0x30,
	0x18, 0xc7, 0x2d, 0xdb, 0xb5, 0xe9, 0x97, 0x2c, 0x75, 0x45, 0x19, 0x9e, 0x0f, 0xb6, 0xc9, 0x61,
	0x98, 0x1d, 0xdc, 0x91, 0xc1, 0x60, 0xbb, 0x2d, 0xe9, 0xa5, 0xd0, 0xc1, 0x50, 0x4b, 0xee, 0x4a,
	0xa4, 0x65, 0x06, 0xc7, 0x32, 0xb2, 0x08, 0xeb, 0x79, 0xb7, 0xbe, 0x43, 0x0f, 0x83, 0x3d, 0xc2,
	0x4e, 0x7b, 0x82, 0x1c, 0xfb, 0x04, 0x61, 0xf5, 0x93, 0x0c, 0x4b, 0xc9, 0x92, 0xed, 0xd4, 0x93,
	0xbf, 0xff, 0xf7, 0xfb, 0xfc, 0xfd, 0xff, 0x48, 0x82, 0xa8, 0xa8, 0x14, 0x97, 0x15, 0x2d, 0xcf,
	0x25, 0xfd, 0xac, 0xea, 0x99, 0xfe, 0xe4, 0xb5, 0x14, 0x4a, 0x60, 0xcf, 0xb4, 0xa2, 0xb3, 0x85,
	0x58, 0x08, 0xdd, 0x3a, 0xef, 0x2a, 0x43, 0x87, 0xdf, 0x10, 0x78, 0x1f, 0xf9, 0x72, 0xc6, 0x25,
	0x7e, 0x0e, 0x76, 0xc1, 0x42, 0x94, 0xa2, 0xcc, 0x1d, 0x7b, 0xed, 0x26, 0xb1, 0x2f, 0x2f, 0x88,
	0x5d, 0x30, 0x9c, 0x80, 0x4b, 0x19, 0x93, 0xa1, 0x9d, 0xa2, 0xec, 0x78, 0xdc, 0x6b, 0x37, 0x89,
	0xff, 0x81, 0x31, 0xc9, 0x9b, 0x86, 0x68, 0x80, 0x5f, 0x82, 0xab, 0x6e, 0x6b, 0x1e, 0x3a, 0x29,
	0xca, 0x06, 0x23, 0x9c, 0x1b, 0xc3, 0xdc, 0xac, 0xbd, 0xb9, 0xad, 0x39, 0xd1, 0x1c, 0x87, 0xe0,
	0xcf, 0x45, 0xa5, 0xf8, 0x57, 0x15, 0xba, 0x29, 0xca, 0xfa, 0x64, 0x27, 0x87, 0xef, 0xe1, 0x98,
	0xf0, 0xba, 0x2c, 0xe6, 0x54, 0x71, 0xfc, 0x02, 0x9c, 0xf9, 0xdf, 0x20, 0x7e, 0xbb, 0x49, 0x9c,
	0xc9, 0xe5, 0x05, 0xe9, 0x7a, 0x18, 0x83, 0xcb, 0xa8, 0xa2, 0x3a, 0x4a, 0x9f, 0xe8, 0x7a, 0xf8,
	0x16, 0xdc, 0x4f, 0x42, 0x94, 0x38, 0x07, 0x7f, 0xa9, 0x1d, 0x9b, 0x10, 0xa5, 0x4e, 0xd6, 0x1b,
	0x0d, 0xfe, 0x0d, 0x32, 0x76, 0xd7, 0x9b, 0xc4, 0x22, 0xbb, 0xa1, 0xe1, 0x77, 0x04, 0x27, 0xd7,
	0x15, 0xad, 0x9b, 0x2f, 0x42, 0xdd, 0x48, 0x5a, 0x94, 0x5c, 0xe2, 0x00, 0x9c, 0x09, 0x99, 0x68,
	0xeb, 0x3e, 0xe9, 0x4a, 0xfc, 0x0e, 0xfc, 0x15, 0x97, 0x4d, 0x21, 0x2a, 0x6d, 0x3a, 0x18, 0x25,
	0xbb, 0xad, 0xff, 0xfd, 0x9b, 0x4f, 0xcd, 0x18, 0xd9, 0xcd, 0x1f, 0x06, 0x72, 0x9e, 0x12, 0xe8,
	0x14, 0xfc, 0xed, 0x0e, 0xec, 0x81, 0x3d, 0x7d, 0x1d, 0x58, 0xaf, 0x7e, 0x22, 0x80, 0xfd, 0x31,
	0xe2, 0x08, 0x8e, 0x56, 0x42, 0x71, 0x19, 0x58, 0xd1, 0xc9, 0xdd, 0x7d, 0xda, 0x9b, 0x76, 0x62,
	0x7b, 0x7b, 0x31, 0xf8, 0x92, 0x2f, 0xc5, 0x8a, 0xb3, 0x00, 0x45, 0xa7, 0x77, 0xf7, 0xe9, 0x33,
	0x62, 0xe4, 0x9e, 0x97, 0x9c, 0xca, 0x8a, 0xcb, 0xc0, 0x36, 0xfc, 0xca, 0xc8, 0x3d, 0x6f, 0x14,
	0x5d, 0x14, 0xd5, 0x22, 0x70, 0x0c, 0xbf, 0x36, 0x72, 0xcb, 0x23, 0x38, 0x2a, 0xc5, 0x9c, 0x96,
	0x81, 0x6b, 0xbc, 0xaf, 0x3a, 0x61, 0x58, 0x34, 0xf8, 0xf5, 0x23, 0x3e, 0xc8, 0x39, 0x3e, 0x5b,
	0x3f, 0xc6, 0xd6, 0xc3, 0x63, 0x6c, 0xad, 0xdb, 0x18, 0x3d, 0xb4, 0x31, 0xfa, 0xdd, 0xc6, 0x68,
	0xe6, 0xe9, 0x17, 0xf7, 0xe6, 0x4f, 0x00, 0x00, 0x00, 0xff, 0xff, 0xf2, 0x08, 0x91, 0x04, 0xad,
	0x02, 0x00, 0x00,
}

func (m *Member) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Member) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Member) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if len(m.Context) > 0 {
		i -= len(m.Context)
		copy(dAtA[i:], m.Context)
		i = encodeVarintRaft(dAtA, i, uint64(len(m.Context)))
		i--
		dAtA[i] = 0x22
	}
	if m.Type != 0 {
		i = encodeVarintRaft(dAtA, i, uint64(m.Type))
		i--
		dAtA[i] = 0x18
	}
	if len(m.Address) > 0 {
		i -= len(m.Address)
		copy(dAtA[i:], m.Address)
		i = encodeVarintRaft(dAtA, i, uint64(len(m.Address)))
		i--
		dAtA[i] = 0x12
	}
	if m.ID != 0 {
		i = encodeVarintRaft(dAtA, i, uint64(m.ID))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *Replicate) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Replicate) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Replicate) MarshalToSizedBuffer(dAtA []byte) (int, error) {
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
	if m.CID != 0 {
		i = encodeVarintRaft(dAtA, i, uint64(m.CID))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *Pool) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Pool) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Pool) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if len(m.Members) > 0 {
		for iNdEx := len(m.Members) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Members[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintRaft(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *SnapshotTrailer) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *SnapshotTrailer) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *SnapshotTrailer) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if len(m.Members) > 0 {
		for iNdEx := len(m.Members) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Members[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintRaft(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x1a
		}
	}
	if m.Version != 0 {
		i = encodeVarintRaft(dAtA, i, uint64(m.Version))
		i--
		dAtA[i] = 0x10
	}
	if len(m.CRC) > 0 {
		i -= len(m.CRC)
		copy(dAtA[i:], m.CRC)
		i = encodeVarintRaft(dAtA, i, uint64(len(m.CRC)))
		i--
		dAtA[i] = 0xa
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
func (m *Member) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.ID != 0 {
		n += 1 + sovRaft(uint64(m.ID))
	}
	l = len(m.Address)
	if l > 0 {
		n += 1 + l + sovRaft(uint64(l))
	}
	if m.Type != 0 {
		n += 1 + sovRaft(uint64(m.Type))
	}
	l = len(m.Context)
	if l > 0 {
		n += 1 + l + sovRaft(uint64(l))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *Replicate) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.CID != 0 {
		n += 1 + sovRaft(uint64(m.CID))
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

func (m *Pool) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.Members) > 0 {
		for _, e := range m.Members {
			l = e.Size()
			n += 1 + l + sovRaft(uint64(l))
		}
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *SnapshotTrailer) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.CRC)
	if l > 0 {
		n += 1 + l + sovRaft(uint64(l))
	}
	if m.Version != 0 {
		n += 1 + sovRaft(uint64(m.Version))
	}
	if len(m.Members) > 0 {
		for _, e := range m.Members {
			l = e.Size()
			n += 1 + l + sovRaft(uint64(l))
		}
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
func (m *Member) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: Member: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Member: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field ID", wireType)
			}
			m.ID = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowRaft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.ID |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Address", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowRaft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthRaft
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthRaft
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Address = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Type", wireType)
			}
			m.Type = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowRaft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Type |= MemberType(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Context", wireType)
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
			m.Context = append(m.Context[:0], dAtA[iNdEx:postIndex]...)
			if m.Context == nil {
				m.Context = []byte{}
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
func (m *Replicate) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: Replicate: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Replicate: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field CID", wireType)
			}
			m.CID = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowRaft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.CID |= uint64(b&0x7F) << shift
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
func (m *Pool) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: Pool: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Pool: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Members", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowRaft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthRaft
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthRaft
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Members = append(m.Members, Member{})
			if err := m.Members[len(m.Members)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
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
func (m *SnapshotTrailer) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: SnapshotTrailer: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: SnapshotTrailer: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field CRC", wireType)
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
			m.CRC = append(m.CRC[:0], dAtA[iNdEx:postIndex]...)
			if m.CRC == nil {
				m.CRC = []byte{}
			}
			iNdEx = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Version", wireType)
			}
			m.Version = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowRaft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Version |= SnapshotTrailer_Version(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Members", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowRaft
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthRaft
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthRaft
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Members = append(m.Members, Member{})
			if err := m.Members[len(m.Members)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
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
