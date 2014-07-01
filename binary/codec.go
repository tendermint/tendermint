package binary

import (
	"io"
)

const (
	TYPE_NIL    = Byte(0x00)
	TYPE_BYTE   = Byte(0x01)
	TYPE_INT8   = Byte(0x02)
	TYPE_UINT8  = Byte(0x03)
	TYPE_INT16  = Byte(0x04)
	TYPE_UINT16 = Byte(0x05)
	TYPE_INT32  = Byte(0x06)
	TYPE_UINT32 = Byte(0x07)
	TYPE_INT64  = Byte(0x08)
	TYPE_UINT64 = Byte(0x09)

	TYPE_STRING    = Byte(0x10)
	TYPE_BYTESLICE = Byte(0x11)

	TYPE_TIME = Byte(0x20)
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
	case Int:
		panic("Int not supported")
	case UInt:
		panic("UInt not supported")

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

func ReadBinary(r io.Reader) Binary {
	type_ := ReadByte(r)
	switch type_ {
	case TYPE_NIL:
		return nil
	case TYPE_BYTE:
		return ReadByte(r)
	case TYPE_INT8:
		return ReadInt8(r)
	case TYPE_UINT8:
		return ReadUInt8(r)
	case TYPE_INT16:
		return ReadInt16(r)
	case TYPE_UINT16:
		return ReadUInt16(r)
	case TYPE_INT32:
		return ReadInt32(r)
	case TYPE_UINT32:
		return ReadUInt32(r)
	case TYPE_INT64:
		return ReadInt64(r)
	case TYPE_UINT64:
		return ReadUInt64(r)

	case TYPE_STRING:
		return ReadString(r)
	case TYPE_BYTESLICE:
		return ReadByteSlice(r)

	case TYPE_TIME:
		return ReadTime(r)

	default:
		panic("Unsupported type")
	}
}
