package tmencoding

import "time"

// Simplest pure interface for encoding and generally preferred.
type TMEncoder interface {
	EncodeBool(b bool) []byte
	EncodeFloat32(f float32) []byte
	EncodeFloat64(f float64) []byte
	EncodeInt8(i int8) []byte
	EncodeInt16(i int16) []byte
	EncodeInt32(i int32) []byte
	EncodeInt64(i int64) []byte
	EncodeOctet(b byte) []byte
	EncodeOctets(b []byte) []byte
	EncodeTime(t time.Time) []byte
	EncodeUint8(i uint8) []byte
	EncodeUint16(i uint16) []byte
	EncodeUint16s(iz []uint16) []byte
	EncodeUint32(i uint32) []byte
	EncodeUint64(i uint64) []byte
	EncodeUvarint(i uint) []byte
	EncodeVarint(i int) []byte
}
