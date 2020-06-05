// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: proto/version/version.proto

package version

import (
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

// App includes the protocol and software version for the application.
// This information is included in ResponseInfo. The App.Protocol can be
// updated in ResponseEndBlock.
type App struct {
	Protocol uint64 `protobuf:"varint,1,opt,name=protocol,proto3" json:"protocol,omitempty"`
	Software string `protobuf:"bytes,2,opt,name=software,proto3" json:"software,omitempty"`
}

func (m *App) Reset()         { *m = App{} }
func (m *App) String() string { return proto.CompactTextString(m) }
func (*App) ProtoMessage()    {}
func (*App) Descriptor() ([]byte, []int) {
	return fileDescriptor_14aa2353622f11e1, []int{0}
}
func (m *App) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *App) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_App.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *App) XXX_Merge(src proto.Message) {
	xxx_messageInfo_App.Merge(m, src)
}
func (m *App) XXX_Size() int {
	return m.Size()
}
func (m *App) XXX_DiscardUnknown() {
	xxx_messageInfo_App.DiscardUnknown(m)
}

var xxx_messageInfo_App proto.InternalMessageInfo

func (m *App) GetProtocol() uint64 {
	if m != nil {
		return m.Protocol
	}
	return 0
}

func (m *App) GetSoftware() string {
	if m != nil {
		return m.Software
	}
	return ""
}

// Consensus captures the consensus rules for processing a block in the blockchain,
// including all blockchain data structures and the rules of the application's
// state transition machine.
type Consensus struct {
	Block uint64 `protobuf:"varint,1,opt,name=block,proto3" json:"block,omitempty"`
	App   uint64 `protobuf:"varint,2,opt,name=app,proto3" json:"app,omitempty"`
}

func (m *Consensus) Reset()         { *m = Consensus{} }
func (m *Consensus) String() string { return proto.CompactTextString(m) }
func (*Consensus) ProtoMessage()    {}
func (*Consensus) Descriptor() ([]byte, []int) {
	return fileDescriptor_14aa2353622f11e1, []int{1}
}
func (m *Consensus) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Consensus) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Consensus.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Consensus) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Consensus.Merge(m, src)
}
func (m *Consensus) XXX_Size() int {
	return m.Size()
}
func (m *Consensus) XXX_DiscardUnknown() {
	xxx_messageInfo_Consensus.DiscardUnknown(m)
}

var xxx_messageInfo_Consensus proto.InternalMessageInfo

func (m *Consensus) GetBlock() uint64 {
	if m != nil {
		return m.Block
	}
	return 0
}

func (m *Consensus) GetApp() uint64 {
	if m != nil {
		return m.App
	}
	return 0
}

func init() {
	proto.RegisterType((*App)(nil), "tendermint.proto.version.App")
	golang_proto.RegisterType((*App)(nil), "tendermint.proto.version.App")
	proto.RegisterType((*Consensus)(nil), "tendermint.proto.version.Consensus")
	golang_proto.RegisterType((*Consensus)(nil), "tendermint.proto.version.Consensus")
}

func init() { proto.RegisterFile("proto/version/version.proto", fileDescriptor_14aa2353622f11e1) }
func init() {
	golang_proto.RegisterFile("proto/version/version.proto", fileDescriptor_14aa2353622f11e1)
}

var fileDescriptor_14aa2353622f11e1 = []byte{
	// 228 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x92, 0x2e, 0x28, 0xca, 0x2f,
	0xc9, 0xd7, 0x2f, 0x4b, 0x2d, 0x2a, 0xce, 0xcc, 0xcf, 0x83, 0xd1, 0x7a, 0x60, 0x51, 0x21, 0x89,
	0x92, 0xd4, 0xbc, 0x94, 0xd4, 0xa2, 0xdc, 0xcc, 0xbc, 0x12, 0x88, 0x88, 0x1e, 0x54, 0x5e, 0x4a,
	0xad, 0x24, 0x23, 0xb3, 0x28, 0x25, 0xbe, 0x20, 0xb1, 0xa8, 0xa4, 0x52, 0x1f, 0x62, 0x44, 0x7a,
	0x7e, 0x7a, 0x3e, 0x82, 0x05, 0x51, 0xaf, 0x64, 0xcb, 0xc5, 0xec, 0x58, 0x50, 0x20, 0x24, 0xc5,
	0xc5, 0x01, 0xe6, 0x27, 0xe7, 0xe7, 0x48, 0x30, 0x2a, 0x30, 0x6a, 0xb0, 0x04, 0xc1, 0xf9, 0x20,
	0xb9, 0xe2, 0xfc, 0xb4, 0x92, 0xf2, 0xc4, 0xa2, 0x54, 0x09, 0x26, 0x05, 0x46, 0x0d, 0xce, 0x20,
	0x38, 0x5f, 0xc9, 0x92, 0x8b, 0xd3, 0x39, 0x3f, 0xaf, 0x38, 0x35, 0xaf, 0xb8, 0xb4, 0x58, 0x48,
	0x84, 0x8b, 0x35, 0x29, 0x27, 0x3f, 0x39, 0x1b, 0x6a, 0x02, 0x84, 0x23, 0x24, 0xc0, 0xc5, 0x9c,
	0x58, 0x50, 0x00, 0xd6, 0xc9, 0x12, 0x04, 0x62, 0x5a, 0xb1, 0xbc, 0x58, 0x20, 0xcf, 0xe8, 0xe4,
	0x73, 0xe2, 0x91, 0x1c, 0xe3, 0x85, 0x47, 0x72, 0x8c, 0x0f, 0x1e, 0xc9, 0x31, 0x4e, 0x78, 0x2c,
	0xc7, 0x70, 0xe0, 0xb1, 0x1c, 0xe3, 0x85, 0xc7, 0x72, 0x0c, 0x37, 0x1e, 0xcb, 0x31, 0x44, 0xe9,
	0xa5, 0x67, 0x96, 0x64, 0x94, 0x26, 0xe9, 0x25, 0xe7, 0xe7, 0xea, 0x23, 0x3c, 0x89, 0xcc, 0x44,
	0x09, 0x97, 0x24, 0x36, 0x30, 0xd7, 0x18, 0x10, 0x00, 0x00, 0xff, 0xff, 0xf6, 0x79, 0x77, 0x31,
	0x2f, 0x01, 0x00, 0x00,
}

func (this *Consensus) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Consensus)
	if !ok {
		that2, ok := that.(Consensus)
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
	if this.Block != that1.Block {
		return false
	}
	if this.App != that1.App {
		return false
	}
	return true
}
func (m *App) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *App) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *App) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Software) > 0 {
		i -= len(m.Software)
		copy(dAtA[i:], m.Software)
		i = encodeVarintVersion(dAtA, i, uint64(len(m.Software)))
		i--
		dAtA[i] = 0x12
	}
	if m.Protocol != 0 {
		i = encodeVarintVersion(dAtA, i, uint64(m.Protocol))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *Consensus) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Consensus) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Consensus) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.App != 0 {
		i = encodeVarintVersion(dAtA, i, uint64(m.App))
		i--
		dAtA[i] = 0x10
	}
	if m.Block != 0 {
		i = encodeVarintVersion(dAtA, i, uint64(m.Block))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func encodeVarintVersion(dAtA []byte, offset int, v uint64) int {
	offset -= sovVersion(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *App) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Protocol != 0 {
		n += 1 + sovVersion(uint64(m.Protocol))
	}
	l = len(m.Software)
	if l > 0 {
		n += 1 + l + sovVersion(uint64(l))
	}
	return n
}

func (m *Consensus) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Block != 0 {
		n += 1 + sovVersion(uint64(m.Block))
	}
	if m.App != 0 {
		n += 1 + sovVersion(uint64(m.App))
	}
	return n
}

func sovVersion(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozVersion(x uint64) (n int) {
	return sovVersion(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *App) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowVersion
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
			return fmt.Errorf("proto: App: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: App: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Protocol", wireType)
			}
			m.Protocol = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowVersion
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Protocol |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Software", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowVersion
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
				return ErrInvalidLengthVersion
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthVersion
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Software = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipVersion(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthVersion
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthVersion
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
func (m *Consensus) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowVersion
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
			return fmt.Errorf("proto: Consensus: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Consensus: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Block", wireType)
			}
			m.Block = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowVersion
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Block |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field App", wireType)
			}
			m.App = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowVersion
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.App |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipVersion(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthVersion
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthVersion
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
func skipVersion(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowVersion
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
					return 0, ErrIntOverflowVersion
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
					return 0, ErrIntOverflowVersion
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
				return 0, ErrInvalidLengthVersion
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupVersion
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthVersion
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthVersion        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowVersion          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupVersion = fmt.Errorf("proto: unexpected end of group")
)
