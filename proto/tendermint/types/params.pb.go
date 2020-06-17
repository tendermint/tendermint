// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: tendermint/types/params.proto

package types

import (
	fmt "fmt"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
	github_com_gogo_protobuf_types "github.com/gogo/protobuf/types"
	_ "github.com/golang/protobuf/ptypes/duration"
	io "io"
	math "math"
	math_bits "math/bits"
	time "time"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf
var _ = time.Kitchen

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

// ConsensusParams contains consensus critical parameters that determine the
// validity of blocks.
type ConsensusParams struct {
	Block     BlockParams     `protobuf:"bytes,1,opt,name=block,proto3" json:"block"`
	Evidence  EvidenceParams  `protobuf:"bytes,2,opt,name=evidence,proto3" json:"evidence"`
	Validator ValidatorParams `protobuf:"bytes,3,opt,name=validator,proto3" json:"validator"`
}

func (m *ConsensusParams) Reset()         { *m = ConsensusParams{} }
func (m *ConsensusParams) String() string { return proto.CompactTextString(m) }
func (*ConsensusParams) ProtoMessage()    {}
func (*ConsensusParams) Descriptor() ([]byte, []int) {
	return fileDescriptor_e12598271a686f57, []int{0}
}
func (m *ConsensusParams) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *ConsensusParams) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_ConsensusParams.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *ConsensusParams) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ConsensusParams.Merge(m, src)
}
func (m *ConsensusParams) XXX_Size() int {
	return m.Size()
}
func (m *ConsensusParams) XXX_DiscardUnknown() {
	xxx_messageInfo_ConsensusParams.DiscardUnknown(m)
}

var xxx_messageInfo_ConsensusParams proto.InternalMessageInfo

func (m *ConsensusParams) GetBlock() BlockParams {
	if m != nil {
		return m.Block
	}
	return BlockParams{}
}

func (m *ConsensusParams) GetEvidence() EvidenceParams {
	if m != nil {
		return m.Evidence
	}
	return EvidenceParams{}
}

func (m *ConsensusParams) GetValidator() ValidatorParams {
	if m != nil {
		return m.Validator
	}
	return ValidatorParams{}
}

// BlockParams contains limits on the block size.
type BlockParams struct {
	// Note: must be greater than 0
	MaxBytes int64 `protobuf:"varint,1,opt,name=max_bytes,json=maxBytes,proto3" json:"max_bytes,omitempty"`
	// Note: must be greater or equal to -1
	MaxGas int64 `protobuf:"varint,2,opt,name=max_gas,json=maxGas,proto3" json:"max_gas,omitempty"`
	// Minimum time increment between consecutive blocks (in milliseconds)
	// Not exposed to the application.
	TimeIotaMs int64 `protobuf:"varint,3,opt,name=time_iota_ms,json=timeIotaMs,proto3" json:"time_iota_ms,omitempty"`
}

func (m *BlockParams) Reset()         { *m = BlockParams{} }
func (m *BlockParams) String() string { return proto.CompactTextString(m) }
func (*BlockParams) ProtoMessage()    {}
func (*BlockParams) Descriptor() ([]byte, []int) {
	return fileDescriptor_e12598271a686f57, []int{1}
}
func (m *BlockParams) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *BlockParams) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_BlockParams.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *BlockParams) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BlockParams.Merge(m, src)
}
func (m *BlockParams) XXX_Size() int {
	return m.Size()
}
func (m *BlockParams) XXX_DiscardUnknown() {
	xxx_messageInfo_BlockParams.DiscardUnknown(m)
}

var xxx_messageInfo_BlockParams proto.InternalMessageInfo

func (m *BlockParams) GetMaxBytes() int64 {
	if m != nil {
		return m.MaxBytes
	}
	return 0
}

func (m *BlockParams) GetMaxGas() int64 {
	if m != nil {
		return m.MaxGas
	}
	return 0
}

func (m *BlockParams) GetTimeIotaMs() int64 {
	if m != nil {
		return m.TimeIotaMs
	}
	return 0
}

// EvidenceParams determine how we handle evidence of malfeasance.
type EvidenceParams struct {
	// Max age of evidence, in blocks.
	//
	// The basic formula for calculating this is: MaxAgeDuration / {average block
	// time}.
	MaxAgeNumBlocks int64 `protobuf:"varint,1,opt,name=max_age_num_blocks,json=maxAgeNumBlocks,proto3" json:"max_age_num_blocks,omitempty"`
	// Max age of evidence, in time.
	//
	// It should correspond with an app's "unbonding period" or other similar
	// mechanism for handling [Nothing-At-Stake
	// attacks](https://github.com/ethereum/wiki/wiki/Proof-of-Stake-FAQ#what-is-the-nothing-at-stake-problem-and-how-can-it-be-fixed).
	MaxAgeDuration time.Duration `protobuf:"bytes,2,opt,name=max_age_duration,json=maxAgeDuration,proto3,stdduration" json:"max_age_duration"`
	// This sets the maximum number of evidence that can be committed in a single block.
	// and should fall comfortably under the max block bytes when we consider the size of
	// each evidence (See MaxEvidenceBytes). The maximum number is MaxEvidencePerBlock.
	// Default is 50
	MaxNum uint32 `protobuf:"varint,3,opt,name=max_num,json=maxNum,proto3" json:"max_num,omitempty"`
	// Proof trial period dictates the time given for nodes accused of amnesia evidence, incorrectly
	// voting twice in two different rounds to respond with their respective proofs.
	// Default is half the max age in blocks: 50,000
	ProofTrialPeriod int64 `protobuf:"varint,4,opt,name=proof_trial_period,json=proofTrialPeriod,proto3" json:"proof_trial_period,omitempty"`
}

func (m *EvidenceParams) Reset()         { *m = EvidenceParams{} }
func (m *EvidenceParams) String() string { return proto.CompactTextString(m) }
func (*EvidenceParams) ProtoMessage()    {}
func (*EvidenceParams) Descriptor() ([]byte, []int) {
	return fileDescriptor_e12598271a686f57, []int{2}
}
func (m *EvidenceParams) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *EvidenceParams) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_EvidenceParams.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *EvidenceParams) XXX_Merge(src proto.Message) {
	xxx_messageInfo_EvidenceParams.Merge(m, src)
}
func (m *EvidenceParams) XXX_Size() int {
	return m.Size()
}
func (m *EvidenceParams) XXX_DiscardUnknown() {
	xxx_messageInfo_EvidenceParams.DiscardUnknown(m)
}

var xxx_messageInfo_EvidenceParams proto.InternalMessageInfo

func (m *EvidenceParams) GetMaxAgeNumBlocks() int64 {
	if m != nil {
		return m.MaxAgeNumBlocks
	}
	return 0
}

func (m *EvidenceParams) GetMaxAgeDuration() time.Duration {
	if m != nil {
		return m.MaxAgeDuration
	}
	return 0
}

func (m *EvidenceParams) GetMaxNum() uint32 {
	if m != nil {
		return m.MaxNum
	}
	return 0
}

func (m *EvidenceParams) GetProofTrialPeriod() int64 {
	if m != nil {
		return m.ProofTrialPeriod
	}
	return 0
}

// ValidatorParams restrict the public key types validators can use.
// NOTE: uses ABCI pubkey naming, not Amino names.
type ValidatorParams struct {
	PubKeyTypes []string `protobuf:"bytes,1,rep,name=pub_key_types,json=pubKeyTypes,proto3" json:"pub_key_types,omitempty"`
}

func (m *ValidatorParams) Reset()         { *m = ValidatorParams{} }
func (m *ValidatorParams) String() string { return proto.CompactTextString(m) }
func (*ValidatorParams) ProtoMessage()    {}
func (*ValidatorParams) Descriptor() ([]byte, []int) {
	return fileDescriptor_e12598271a686f57, []int{3}
}
func (m *ValidatorParams) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *ValidatorParams) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_ValidatorParams.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *ValidatorParams) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ValidatorParams.Merge(m, src)
}
func (m *ValidatorParams) XXX_Size() int {
	return m.Size()
}
func (m *ValidatorParams) XXX_DiscardUnknown() {
	xxx_messageInfo_ValidatorParams.DiscardUnknown(m)
}

var xxx_messageInfo_ValidatorParams proto.InternalMessageInfo

func (m *ValidatorParams) GetPubKeyTypes() []string {
	if m != nil {
		return m.PubKeyTypes
	}
	return nil
}

// HashedParams is a subset of ConsensusParams.
// It is amino encoded and hashed into
// the Header.ConsensusHash.
type HashedParams struct {
	BlockMaxBytes int64 `protobuf:"varint,1,opt,name=block_max_bytes,json=blockMaxBytes,proto3" json:"block_max_bytes,omitempty"`
	BlockMaxGas   int64 `protobuf:"varint,2,opt,name=block_max_gas,json=blockMaxGas,proto3" json:"block_max_gas,omitempty"`
}

func (m *HashedParams) Reset()         { *m = HashedParams{} }
func (m *HashedParams) String() string { return proto.CompactTextString(m) }
func (*HashedParams) ProtoMessage()    {}
func (*HashedParams) Descriptor() ([]byte, []int) {
	return fileDescriptor_e12598271a686f57, []int{4}
}
func (m *HashedParams) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *HashedParams) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_HashedParams.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *HashedParams) XXX_Merge(src proto.Message) {
	xxx_messageInfo_HashedParams.Merge(m, src)
}
func (m *HashedParams) XXX_Size() int {
	return m.Size()
}
func (m *HashedParams) XXX_DiscardUnknown() {
	xxx_messageInfo_HashedParams.DiscardUnknown(m)
}

var xxx_messageInfo_HashedParams proto.InternalMessageInfo

func (m *HashedParams) GetBlockMaxBytes() int64 {
	if m != nil {
		return m.BlockMaxBytes
	}
	return 0
}

func (m *HashedParams) GetBlockMaxGas() int64 {
	if m != nil {
		return m.BlockMaxGas
	}
	return 0
}

func init() {
	proto.RegisterType((*ConsensusParams)(nil), "tendermint.types.ConsensusParams")
	proto.RegisterType((*BlockParams)(nil), "tendermint.types.BlockParams")
	proto.RegisterType((*EvidenceParams)(nil), "tendermint.types.EvidenceParams")
	proto.RegisterType((*ValidatorParams)(nil), "tendermint.types.ValidatorParams")
	proto.RegisterType((*HashedParams)(nil), "tendermint.types.HashedParams")
}

func init() { proto.RegisterFile("tendermint/types/params.proto", fileDescriptor_e12598271a686f57) }

var fileDescriptor_e12598271a686f57 = []byte{
	// 528 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x64, 0x53, 0x41, 0x6f, 0xd3, 0x30,
	0x14, 0xae, 0xe9, 0x18, 0x9d, 0xbb, 0xae, 0x95, 0x85, 0x44, 0x18, 0x5a, 0x5a, 0x72, 0x40, 0x93,
	0x40, 0x89, 0x04, 0x07, 0x04, 0x17, 0x44, 0x60, 0x1a, 0x08, 0x75, 0x9a, 0xa2, 0xc1, 0x61, 0x17,
	0xcb, 0x69, 0xbc, 0x2c, 0x5a, 0x1d, 0x47, 0xb1, 0x3d, 0xb5, 0xff, 0x82, 0x23, 0xc7, 0x1d, 0xf9,
	0x09, 0xfc, 0x84, 0x1d, 0x27, 0x71, 0x80, 0x13, 0xa0, 0xf6, 0xc2, 0xcf, 0x40, 0x79, 0x69, 0xc8,
	0xda, 0xdd, 0xec, 0xf7, 0x7d, 0xef, 0xf3, 0xf7, 0xde, 0x27, 0xe3, 0x1d, 0xcd, 0xd3, 0x88, 0xe7,
	0x22, 0x49, 0xb5, 0xa7, 0xa7, 0x19, 0x57, 0x5e, 0xc6, 0x72, 0x26, 0x94, 0x9b, 0xe5, 0x52, 0x4b,
	0xd2, 0xab, 0x61, 0x17, 0xe0, 0xed, 0xbb, 0xb1, 0x8c, 0x25, 0x80, 0x5e, 0x71, 0x2a, 0x79, 0xdb,
	0x76, 0x2c, 0x65, 0x3c, 0xe6, 0x1e, 0xdc, 0x42, 0x73, 0xe2, 0x45, 0x26, 0x67, 0x3a, 0x91, 0x69,
	0x89, 0x3b, 0x3f, 0x10, 0xee, 0xbe, 0x91, 0xa9, 0xe2, 0xa9, 0x32, 0xea, 0x10, 0x5e, 0x20, 0x2f,
	0xf0, 0xed, 0x70, 0x2c, 0x47, 0x67, 0x16, 0x1a, 0xa0, 0xdd, 0xf6, 0xd3, 0x1d, 0x77, 0xf5, 0x2d,
	0xd7, 0x2f, 0xe0, 0x92, 0xed, 0xaf, 0x5d, 0xfe, 0xea, 0x37, 0x82, 0xb2, 0x83, 0xf8, 0xb8, 0xc5,
	0xcf, 0x93, 0x88, 0xa7, 0x23, 0x6e, 0xdd, 0x82, 0xee, 0xc1, 0xcd, 0xee, 0xbd, 0x05, 0x63, 0x49,
	0xe0, 0x7f, 0x1f, 0xd9, 0xc3, 0x1b, 0xe7, 0x6c, 0x9c, 0x44, 0x4c, 0xcb, 0xdc, 0x6a, 0x82, 0xc8,
	0xc3, 0x9b, 0x22, 0x9f, 0x2a, 0xca, 0x92, 0x4a, 0xdd, 0xe9, 0x70, 0xdc, 0xbe, 0x66, 0x93, 0x3c,
	0xc0, 0x1b, 0x82, 0x4d, 0x68, 0x38, 0xd5, 0x5c, 0xc1, 0x60, 0xcd, 0xa0, 0x25, 0xd8, 0xc4, 0x2f,
	0xee, 0xe4, 0x1e, 0xbe, 0x53, 0x80, 0x31, 0x53, 0xe0, 0xba, 0x19, 0xac, 0x0b, 0x36, 0xd9, 0x67,
	0x8a, 0x0c, 0xf0, 0xa6, 0x4e, 0x04, 0xa7, 0x89, 0xd4, 0x8c, 0x0a, 0x05, 0x76, 0x9a, 0x01, 0x2e,
	0x6a, 0xef, 0xa5, 0x66, 0x43, 0xe5, 0x7c, 0x47, 0x78, 0x6b, 0x79, 0x20, 0xf2, 0x18, 0x93, 0x42,
	0x8d, 0xc5, 0x9c, 0xa6, 0x46, 0x50, 0xd8, 0x4c, 0xf5, 0x66, 0x57, 0xb0, 0xc9, 0xeb, 0x98, 0x1f,
	0x18, 0x01, 0xe6, 0x14, 0x19, 0xe2, 0x5e, 0x45, 0xae, 0xa2, 0x59, 0x6c, 0xee, 0xbe, 0x5b, 0x66,
	0xe7, 0x56, 0xd9, 0xb9, 0x6f, 0x17, 0x04, 0xbf, 0x55, 0x0c, 0xfb, 0xe5, 0x77, 0x1f, 0x05, 0x5b,
	0xa5, 0x5e, 0x85, 0x54, 0x93, 0xa4, 0x46, 0x80, 0xd7, 0x0e, 0x4c, 0x72, 0x60, 0x04, 0x79, 0x82,
	0x49, 0x96, 0x4b, 0x79, 0x42, 0x75, 0x9e, 0xb0, 0x31, 0xcd, 0x78, 0x9e, 0xc8, 0xc8, 0x5a, 0x03,
	0x53, 0x3d, 0x40, 0x8e, 0x0a, 0xe0, 0x10, 0xea, 0xce, 0x2b, 0xdc, 0x5d, 0x59, 0x30, 0x71, 0x70,
	0x27, 0x33, 0x21, 0x3d, 0xe3, 0x53, 0x0a, 0x09, 0x58, 0x68, 0xd0, 0xdc, 0xdd, 0x08, 0xda, 0x99,
	0x09, 0x3f, 0xf0, 0xe9, 0x51, 0x51, 0x7a, 0xd9, 0xfa, 0x76, 0xd1, 0x47, 0x7f, 0x2f, 0xfa, 0xc8,
	0x39, 0xc6, 0x9b, 0xef, 0x98, 0x3a, 0xe5, 0xd1, 0xa2, 0xfb, 0x11, 0xee, 0xc2, 0x1e, 0xe8, 0x6a,
	0x08, 0x1d, 0x28, 0x0f, 0xab, 0x24, 0x1c, 0xdc, 0xa9, 0x79, 0x75, 0x1e, 0xed, 0x8a, 0xb5, 0xcf,
	0x94, 0xff, 0xf1, 0xeb, 0xcc, 0x46, 0x97, 0x33, 0x1b, 0x5d, 0xcd, 0x6c, 0xf4, 0x67, 0x66, 0xa3,
	0xcf, 0x73, 0xbb, 0x71, 0x35, 0xb7, 0x1b, 0x3f, 0xe7, 0x76, 0xe3, 0xf8, 0x79, 0x9c, 0xe8, 0x53,
	0x13, 0xba, 0x23, 0x29, 0xbc, 0xeb, 0x7f, 0xa8, 0x3e, 0x96, 0x9f, 0x64, 0xf5, 0x7f, 0x85, 0xeb,
	0x50, 0x7f, 0xf6, 0x2f, 0x00, 0x00, 0xff, 0xff, 0x47, 0xb8, 0x4b, 0xd4, 0x7a, 0x03, 0x00, 0x00,
}

func (this *ConsensusParams) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*ConsensusParams)
	if !ok {
		that2, ok := that.(ConsensusParams)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if !this.Block.Equal(&that1.Block) {
		return false
	}
	if !this.Evidence.Equal(&that1.Evidence) {
		return false
	}
	if !this.Validator.Equal(&that1.Validator) {
		return false
	}
	return true
}
func (this *BlockParams) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*BlockParams)
	if !ok {
		that2, ok := that.(BlockParams)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if this.MaxBytes != that1.MaxBytes {
		return false
	}
	if this.MaxGas != that1.MaxGas {
		return false
	}
	if this.TimeIotaMs != that1.TimeIotaMs {
		return false
	}
	return true
}
func (this *EvidenceParams) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*EvidenceParams)
	if !ok {
		that2, ok := that.(EvidenceParams)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if this.MaxAgeNumBlocks != that1.MaxAgeNumBlocks {
		return false
	}
	if this.MaxAgeDuration != that1.MaxAgeDuration {
		return false
	}
	if this.MaxNum != that1.MaxNum {
		return false
	}
	if this.ProofTrialPeriod != that1.ProofTrialPeriod {
		return false
	}
	return true
}
func (this *ValidatorParams) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*ValidatorParams)
	if !ok {
		that2, ok := that.(ValidatorParams)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if len(this.PubKeyTypes) != len(that1.PubKeyTypes) {
		return false
	}
	for i := range this.PubKeyTypes {
		if this.PubKeyTypes[i] != that1.PubKeyTypes[i] {
			return false
		}
	}
	return true
}
func (this *HashedParams) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*HashedParams)
	if !ok {
		that2, ok := that.(HashedParams)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if this.BlockMaxBytes != that1.BlockMaxBytes {
		return false
	}
	if this.BlockMaxGas != that1.BlockMaxGas {
		return false
	}
	return true
}
func (m *ConsensusParams) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ConsensusParams) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *ConsensusParams) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	{
		size, err := m.Validator.MarshalToSizedBuffer(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarintParams(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0x1a
	{
		size, err := m.Evidence.MarshalToSizedBuffer(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarintParams(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0x12
	{
		size, err := m.Block.MarshalToSizedBuffer(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarintParams(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0xa
	return len(dAtA) - i, nil
}

func (m *BlockParams) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *BlockParams) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *BlockParams) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.TimeIotaMs != 0 {
		i = encodeVarintParams(dAtA, i, uint64(m.TimeIotaMs))
		i--
		dAtA[i] = 0x18
	}
	if m.MaxGas != 0 {
		i = encodeVarintParams(dAtA, i, uint64(m.MaxGas))
		i--
		dAtA[i] = 0x10
	}
	if m.MaxBytes != 0 {
		i = encodeVarintParams(dAtA, i, uint64(m.MaxBytes))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *EvidenceParams) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *EvidenceParams) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *EvidenceParams) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.ProofTrialPeriod != 0 {
		i = encodeVarintParams(dAtA, i, uint64(m.ProofTrialPeriod))
		i--
		dAtA[i] = 0x20
	}
	if m.MaxNum != 0 {
		i = encodeVarintParams(dAtA, i, uint64(m.MaxNum))
		i--
		dAtA[i] = 0x18
	}
	n4, err4 := github_com_gogo_protobuf_types.StdDurationMarshalTo(m.MaxAgeDuration, dAtA[i-github_com_gogo_protobuf_types.SizeOfStdDuration(m.MaxAgeDuration):])
	if err4 != nil {
		return 0, err4
	}
	i -= n4
	i = encodeVarintParams(dAtA, i, uint64(n4))
	i--
	dAtA[i] = 0x12
	if m.MaxAgeNumBlocks != 0 {
		i = encodeVarintParams(dAtA, i, uint64(m.MaxAgeNumBlocks))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *ValidatorParams) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ValidatorParams) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *ValidatorParams) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.PubKeyTypes) > 0 {
		for iNdEx := len(m.PubKeyTypes) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.PubKeyTypes[iNdEx])
			copy(dAtA[i:], m.PubKeyTypes[iNdEx])
			i = encodeVarintParams(dAtA, i, uint64(len(m.PubKeyTypes[iNdEx])))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *HashedParams) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *HashedParams) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *HashedParams) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.BlockMaxGas != 0 {
		i = encodeVarintParams(dAtA, i, uint64(m.BlockMaxGas))
		i--
		dAtA[i] = 0x10
	}
	if m.BlockMaxBytes != 0 {
		i = encodeVarintParams(dAtA, i, uint64(m.BlockMaxBytes))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func encodeVarintParams(dAtA []byte, offset int, v uint64) int {
	offset -= sovParams(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func NewPopulatedValidatorParams(r randyParams, easy bool) *ValidatorParams {
	this := &ValidatorParams{}
	v1 := r.Intn(10)
	this.PubKeyTypes = make([]string, v1)
	for i := 0; i < v1; i++ {
		this.PubKeyTypes[i] = string(randStringParams(r))
	}
	if !easy && r.Intn(10) != 0 {
	}
	return this
}

type randyParams interface {
	Float32() float32
	Float64() float64
	Int63() int64
	Int31() int32
	Uint32() uint32
	Intn(n int) int
}

func randUTF8RuneParams(r randyParams) rune {
	ru := r.Intn(62)
	if ru < 10 {
		return rune(ru + 48)
	} else if ru < 36 {
		return rune(ru + 55)
	}
	return rune(ru + 61)
}
func randStringParams(r randyParams) string {
	v2 := r.Intn(100)
	tmps := make([]rune, v2)
	for i := 0; i < v2; i++ {
		tmps[i] = randUTF8RuneParams(r)
	}
	return string(tmps)
}
func randUnrecognizedParams(r randyParams, maxFieldNumber int) (dAtA []byte) {
	l := r.Intn(5)
	for i := 0; i < l; i++ {
		wire := r.Intn(4)
		if wire == 3 {
			wire = 5
		}
		fieldNumber := maxFieldNumber + r.Intn(100)
		dAtA = randFieldParams(dAtA, r, fieldNumber, wire)
	}
	return dAtA
}
func randFieldParams(dAtA []byte, r randyParams, fieldNumber int, wire int) []byte {
	key := uint32(fieldNumber)<<3 | uint32(wire)
	switch wire {
	case 0:
		dAtA = encodeVarintPopulateParams(dAtA, uint64(key))
		v3 := r.Int63()
		if r.Intn(2) == 0 {
			v3 *= -1
		}
		dAtA = encodeVarintPopulateParams(dAtA, uint64(v3))
	case 1:
		dAtA = encodeVarintPopulateParams(dAtA, uint64(key))
		dAtA = append(dAtA, byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)))
	case 2:
		dAtA = encodeVarintPopulateParams(dAtA, uint64(key))
		ll := r.Intn(100)
		dAtA = encodeVarintPopulateParams(dAtA, uint64(ll))
		for j := 0; j < ll; j++ {
			dAtA = append(dAtA, byte(r.Intn(256)))
		}
	default:
		dAtA = encodeVarintPopulateParams(dAtA, uint64(key))
		dAtA = append(dAtA, byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)))
	}
	return dAtA
}
func encodeVarintPopulateParams(dAtA []byte, v uint64) []byte {
	for v >= 1<<7 {
		dAtA = append(dAtA, uint8(uint64(v)&0x7f|0x80))
		v >>= 7
	}
	dAtA = append(dAtA, uint8(v))
	return dAtA
}
func (m *ConsensusParams) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = m.Block.Size()
	n += 1 + l + sovParams(uint64(l))
	l = m.Evidence.Size()
	n += 1 + l + sovParams(uint64(l))
	l = m.Validator.Size()
	n += 1 + l + sovParams(uint64(l))
	return n
}

func (m *BlockParams) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.MaxBytes != 0 {
		n += 1 + sovParams(uint64(m.MaxBytes))
	}
	if m.MaxGas != 0 {
		n += 1 + sovParams(uint64(m.MaxGas))
	}
	if m.TimeIotaMs != 0 {
		n += 1 + sovParams(uint64(m.TimeIotaMs))
	}
	return n
}

func (m *EvidenceParams) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.MaxAgeNumBlocks != 0 {
		n += 1 + sovParams(uint64(m.MaxAgeNumBlocks))
	}
	l = github_com_gogo_protobuf_types.SizeOfStdDuration(m.MaxAgeDuration)
	n += 1 + l + sovParams(uint64(l))
	if m.MaxNum != 0 {
		n += 1 + sovParams(uint64(m.MaxNum))
	}
	if m.ProofTrialPeriod != 0 {
		n += 1 + sovParams(uint64(m.ProofTrialPeriod))
	}
	return n
}

func (m *ValidatorParams) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.PubKeyTypes) > 0 {
		for _, s := range m.PubKeyTypes {
			l = len(s)
			n += 1 + l + sovParams(uint64(l))
		}
	}
	return n
}

func (m *HashedParams) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.BlockMaxBytes != 0 {
		n += 1 + sovParams(uint64(m.BlockMaxBytes))
	}
	if m.BlockMaxGas != 0 {
		n += 1 + sovParams(uint64(m.BlockMaxGas))
	}
	return n
}

func sovParams(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozParams(x uint64) (n int) {
	return sovParams(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *ConsensusParams) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowParams
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
			return fmt.Errorf("proto: ConsensusParams: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ConsensusParams: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Block", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
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
				return ErrInvalidLengthParams
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthParams
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.Block.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Evidence", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
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
				return ErrInvalidLengthParams
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthParams
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.Evidence.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Validator", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
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
				return ErrInvalidLengthParams
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthParams
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.Validator.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipParams(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthParams
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthParams
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *BlockParams) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowParams
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
			return fmt.Errorf("proto: BlockParams: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: BlockParams: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field MaxBytes", wireType)
			}
			m.MaxBytes = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.MaxBytes |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field MaxGas", wireType)
			}
			m.MaxGas = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.MaxGas |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field TimeIotaMs", wireType)
			}
			m.TimeIotaMs = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.TimeIotaMs |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipParams(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthParams
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthParams
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *EvidenceParams) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowParams
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
			return fmt.Errorf("proto: EvidenceParams: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: EvidenceParams: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field MaxAgeNumBlocks", wireType)
			}
			m.MaxAgeNumBlocks = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.MaxAgeNumBlocks |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field MaxAgeDuration", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
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
				return ErrInvalidLengthParams
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthParams
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := github_com_gogo_protobuf_types.StdDurationUnmarshal(&m.MaxAgeDuration, dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field MaxNum", wireType)
			}
			m.MaxNum = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.MaxNum |= uint32(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 4:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field ProofTrialPeriod", wireType)
			}
			m.ProofTrialPeriod = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.ProofTrialPeriod |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipParams(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthParams
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthParams
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *ValidatorParams) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowParams
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
			return fmt.Errorf("proto: ValidatorParams: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ValidatorParams: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field PubKeyTypes", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
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
				return ErrInvalidLengthParams
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthParams
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.PubKeyTypes = append(m.PubKeyTypes, string(dAtA[iNdEx:postIndex]))
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipParams(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthParams
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthParams
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *HashedParams) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowParams
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
			return fmt.Errorf("proto: HashedParams: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: HashedParams: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field BlockMaxBytes", wireType)
			}
			m.BlockMaxBytes = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.BlockMaxBytes |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field BlockMaxGas", wireType)
			}
			m.BlockMaxGas = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.BlockMaxGas |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipParams(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthParams
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthParams
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipParams(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowParams
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
					return 0, ErrIntOverflowParams
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
					return 0, ErrIntOverflowParams
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
				return 0, ErrInvalidLengthParams
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupParams
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthParams
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthParams        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowParams          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupParams = fmt.Errorf("proto: unexpected end of group")
)
