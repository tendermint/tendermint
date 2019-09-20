// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: types/proto3/block.proto

package proto3

import (
	fmt "fmt"
	proto "github.com/gogo/protobuf/proto"
	math "math"
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

type PartSetHeader struct {
	Total                int32    `protobuf:"varint,1,opt,name=Total,proto3" json:"Total,omitempty"`
	Hash                 []byte   `protobuf:"bytes,2,opt,name=Hash,proto3" json:"Hash,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PartSetHeader) Reset()         { *m = PartSetHeader{} }
func (m *PartSetHeader) String() string { return proto.CompactTextString(m) }
func (*PartSetHeader) ProtoMessage()    {}
func (*PartSetHeader) Descriptor() ([]byte, []int) {
	return fileDescriptor_760f4d5ceb2a11f0, []int{0}
}
func (m *PartSetHeader) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PartSetHeader.Unmarshal(m, b)
}
func (m *PartSetHeader) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PartSetHeader.Marshal(b, m, deterministic)
}
func (m *PartSetHeader) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PartSetHeader.Merge(m, src)
}
func (m *PartSetHeader) XXX_Size() int {
	return xxx_messageInfo_PartSetHeader.Size(m)
}
func (m *PartSetHeader) XXX_DiscardUnknown() {
	xxx_messageInfo_PartSetHeader.DiscardUnknown(m)
}

var xxx_messageInfo_PartSetHeader proto.InternalMessageInfo

func (m *PartSetHeader) GetTotal() int32 {
	if m != nil {
		return m.Total
	}
	return 0
}

func (m *PartSetHeader) GetHash() []byte {
	if m != nil {
		return m.Hash
	}
	return nil
}

type BlockID struct {
	Hash                 []byte         `protobuf:"bytes,1,opt,name=Hash,proto3" json:"Hash,omitempty"`
	PartsHeader          *PartSetHeader `protobuf:"bytes,2,opt,name=PartsHeader,proto3" json:"PartsHeader,omitempty"`
	XXX_NoUnkeyedLiteral struct{}       `json:"-"`
	XXX_unrecognized     []byte         `json:"-"`
	XXX_sizecache        int32          `json:"-"`
}

func (m *BlockID) Reset()         { *m = BlockID{} }
func (m *BlockID) String() string { return proto.CompactTextString(m) }
func (*BlockID) ProtoMessage()    {}
func (*BlockID) Descriptor() ([]byte, []int) {
	return fileDescriptor_760f4d5ceb2a11f0, []int{1}
}
func (m *BlockID) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BlockID.Unmarshal(m, b)
}
func (m *BlockID) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BlockID.Marshal(b, m, deterministic)
}
func (m *BlockID) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BlockID.Merge(m, src)
}
func (m *BlockID) XXX_Size() int {
	return xxx_messageInfo_BlockID.Size(m)
}
func (m *BlockID) XXX_DiscardUnknown() {
	xxx_messageInfo_BlockID.DiscardUnknown(m)
}

var xxx_messageInfo_BlockID proto.InternalMessageInfo

func (m *BlockID) GetHash() []byte {
	if m != nil {
		return m.Hash
	}
	return nil
}

func (m *BlockID) GetPartsHeader() *PartSetHeader {
	if m != nil {
		return m.PartsHeader
	}
	return nil
}

type Header struct {
	// basic block info
	Version  *Version   `protobuf:"bytes,1,opt,name=Version,proto3" json:"Version,omitempty"`
	ChainID  string     `protobuf:"bytes,2,opt,name=ChainID,proto3" json:"ChainID,omitempty"`
	Height   int64      `protobuf:"varint,3,opt,name=Height,proto3" json:"Height,omitempty"`
	Time     *Timestamp `protobuf:"bytes,4,opt,name=Time,proto3" json:"Time,omitempty"`
	NumTxs   int64      `protobuf:"varint,5,opt,name=NumTxs,proto3" json:"NumTxs,omitempty"`
	TotalTxs int64      `protobuf:"varint,6,opt,name=TotalTxs,proto3" json:"TotalTxs,omitempty"`
	// prev block info
	LastBlockID *BlockID `protobuf:"bytes,7,opt,name=LastBlockID,proto3" json:"LastBlockID,omitempty"`
	// hashes of block data
	LastCommitHash []byte `protobuf:"bytes,8,opt,name=LastCommitHash,proto3" json:"LastCommitHash,omitempty"`
	DataHash       []byte `protobuf:"bytes,9,opt,name=DataHash,proto3" json:"DataHash,omitempty"`
	// hashes from the app output from the prev block
	ValidatorsHash     []byte `protobuf:"bytes,10,opt,name=ValidatorsHash,proto3" json:"ValidatorsHash,omitempty"`
	NextValidatorsHash []byte `protobuf:"bytes,11,opt,name=NextValidatorsHash,proto3" json:"NextValidatorsHash,omitempty"`
	ConsensusHash      []byte `protobuf:"bytes,12,opt,name=ConsensusHash,proto3" json:"ConsensusHash,omitempty"`
	AppHash            []byte `protobuf:"bytes,13,opt,name=AppHash,proto3" json:"AppHash,omitempty"`
	LastResultsHash    []byte `protobuf:"bytes,14,opt,name=LastResultsHash,proto3" json:"LastResultsHash,omitempty"`
	// consensus info
	EvidenceHash         []byte   `protobuf:"bytes,15,opt,name=EvidenceHash,proto3" json:"EvidenceHash,omitempty"`
	ProposerAddress      []byte   `protobuf:"bytes,16,opt,name=ProposerAddress,proto3" json:"ProposerAddress,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Header) Reset()         { *m = Header{} }
func (m *Header) String() string { return proto.CompactTextString(m) }
func (*Header) ProtoMessage()    {}
func (*Header) Descriptor() ([]byte, []int) {
	return fileDescriptor_760f4d5ceb2a11f0, []int{2}
}
func (m *Header) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Header.Unmarshal(m, b)
}
func (m *Header) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Header.Marshal(b, m, deterministic)
}
func (m *Header) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Header.Merge(m, src)
}
func (m *Header) XXX_Size() int {
	return xxx_messageInfo_Header.Size(m)
}
func (m *Header) XXX_DiscardUnknown() {
	xxx_messageInfo_Header.DiscardUnknown(m)
}

var xxx_messageInfo_Header proto.InternalMessageInfo

func (m *Header) GetVersion() *Version {
	if m != nil {
		return m.Version
	}
	return nil
}

func (m *Header) GetChainID() string {
	if m != nil {
		return m.ChainID
	}
	return ""
}

func (m *Header) GetHeight() int64 {
	if m != nil {
		return m.Height
	}
	return 0
}

func (m *Header) GetTime() *Timestamp {
	if m != nil {
		return m.Time
	}
	return nil
}

func (m *Header) GetNumTxs() int64 {
	if m != nil {
		return m.NumTxs
	}
	return 0
}

func (m *Header) GetTotalTxs() int64 {
	if m != nil {
		return m.TotalTxs
	}
	return 0
}

func (m *Header) GetLastBlockID() *BlockID {
	if m != nil {
		return m.LastBlockID
	}
	return nil
}

func (m *Header) GetLastCommitHash() []byte {
	if m != nil {
		return m.LastCommitHash
	}
	return nil
}

func (m *Header) GetDataHash() []byte {
	if m != nil {
		return m.DataHash
	}
	return nil
}

func (m *Header) GetValidatorsHash() []byte {
	if m != nil {
		return m.ValidatorsHash
	}
	return nil
}

func (m *Header) GetNextValidatorsHash() []byte {
	if m != nil {
		return m.NextValidatorsHash
	}
	return nil
}

func (m *Header) GetConsensusHash() []byte {
	if m != nil {
		return m.ConsensusHash
	}
	return nil
}

func (m *Header) GetAppHash() []byte {
	if m != nil {
		return m.AppHash
	}
	return nil
}

func (m *Header) GetLastResultsHash() []byte {
	if m != nil {
		return m.LastResultsHash
	}
	return nil
}

func (m *Header) GetEvidenceHash() []byte {
	if m != nil {
		return m.EvidenceHash
	}
	return nil
}

func (m *Header) GetProposerAddress() []byte {
	if m != nil {
		return m.ProposerAddress
	}
	return nil
}

type Version struct {
	Block                uint64   `protobuf:"varint,1,opt,name=Block,proto3" json:"Block,omitempty"`
	App                  uint64   `protobuf:"varint,2,opt,name=App,proto3" json:"App,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Version) Reset()         { *m = Version{} }
func (m *Version) String() string { return proto.CompactTextString(m) }
func (*Version) ProtoMessage()    {}
func (*Version) Descriptor() ([]byte, []int) {
	return fileDescriptor_760f4d5ceb2a11f0, []int{3}
}
func (m *Version) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Version.Unmarshal(m, b)
}
func (m *Version) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Version.Marshal(b, m, deterministic)
}
func (m *Version) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Version.Merge(m, src)
}
func (m *Version) XXX_Size() int {
	return xxx_messageInfo_Version.Size(m)
}
func (m *Version) XXX_DiscardUnknown() {
	xxx_messageInfo_Version.DiscardUnknown(m)
}

var xxx_messageInfo_Version proto.InternalMessageInfo

func (m *Version) GetBlock() uint64 {
	if m != nil {
		return m.Block
	}
	return 0
}

func (m *Version) GetApp() uint64 {
	if m != nil {
		return m.App
	}
	return 0
}

// Timestamp wraps how amino encodes time.
// This is the protobuf well-known type protobuf/timestamp.proto
// See:
// https://github.com/google/protobuf/blob/d2980062c859649523d5fd51d6b55ab310e47482/src/google/protobuf/timestamp.proto#L123-L135
// NOTE/XXX: nanos do not get skipped if they are zero in amino.
type Timestamp struct {
	Seconds              int64    `protobuf:"varint,1,opt,name=seconds,proto3" json:"seconds,omitempty"`
	Nanos                int32    `protobuf:"varint,2,opt,name=nanos,proto3" json:"nanos,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Timestamp) Reset()         { *m = Timestamp{} }
func (m *Timestamp) String() string { return proto.CompactTextString(m) }
func (*Timestamp) ProtoMessage()    {}
func (*Timestamp) Descriptor() ([]byte, []int) {
	return fileDescriptor_760f4d5ceb2a11f0, []int{4}
}
func (m *Timestamp) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Timestamp.Unmarshal(m, b)
}
func (m *Timestamp) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Timestamp.Marshal(b, m, deterministic)
}
func (m *Timestamp) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Timestamp.Merge(m, src)
}
func (m *Timestamp) XXX_Size() int {
	return xxx_messageInfo_Timestamp.Size(m)
}
func (m *Timestamp) XXX_DiscardUnknown() {
	xxx_messageInfo_Timestamp.DiscardUnknown(m)
}

var xxx_messageInfo_Timestamp proto.InternalMessageInfo

func (m *Timestamp) GetSeconds() int64 {
	if m != nil {
		return m.Seconds
	}
	return 0
}

func (m *Timestamp) GetNanos() int32 {
	if m != nil {
		return m.Nanos
	}
	return 0
}

func init() {
	proto.RegisterType((*PartSetHeader)(nil), "proto3.PartSetHeader")
	proto.RegisterType((*BlockID)(nil), "proto3.BlockID")
	proto.RegisterType((*Header)(nil), "proto3.Header")
	proto.RegisterType((*Version)(nil), "proto3.Version")
	proto.RegisterType((*Timestamp)(nil), "proto3.Timestamp")
}

func init() { proto.RegisterFile("types/proto3/block.proto", fileDescriptor_760f4d5ceb2a11f0) }

var fileDescriptor_760f4d5ceb2a11f0 = []byte{
	// 477 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x53, 0x4f, 0x6f, 0xd3, 0x30,
	0x14, 0x57, 0x68, 0xda, 0xae, 0x2f, 0xed, 0x3a, 0x2c, 0x40, 0x16, 0xa7, 0x2a, 0x02, 0x54, 0x0e,
	0xb4, 0xda, 0x76, 0x40, 0x88, 0x53, 0xd7, 0x22, 0x6d, 0x12, 0x9a, 0x26, 0x53, 0xf5, 0xc0, 0xcd,
	0x6d, 0xac, 0xd5, 0xa2, 0xb1, 0x23, 0xdb, 0x45, 0xe3, 0x13, 0xf2, 0xb5, 0x90, 0x9f, 0x93, 0xd0,
	0x44, 0xbb, 0xf9, 0xf7, 0xe7, 0xfd, 0x5e, 0xfc, 0xf2, 0x0c, 0xd4, 0xfd, 0x29, 0x84, 0x9d, 0x17,
	0x46, 0x3b, 0x7d, 0x3d, 0xdf, 0x1e, 0xf4, 0xee, 0xd7, 0x0c, 0x01, 0xe9, 0x05, 0x2e, 0xfd, 0x02,
	0xa3, 0x07, 0x6e, 0xdc, 0x0f, 0xe1, 0x6e, 0x05, 0xcf, 0x84, 0x21, 0xaf, 0xa0, 0xbb, 0xd6, 0x8e,
	0x1f, 0x68, 0x34, 0x89, 0xa6, 0x5d, 0x16, 0x00, 0x21, 0x10, 0xdf, 0x72, 0xbb, 0xa7, 0x2f, 0x26,
	0xd1, 0x74, 0xc8, 0xf0, 0x9c, 0x6e, 0xa0, 0x7f, 0xe3, 0x13, 0xef, 0x56, 0xb5, 0x1c, 0xfd, 0x97,
	0xc9, 0x67, 0x48, 0x7c, 0xb2, 0x0d, 0xb9, 0x58, 0x99, 0x5c, 0xbd, 0x0e, 0xed, 0xaf, 0x67, 0x8d,
	0xa6, 0xec, 0xd4, 0x99, 0xfe, 0x8d, 0xa1, 0x57, 0x7e, 0xcc, 0x47, 0xe8, 0x6f, 0x84, 0xb1, 0x52,
	0x2b, 0x8c, 0x4e, 0xae, 0xc6, 0x55, 0x7d, 0x49, 0xb3, 0x4a, 0x27, 0x14, 0xfa, 0xcb, 0x3d, 0x97,
	0xea, 0x6e, 0x85, 0xad, 0x06, 0xac, 0x82, 0xe4, 0x8d, 0x8f, 0x93, 0x8f, 0x7b, 0x47, 0x3b, 0x93,
	0x68, 0xda, 0x61, 0x25, 0x22, 0xef, 0x21, 0x5e, 0xcb, 0x5c, 0xd0, 0x18, 0x93, 0x5f, 0x56, 0xc9,
	0x9e, 0xb3, 0x8e, 0xe7, 0x05, 0x43, 0xd9, 0x97, 0xdf, 0x1f, 0xf3, 0xf5, 0x93, 0xa5, 0xdd, 0x50,
	0x1e, 0x10, 0x79, 0x0b, 0x67, 0x38, 0x1b, 0xaf, 0xf4, 0x50, 0xa9, 0x31, 0xb9, 0x84, 0xe4, 0x3b,
	0xb7, 0xae, 0x1c, 0x0f, 0xed, 0x37, 0xbf, 0xbd, 0xa4, 0xd9, 0xa9, 0x87, 0x7c, 0x80, 0x73, 0x0f,
	0x97, 0x3a, 0xcf, 0xa5, 0xc3, 0x61, 0x9e, 0xe1, 0x30, 0x5b, 0xac, 0x6f, 0xbb, 0xe2, 0x8e, 0xa3,
	0x63, 0x80, 0x8e, 0x1a, 0xfb, 0x8c, 0x0d, 0x3f, 0xc8, 0x8c, 0x3b, 0x6d, 0x2c, 0x3a, 0x20, 0x64,
	0x34, 0x59, 0x32, 0x03, 0x72, 0x2f, 0x9e, 0x5c, 0xcb, 0x9b, 0xa0, 0xf7, 0x19, 0x85, 0xbc, 0x83,
	0xd1, 0x52, 0x2b, 0x2b, 0x94, 0x3d, 0x06, 0xeb, 0x10, 0xad, 0x4d, 0xd2, 0xff, 0x81, 0x45, 0x51,
	0xa0, 0x3e, 0x42, 0xbd, 0x82, 0x64, 0x0a, 0x63, 0x7f, 0x0b, 0x26, 0xec, 0xf1, 0xe0, 0x42, 0xc2,
	0x39, 0x3a, 0xda, 0x34, 0x49, 0x61, 0xf8, 0xed, 0xb7, 0xcc, 0x84, 0xda, 0x09, 0xb4, 0x8d, 0xd1,
	0xd6, 0xe0, 0x7c, 0xda, 0x83, 0xd1, 0x85, 0xb6, 0xc2, 0x2c, 0xb2, 0xcc, 0x08, 0x6b, 0xe9, 0x45,
	0x48, 0x6b, 0xd1, 0xe9, 0x65, 0xbd, 0x3e, 0x7e, 0xad, 0x71, 0xd2, 0xb8, 0x47, 0x31, 0x0b, 0x80,
	0x5c, 0x40, 0x67, 0x51, 0x14, 0xb8, 0x30, 0x31, 0xf3, 0xc7, 0xf4, 0x2b, 0x0c, 0xea, 0x05, 0xf0,
	0x37, 0xb2, 0x62, 0xa7, 0x55, 0x66, 0xb1, 0xac, 0xc3, 0x2a, 0xe8, 0xe3, 0x14, 0x57, 0xda, 0x62,
	0x69, 0x97, 0x05, 0x70, 0x33, 0xff, 0xf9, 0xe9, 0x51, 0xba, 0xfd, 0x71, 0x3b, 0xdb, 0xe9, 0x7c,
	0xee, 0x84, 0xca, 0x84, 0xc9, 0xa5, 0x72, 0x8d, 0xe3, 0xc9, 0x8b, 0xdc, 0x96, 0xaf, 0xf0, 0x5f,
	0x00, 0x00, 0x00, 0xff, 0xff, 0xfe, 0x9c, 0x88, 0x53, 0xa8, 0x03, 0x00, 0x00,
}
