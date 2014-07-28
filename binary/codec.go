package binary

import (
	"io"
)

const (
	TYPE_NIL       = Byte(0x00)
	TYPE_BYTE      = Byte(0x01)
	TYPE_INT8      = Byte(0x02)
	TYPE_UINT8     = Byte(0x03)
	TYPE_INT16     = Byte(0x04)
	TYPE_UINT16    = Byte(0x05)
	TYPE_INT32     = Byte(0x06)
	TYPE_UINT32    = Byte(0x07)
	TYPE_INT64     = Byte(0x08)
	TYPE_UINT64    = Byte(0x09)
	TYPE_STRING    = Byte(0x10)
	TYPE_BYTESLICE = Byte(0x11)
	TYPE_TIME      = Byte(0x20)
)

func GetBinaryType(o Binary) Byte {
	switch o.(type) {
	case nil:
		return TYPE_NIL
	case Byte:
		return TYPE_BYTE
	case Int8:
		return TYPE_INT8
	case UInt8:
		return TYPE_UINT8
	case Int16:
		return TYPE_INT16
	case UInt16:
		return TYPE_UINT16
	case Int32:
		return TYPE_INT32
	case UInt32:
		return TYPE_UINT32
	case Int64:
		return TYPE_INT64
	case UInt64:
		return TYPE_UINT64
	case String:
		return TYPE_STRING
	case ByteSlice:
		return TYPE_BYTESLICE
	case Time:
		return TYPE_TIME
	default:
		panic("Unsupported type")
	}
}

func ReadBinaryN(r io.Reader) (o Binary, n int64) {
	type_, n_ := ReadByteN(r)
	n += n_
	switch type_ {
	case TYPE_NIL:
		o, n_ = nil, 0
	case TYPE_BYTE:
		o, n_ = ReadByteN(r)
	case TYPE_INT8:
		o, n_ = ReadInt8N(r)
	case TYPE_UINT8:
		o, n_ = ReadUInt8N(r)
	case TYPE_INT16:
		o, n_ = ReadInt16N(r)
	case TYPE_UINT16:
		o, n_ = ReadUInt16N(r)
	case TYPE_INT32:
		o, n_ = ReadInt32N(r)
	case TYPE_UINT32:
		o, n_ = ReadUInt32N(r)
	case TYPE_INT64:
		o, n_ = ReadInt64N(r)
	case TYPE_UINT64:
		o, n_ = ReadUInt64N(r)
	case TYPE_STRING:
		o, n_ = ReadStringN(r)
	case TYPE_BYTESLICE:
		o, n_ = ReadByteSliceN(r)
	case TYPE_TIME:
		o, n_ = ReadTimeN(r)
	default:
		panic("Unsupported type")
	}
	n += n_
	return o, n
}
