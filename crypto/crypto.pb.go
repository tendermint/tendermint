// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: crypto/crypto.proto

package crypto

import (
	bytes "bytes"
	fmt "fmt"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
	golang_proto "github.com/golang/protobuf/proto"
	io "io"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = golang_proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type PubKey struct {
	// Types that are valid to be assigned to Sum:
	//	*PubKey_Ed25519
	//	*PubKey_Sr25519
	//	*PubKey_Secp256K1
	Sum                  isPubKey_Sum `protobuf_oneof:"sum"`
	XXX_NoUnkeyedLiteral struct{}     `json:"-"`
	XXX_unrecognized     []byte       `json:"-"`
	XXX_sizecache        int32        `json:"-"`
}

func (m *PubKey) Reset()         { *m = PubKey{} }
func (m *PubKey) String() string { return proto.CompactTextString(m) }
func (*PubKey) ProtoMessage()    {}
func (*PubKey) Descriptor() ([]byte, []int) {
	return fileDescriptor_27e1fe69dc202ca1, []int{0}
}
func (m *PubKey) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *PubKey) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_PubKey.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *PubKey) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PubKey.Merge(m, src)
}
func (m *PubKey) XXX_Size() int {
	return m.Size()
}
func (m *PubKey) XXX_DiscardUnknown() {
	xxx_messageInfo_PubKey.DiscardUnknown(m)
}

var xxx_messageInfo_PubKey proto.InternalMessageInfo

type isPubKey_Sum interface {
	isPubKey_Sum()
	Equal(interface{}) bool
	MarshalTo([]byte) (int, error)
	Size() int
}

type PubKey_Ed25519 struct {
	Ed25519 []byte `protobuf:"bytes,1,opt,name=ed25519,proto3,oneof" json:"ed25519,omitempty"`
}
type PubKey_Sr25519 struct {
	Sr25519 []byte `protobuf:"bytes,2,opt,name=sr25519,proto3,oneof" json:"sr25519,omitempty"`
}
type PubKey_Secp256K1 struct {
	Secp256K1 []byte `protobuf:"bytes,3,opt,name=secp256k1,proto3,oneof" json:"secp256k1,omitempty"`
}

func (*PubKey_Ed25519) isPubKey_Sum()   {}
func (*PubKey_Sr25519) isPubKey_Sum()   {}
func (*PubKey_Secp256K1) isPubKey_Sum() {}

func (m *PubKey) GetSum() isPubKey_Sum {
	if m != nil {
		return m.Sum
	}
	return nil
}

func (m *PubKey) GetEd25519() []byte {
	if x, ok := m.GetSum().(*PubKey_Ed25519); ok {
		return x.Ed25519
	}
	return nil
}

func (m *PubKey) GetSr25519() []byte {
	if x, ok := m.GetSum().(*PubKey_Sr25519); ok {
		return x.Sr25519
	}
	return nil
}

func (m *PubKey) GetSecp256K1() []byte {
	if x, ok := m.GetSum().(*PubKey_Secp256K1); ok {
		return x.Secp256K1
	}
	return nil
}

// XXX_OneofWrappers is for the internal use of the proto package.
func (*PubKey) XXX_OneofWrappers() []interface{} {
	return []interface{}{
		(*PubKey_Ed25519)(nil),
		(*PubKey_Sr25519)(nil),
		(*PubKey_Secp256K1)(nil),
	}
}

type PrivKey struct {
	// Types that are valid to be assigned to Sum:
	//	*PrivKey_Ed25519
	//	*PrivKey_Sr25519
	//	*PrivKey_Secp256K1
	Sum                  isPrivKey_Sum `protobuf_oneof:"sum"`
	XXX_NoUnkeyedLiteral struct{}      `json:"-"`
	XXX_unrecognized     []byte        `json:"-"`
	XXX_sizecache        int32         `json:"-"`
}

func (m *PrivKey) Reset()         { *m = PrivKey{} }
func (m *PrivKey) String() string { return proto.CompactTextString(m) }
func (*PrivKey) ProtoMessage()    {}
func (*PrivKey) Descriptor() ([]byte, []int) {
	return fileDescriptor_27e1fe69dc202ca1, []int{1}
}
func (m *PrivKey) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *PrivKey) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_PrivKey.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *PrivKey) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PrivKey.Merge(m, src)
}
func (m *PrivKey) XXX_Size() int {
	return m.Size()
}
func (m *PrivKey) XXX_DiscardUnknown() {
	xxx_messageInfo_PrivKey.DiscardUnknown(m)
}

var xxx_messageInfo_PrivKey proto.InternalMessageInfo

type isPrivKey_Sum interface {
	isPrivKey_Sum()
	Equal(interface{}) bool
	MarshalTo([]byte) (int, error)
	Size() int
}

type PrivKey_Ed25519 struct {
	Ed25519 []byte `protobuf:"bytes,1,opt,name=ed25519,proto3,oneof" json:"ed25519,omitempty"`
}
type PrivKey_Sr25519 struct {
	Sr25519 []byte `protobuf:"bytes,2,opt,name=sr25519,proto3,oneof" json:"sr25519,omitempty"`
}
type PrivKey_Secp256K1 struct {
	Secp256K1 []byte `protobuf:"bytes,3,opt,name=secp256k1,proto3,oneof" json:"secp256k1,omitempty"`
}

func (*PrivKey_Ed25519) isPrivKey_Sum()   {}
func (*PrivKey_Sr25519) isPrivKey_Sum()   {}
func (*PrivKey_Secp256K1) isPrivKey_Sum() {}

func (m *PrivKey) GetSum() isPrivKey_Sum {
	if m != nil {
		return m.Sum
	}
	return nil
}

func (m *PrivKey) GetEd25519() []byte {
	if x, ok := m.GetSum().(*PrivKey_Ed25519); ok {
		return x.Ed25519
	}
	return nil
}

func (m *PrivKey) GetSr25519() []byte {
	if x, ok := m.GetSum().(*PrivKey_Sr25519); ok {
		return x.Sr25519
	}
	return nil
}

func (m *PrivKey) GetSecp256K1() []byte {
	if x, ok := m.GetSum().(*PrivKey_Secp256K1); ok {
		return x.Secp256K1
	}
	return nil
}

// XXX_OneofWrappers is for the internal use of the proto package.
func (*PrivKey) XXX_OneofWrappers() []interface{} {
	return []interface{}{
		(*PrivKey_Ed25519)(nil),
		(*PrivKey_Sr25519)(nil),
		(*PrivKey_Secp256K1)(nil),
	}
}

func init() {
	proto.RegisterType((*PubKey)(nil), "tendermint.crypto.crypto.PubKey")
	golang_proto.RegisterType((*PubKey)(nil), "tendermint.crypto.crypto.PubKey")
	proto.RegisterType((*PrivKey)(nil), "tendermint.crypto.crypto.PrivKey")
	golang_proto.RegisterType((*PrivKey)(nil), "tendermint.crypto.crypto.PrivKey")
}

func init() { proto.RegisterFile("crypto/crypto.proto", fileDescriptor_27e1fe69dc202ca1) }
func init() { golang_proto.RegisterFile("crypto/crypto.proto", fileDescriptor_27e1fe69dc202ca1) }

var fileDescriptor_27e1fe69dc202ca1 = []byte{
	// 215 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x12, 0x4e, 0x2e, 0xaa, 0x2c,
	0x28, 0xc9, 0xd7, 0x87, 0x50, 0x7a, 0x05, 0x45, 0xf9, 0x25, 0xf9, 0x42, 0x12, 0x25, 0xa9, 0x79,
	0x29, 0xa9, 0x45, 0xb9, 0x99, 0x79, 0x25, 0x7a, 0x50, 0x09, 0x08, 0x25, 0xa5, 0x9b, 0x9e, 0x59,
	0x92, 0x51, 0x9a, 0xa4, 0x97, 0x9c, 0x9f, 0xab, 0x9f, 0x9e, 0x9f, 0x9e, 0xaf, 0x0f, 0xd6, 0x90,
	0x54, 0x9a, 0x06, 0xe6, 0x81, 0x39, 0x60, 0x16, 0xc4, 0x20, 0xa5, 0x74, 0x2e, 0xb6, 0x80, 0xd2,
	0x24, 0xef, 0xd4, 0x4a, 0x21, 0x29, 0x2e, 0xf6, 0xd4, 0x14, 0x23, 0x53, 0x53, 0x43, 0x4b, 0x09,
	0x46, 0x05, 0x46, 0x0d, 0x1e, 0x0f, 0x86, 0x20, 0x98, 0x00, 0x48, 0xae, 0xb8, 0x08, 0x22, 0xc7,
	0x04, 0x93, 0x83, 0x0a, 0x08, 0xc9, 0x71, 0x71, 0x16, 0xa7, 0x26, 0x17, 0x18, 0x99, 0x9a, 0x65,
	0x1b, 0x4a, 0x30, 0x43, 0x65, 0x11, 0x42, 0x4e, 0xac, 0x5c, 0xcc, 0xc5, 0xa5, 0xb9, 0x4a, 0x19,
	0x5c, 0xec, 0x01, 0x45, 0x99, 0x65, 0xb4, 0xb7, 0xc9, 0xc9, 0xf5, 0xc7, 0x43, 0x39, 0xc6, 0x15,
	0x8f, 0xe4, 0x18, 0x77, 0x3c, 0x92, 0x63, 0x3c, 0xf1, 0x48, 0x8e, 0xf1, 0xc2, 0x23, 0x39, 0xc6,
	0x07, 0x8f, 0xe4, 0x18, 0x0f, 0x3c, 0x96, 0x63, 0x8c, 0x52, 0x47, 0x0a, 0x1f, 0x44, 0x20, 0x22,
	0x33, 0x21, 0x01, 0x99, 0xc4, 0x06, 0x0e, 0x20, 0x63, 0x40, 0x00, 0x00, 0x00, 0xff, 0xff, 0x88,
	0xec, 0x3d, 0xd0, 0x80, 0x01, 0x00, 0x00,
}

func (this *PubKey) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*PubKey)
	if !ok {
		that2, ok := that.(PubKey)
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
	if that1.Sum == nil {
		if this.Sum != nil {
			return false
		}
	} else if this.Sum == nil {
		return false
	} else if !this.Sum.Equal(that1.Sum) {
		return false
	}
	if !bytes.Equal(this.XXX_unrecognized, that1.XXX_unrecognized) {
		return false
	}
	return true
}
func (this *PubKey_Ed25519) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*PubKey_Ed25519)
	if !ok {
		that2, ok := that.(PubKey_Ed25519)
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
	if !bytes.Equal(this.Ed25519, that1.Ed25519) {
		return false
	}
	return true
}
func (this *PubKey_Sr25519) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*PubKey_Sr25519)
	if !ok {
		that2, ok := that.(PubKey_Sr25519)
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
	if !bytes.Equal(this.Sr25519, that1.Sr25519) {
		return false
	}
	return true
}
func (this *PubKey_Secp256K1) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*PubKey_Secp256K1)
	if !ok {
		that2, ok := that.(PubKey_Secp256K1)
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
	if !bytes.Equal(this.Secp256K1, that1.Secp256K1) {
		return false
	}
	return true
}
func (this *PrivKey) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*PrivKey)
	if !ok {
		that2, ok := that.(PrivKey)
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
	if that1.Sum == nil {
		if this.Sum != nil {
			return false
		}
	} else if this.Sum == nil {
		return false
	} else if !this.Sum.Equal(that1.Sum) {
		return false
	}
	if !bytes.Equal(this.XXX_unrecognized, that1.XXX_unrecognized) {
		return false
	}
	return true
}
func (this *PrivKey_Ed25519) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*PrivKey_Ed25519)
	if !ok {
		that2, ok := that.(PrivKey_Ed25519)
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
	if !bytes.Equal(this.Ed25519, that1.Ed25519) {
		return false
	}
	return true
}
func (this *PrivKey_Sr25519) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*PrivKey_Sr25519)
	if !ok {
		that2, ok := that.(PrivKey_Sr25519)
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
	if !bytes.Equal(this.Sr25519, that1.Sr25519) {
		return false
	}
	return true
}
func (this *PrivKey_Secp256K1) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*PrivKey_Secp256K1)
	if !ok {
		that2, ok := that.(PrivKey_Secp256K1)
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
	if !bytes.Equal(this.Secp256K1, that1.Secp256K1) {
		return false
	}
	return true
}
func (m *PubKey) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *PubKey) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *PubKey) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if m.Sum != nil {
		{
			size := m.Sum.Size()
			i -= size
			if _, err := m.Sum.MarshalTo(dAtA[i:]); err != nil {
				return 0, err
			}
		}
	}
	return len(dAtA) - i, nil
}

func (m *PubKey_Ed25519) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *PubKey_Ed25519) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	if m.Ed25519 != nil {
		i -= len(m.Ed25519)
		copy(dAtA[i:], m.Ed25519)
		i = encodeVarintCrypto(dAtA, i, uint64(len(m.Ed25519)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}
func (m *PubKey_Sr25519) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *PubKey_Sr25519) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	if m.Sr25519 != nil {
		i -= len(m.Sr25519)
		copy(dAtA[i:], m.Sr25519)
		i = encodeVarintCrypto(dAtA, i, uint64(len(m.Sr25519)))
		i--
		dAtA[i] = 0x12
	}
	return len(dAtA) - i, nil
}
func (m *PubKey_Secp256K1) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *PubKey_Secp256K1) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	if m.Secp256K1 != nil {
		i -= len(m.Secp256K1)
		copy(dAtA[i:], m.Secp256K1)
		i = encodeVarintCrypto(dAtA, i, uint64(len(m.Secp256K1)))
		i--
		dAtA[i] = 0x1a
	}
	return len(dAtA) - i, nil
}
func (m *PrivKey) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *PrivKey) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *PrivKey) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if m.Sum != nil {
		{
			size := m.Sum.Size()
			i -= size
			if _, err := m.Sum.MarshalTo(dAtA[i:]); err != nil {
				return 0, err
			}
		}
	}
	return len(dAtA) - i, nil
}

func (m *PrivKey_Ed25519) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *PrivKey_Ed25519) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	if m.Ed25519 != nil {
		i -= len(m.Ed25519)
		copy(dAtA[i:], m.Ed25519)
		i = encodeVarintCrypto(dAtA, i, uint64(len(m.Ed25519)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}
func (m *PrivKey_Sr25519) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *PrivKey_Sr25519) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	if m.Sr25519 != nil {
		i -= len(m.Sr25519)
		copy(dAtA[i:], m.Sr25519)
		i = encodeVarintCrypto(dAtA, i, uint64(len(m.Sr25519)))
		i--
		dAtA[i] = 0x12
	}
	return len(dAtA) - i, nil
}
func (m *PrivKey_Secp256K1) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *PrivKey_Secp256K1) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	if m.Secp256K1 != nil {
		i -= len(m.Secp256K1)
		copy(dAtA[i:], m.Secp256K1)
		i = encodeVarintCrypto(dAtA, i, uint64(len(m.Secp256K1)))
		i--
		dAtA[i] = 0x1a
	}
	return len(dAtA) - i, nil
}
func encodeVarintCrypto(dAtA []byte, offset int, v uint64) int {
	offset -= sovCrypto(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func NewPopulatedPubKey(r randyCrypto, easy bool) *PubKey {
	this := &PubKey{}
	oneofNumber_Sum := []int32{1, 2, 3}[r.Intn(3)]
	switch oneofNumber_Sum {
	case 1:
		this.Sum = NewPopulatedPubKey_Ed25519(r, easy)
	case 2:
		this.Sum = NewPopulatedPubKey_Sr25519(r, easy)
	case 3:
		this.Sum = NewPopulatedPubKey_Secp256K1(r, easy)
	}
	if !easy && r.Intn(10) != 0 {
		this.XXX_unrecognized = randUnrecognizedCrypto(r, 4)
	}
	return this
}

func NewPopulatedPubKey_Ed25519(r randyCrypto, easy bool) *PubKey_Ed25519 {
	this := &PubKey_Ed25519{}
	v1 := r.Intn(100)
	this.Ed25519 = make([]byte, v1)
	for i := 0; i < v1; i++ {
		this.Ed25519[i] = byte(r.Intn(256))
	}
	return this
}
func NewPopulatedPubKey_Sr25519(r randyCrypto, easy bool) *PubKey_Sr25519 {
	this := &PubKey_Sr25519{}
	v2 := r.Intn(100)
	this.Sr25519 = make([]byte, v2)
	for i := 0; i < v2; i++ {
		this.Sr25519[i] = byte(r.Intn(256))
	}
	return this
}
func NewPopulatedPubKey_Secp256K1(r randyCrypto, easy bool) *PubKey_Secp256K1 {
	this := &PubKey_Secp256K1{}
	v3 := r.Intn(100)
	this.Secp256K1 = make([]byte, v3)
	for i := 0; i < v3; i++ {
		this.Secp256K1[i] = byte(r.Intn(256))
	}
	return this
}
func NewPopulatedPrivKey(r randyCrypto, easy bool) *PrivKey {
	this := &PrivKey{}
	oneofNumber_Sum := []int32{1, 2, 3}[r.Intn(3)]
	switch oneofNumber_Sum {
	case 1:
		this.Sum = NewPopulatedPrivKey_Ed25519(r, easy)
	case 2:
		this.Sum = NewPopulatedPrivKey_Sr25519(r, easy)
	case 3:
		this.Sum = NewPopulatedPrivKey_Secp256K1(r, easy)
	}
	if !easy && r.Intn(10) != 0 {
		this.XXX_unrecognized = randUnrecognizedCrypto(r, 4)
	}
	return this
}

func NewPopulatedPrivKey_Ed25519(r randyCrypto, easy bool) *PrivKey_Ed25519 {
	this := &PrivKey_Ed25519{}
	v4 := r.Intn(100)
	this.Ed25519 = make([]byte, v4)
	for i := 0; i < v4; i++ {
		this.Ed25519[i] = byte(r.Intn(256))
	}
	return this
}
func NewPopulatedPrivKey_Sr25519(r randyCrypto, easy bool) *PrivKey_Sr25519 {
	this := &PrivKey_Sr25519{}
	v5 := r.Intn(100)
	this.Sr25519 = make([]byte, v5)
	for i := 0; i < v5; i++ {
		this.Sr25519[i] = byte(r.Intn(256))
	}
	return this
}
func NewPopulatedPrivKey_Secp256K1(r randyCrypto, easy bool) *PrivKey_Secp256K1 {
	this := &PrivKey_Secp256K1{}
	v6 := r.Intn(100)
	this.Secp256K1 = make([]byte, v6)
	for i := 0; i < v6; i++ {
		this.Secp256K1[i] = byte(r.Intn(256))
	}
	return this
}

type randyCrypto interface {
	Float32() float32
	Float64() float64
	Int63() int64
	Int31() int32
	Uint32() uint32
	Intn(n int) int
}

func randUTF8RuneCrypto(r randyCrypto) rune {
	ru := r.Intn(62)
	if ru < 10 {
		return rune(ru + 48)
	} else if ru < 36 {
		return rune(ru + 55)
	}
	return rune(ru + 61)
}
func randStringCrypto(r randyCrypto) string {
	v7 := r.Intn(100)
	tmps := make([]rune, v7)
	for i := 0; i < v7; i++ {
		tmps[i] = randUTF8RuneCrypto(r)
	}
	return string(tmps)
}
func randUnrecognizedCrypto(r randyCrypto, maxFieldNumber int) (dAtA []byte) {
	l := r.Intn(5)
	for i := 0; i < l; i++ {
		wire := r.Intn(4)
		if wire == 3 {
			wire = 5
		}
		fieldNumber := maxFieldNumber + r.Intn(100)
		dAtA = randFieldCrypto(dAtA, r, fieldNumber, wire)
	}
	return dAtA
}
func randFieldCrypto(dAtA []byte, r randyCrypto, fieldNumber int, wire int) []byte {
	key := uint32(fieldNumber)<<3 | uint32(wire)
	switch wire {
	case 0:
		dAtA = encodeVarintPopulateCrypto(dAtA, uint64(key))
		v8 := r.Int63()
		if r.Intn(2) == 0 {
			v8 *= -1
		}
		dAtA = encodeVarintPopulateCrypto(dAtA, uint64(v8))
	case 1:
		dAtA = encodeVarintPopulateCrypto(dAtA, uint64(key))
		dAtA = append(dAtA, byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)))
	case 2:
		dAtA = encodeVarintPopulateCrypto(dAtA, uint64(key))
		ll := r.Intn(100)
		dAtA = encodeVarintPopulateCrypto(dAtA, uint64(ll))
		for j := 0; j < ll; j++ {
			dAtA = append(dAtA, byte(r.Intn(256)))
		}
	default:
		dAtA = encodeVarintPopulateCrypto(dAtA, uint64(key))
		dAtA = append(dAtA, byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)), byte(r.Intn(256)))
	}
	return dAtA
}
func encodeVarintPopulateCrypto(dAtA []byte, v uint64) []byte {
	for v >= 1<<7 {
		dAtA = append(dAtA, uint8(uint64(v)&0x7f|0x80))
		v >>= 7
	}
	dAtA = append(dAtA, uint8(v))
	return dAtA
}
func (m *PubKey) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Sum != nil {
		n += m.Sum.Size()
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *PubKey_Ed25519) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Ed25519 != nil {
		l = len(m.Ed25519)
		n += 1 + l + sovCrypto(uint64(l))
	}
	return n
}
func (m *PubKey_Sr25519) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Sr25519 != nil {
		l = len(m.Sr25519)
		n += 1 + l + sovCrypto(uint64(l))
	}
	return n
}
func (m *PubKey_Secp256K1) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Secp256K1 != nil {
		l = len(m.Secp256K1)
		n += 1 + l + sovCrypto(uint64(l))
	}
	return n
}
func (m *PrivKey) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Sum != nil {
		n += m.Sum.Size()
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *PrivKey_Ed25519) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Ed25519 != nil {
		l = len(m.Ed25519)
		n += 1 + l + sovCrypto(uint64(l))
	}
	return n
}
func (m *PrivKey_Sr25519) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Sr25519 != nil {
		l = len(m.Sr25519)
		n += 1 + l + sovCrypto(uint64(l))
	}
	return n
}
func (m *PrivKey_Secp256K1) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Secp256K1 != nil {
		l = len(m.Secp256K1)
		n += 1 + l + sovCrypto(uint64(l))
	}
	return n
}

func sovCrypto(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozCrypto(x uint64) (n int) {
	return sovCrypto(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *PubKey) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowCrypto
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
			return fmt.Errorf("proto: PubKey: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: PubKey: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Ed25519", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCrypto
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
				return ErrInvalidLengthCrypto
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthCrypto
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			v := make([]byte, postIndex-iNdEx)
			copy(v, dAtA[iNdEx:postIndex])
			m.Sum = &PubKey_Ed25519{v}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Sr25519", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCrypto
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
				return ErrInvalidLengthCrypto
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthCrypto
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			v := make([]byte, postIndex-iNdEx)
			copy(v, dAtA[iNdEx:postIndex])
			m.Sum = &PubKey_Sr25519{v}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Secp256K1", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCrypto
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
				return ErrInvalidLengthCrypto
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthCrypto
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			v := make([]byte, postIndex-iNdEx)
			copy(v, dAtA[iNdEx:postIndex])
			m.Sum = &PubKey_Secp256K1{v}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipCrypto(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthCrypto
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthCrypto
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
func (m *PrivKey) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowCrypto
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
			return fmt.Errorf("proto: PrivKey: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: PrivKey: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Ed25519", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCrypto
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
				return ErrInvalidLengthCrypto
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthCrypto
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			v := make([]byte, postIndex-iNdEx)
			copy(v, dAtA[iNdEx:postIndex])
			m.Sum = &PrivKey_Ed25519{v}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Sr25519", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCrypto
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
				return ErrInvalidLengthCrypto
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthCrypto
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			v := make([]byte, postIndex-iNdEx)
			copy(v, dAtA[iNdEx:postIndex])
			m.Sum = &PrivKey_Sr25519{v}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Secp256K1", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCrypto
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
				return ErrInvalidLengthCrypto
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthCrypto
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			v := make([]byte, postIndex-iNdEx)
			copy(v, dAtA[iNdEx:postIndex])
			m.Sum = &PrivKey_Secp256K1{v}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipCrypto(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthCrypto
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthCrypto
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
func skipCrypto(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowCrypto
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
					return 0, ErrIntOverflowCrypto
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
					return 0, ErrIntOverflowCrypto
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
				return 0, ErrInvalidLengthCrypto
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupCrypto
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthCrypto
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthCrypto        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowCrypto          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupCrypto = fmt.Errorf("proto: unexpected end of group")
)
