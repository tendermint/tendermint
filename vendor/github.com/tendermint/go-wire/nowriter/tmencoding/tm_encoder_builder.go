package tmencoding

import "time"

// Allow chaining builder pattern syntactic sugar variation of Facade
// teb.EncodeInt8(42).EncodeInt32(my_integer).Bytes()
type TMEncoderBuilder interface {
	Bytes() []byte
	EncodeBool(b bool) TMEncoderBuilder
	EncodeFloat32(f float32) TMEncoderBuilder
	EncodeFloat64(f float64) TMEncoderBuilder
	EncodeInt8(i int8) TMEncoderBuilder
	EncodeInt16(i int16) TMEncoderBuilder
	EncodeInt32(i int32) TMEncoderBuilder
	EncodeInt64(i int64) TMEncoderBuilder
	EncodeOctet(b byte) TMEncoderBuilder
	EncodeOctets(b []byte) TMEncoderBuilder
	EncodeTime(t time.Time) TMEncoderBuilder
	EncodeUint8(i uint8) TMEncoderBuilder
	EncodeUint16s(iz []uint16) TMEncoderBuilder
	EncodeUint32(i uint32) TMEncoderBuilder
	EncodeUint64(i uint64) TMEncoderBuilder
	EncodeUvarint(i uint) TMEncoderBuilder
	EncodeVarint(i int) TMEncoderBuilder
}

// Ensure chaining syntax functions properly with compile-time assertion.
func sugar_assertion_do_not_call(t TMEncoderBuilder) int {
	var confirm_syntax []byte
	confirm_syntax = t.EncodeBool(false).EncodeUint64(17).Bytes()
	return len(confirm_syntax)
}
