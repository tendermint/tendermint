package binary

import (
	"bytes"
	"io"
	"time"
)

type Codec interface {
	Encode(o interface{}, w io.Writer, n *int64, err *error)
	Decode(r io.Reader, n *int64, err *error) interface{}
	Compare(o1 interface{}, o2 interface{}) int
}

const (
	typeByte = byte(0x01)
	typeInt8 = byte(0x02)
	// typeUInt8     = byte(0x03)
	typeInt16     = byte(0x04)
	typeUInt16    = byte(0x05)
	typeInt32     = byte(0x06)
	typeUInt32    = byte(0x07)
	typeInt64     = byte(0x08)
	typeUInt64    = byte(0x09)
	typeVarInt    = byte(0x0A)
	typeUVarInt   = byte(0x0B)
	typeString    = byte(0x10)
	typeByteSlice = byte(0x11)
	typeTime      = byte(0x20)
)

var BasicCodec = basicCodec{}

type basicCodec struct{}

func (bc basicCodec) Encode(o interface{}, w io.Writer, n *int64, err *error) {
	switch o.(type) {
	case nil:
		panic("nil type unsupported")
	case byte:
		WriteByte(w, typeByte, n, err)
		WriteByte(w, o.(byte), n, err)
	case int8:
		WriteByte(w, typeInt8, n, err)
		WriteInt8(w, o.(int8), n, err)
	//case uint8:
	//	WriteByte(w, typeUInt8, n, err)
	//	WriteUInt8(w, o.(uint8), n, err)
	case int16:
		WriteByte(w, typeInt16, n, err)
		WriteInt16(w, o.(int16), n, err)
	case uint16:
		WriteByte(w, typeUInt16, n, err)
		WriteUInt16(w, o.(uint16), n, err)
	case int32:
		WriteByte(w, typeInt32, n, err)
		WriteInt32(w, o.(int32), n, err)
	case uint32:
		WriteByte(w, typeUInt32, n, err)
		WriteUInt32(w, o.(uint32), n, err)
	case int64:
		WriteByte(w, typeInt64, n, err)
		WriteInt64(w, o.(int64), n, err)
	case uint64:
		WriteByte(w, typeUInt64, n, err)
		WriteUInt64(w, o.(uint64), n, err)
	case int:
		WriteByte(w, typeVarInt, n, err)
		WriteVarInt(w, o.(int), n, err)
	case uint:
		WriteByte(w, typeUVarInt, n, err)
		WriteUVarInt(w, o.(uint), n, err)
	case string:
		WriteByte(w, typeString, n, err)
		WriteString(w, o.(string), n, err)
	case []byte:
		WriteByte(w, typeByteSlice, n, err)
		WriteByteSlice(w, o.([]byte), n, err)
	case time.Time:
		WriteByte(w, typeTime, n, err)
		WriteTime(w, o.(time.Time), n, err)
	default:
		panic("Unsupported type")
	}
}

func (bc basicCodec) Decode(r io.Reader, n *int64, err *error) (o interface{}) {
	type_ := ReadByte(r, n, err)
	switch type_ {
	case typeByte:
		o = ReadByte(r, n, err)
	case typeInt8:
		o = ReadInt8(r, n, err)
	//case typeUInt8:
	//	o = ReadUInt8(r, n, err)
	case typeInt16:
		o = ReadInt16(r, n, err)
	case typeUInt16:
		o = ReadUInt16(r, n, err)
	case typeInt32:
		o = ReadInt32(r, n, err)
	case typeUInt32:
		o = ReadUInt32(r, n, err)
	case typeInt64:
		o = ReadInt64(r, n, err)
	case typeUInt64:
		o = ReadUInt64(r, n, err)
	case typeVarInt:
		o = ReadVarInt(r, n, err)
	case typeUVarInt:
		o = ReadUVarInt(r, n, err)
	case typeString:
		o = ReadString(r, n, err)
	case typeByteSlice:
		o = ReadByteSlice(r, n, err)
	case typeTime:
		o = ReadTime(r, n, err)
	default:
		panic("Unsupported type")
	}
	return o
}

func (bc basicCodec) Compare(o1 interface{}, o2 interface{}) int {
	switch o1.(type) {
	case byte:
		return int(o1.(byte) - o2.(byte))
	case int8:
		return int(o1.(int8) - o2.(int8))
	//case uint8:
	case int16:
		return int(o1.(int16) - o2.(int16))
	case uint16:
		return int(o1.(uint16) - o2.(uint16))
	case int32:
		return int(o1.(int32) - o2.(int32))
	case uint32:
		return int(o1.(uint32) - o2.(uint32))
	case int64:
		return int(o1.(int64) - o2.(int64))
	case uint64:
		return int(o1.(uint64) - o2.(uint64))
	case int:
		return o1.(int) - o2.(int)
	case uint:
		return int(o1.(uint)) - int(o2.(uint))
	case string:
		return bytes.Compare([]byte(o1.(string)), []byte(o2.(string)))
	case []byte:
		return bytes.Compare(o1.([]byte), o2.([]byte))
	case time.Time:
		return int(o1.(time.Time).UnixNano() - o2.(time.Time).UnixNano())
	default:
		panic("Unsupported type")
	}
}
