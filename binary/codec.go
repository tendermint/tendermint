package binary

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"time"
)

type Encoder func(o interface{}, w io.Writer, n *int64, err *error)
type Decoder func(r io.Reader, n *int64, err *error) interface{}
type Comparator func(o1 interface{}, o2 interface{}) int

type Codec struct {
	Encode  Encoder
	Decode  Decoder
	Compare Comparator
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

func BasicCodecEncoder(o interface{}, w io.Writer, n *int64, err *error) {
	switch o.(type) {
	case nil:
		panic("nil type unsupported")
	case byte:
		WriteByte(typeByte, w, n, err)
		WriteByte(o.(byte), w, n, err)
	case int8:
		WriteByte(typeInt8, w, n, err)
		WriteInt8(o.(int8), w, n, err)
	//case uint8:
	//	WriteByte( typeUInt8, w, n, err)
	//	WriteUInt8( o.(uint8), w, n, err)
	case int16:
		WriteByte(typeInt16, w, n, err)
		WriteInt16(o.(int16), w, n, err)
	case uint16:
		WriteByte(typeUInt16, w, n, err)
		WriteUInt16(o.(uint16), w, n, err)
	case int32:
		WriteByte(typeInt32, w, n, err)
		WriteInt32(o.(int32), w, n, err)
	case uint32:
		WriteByte(typeUInt32, w, n, err)
		WriteUInt32(o.(uint32), w, n, err)
	case int64:
		WriteByte(typeInt64, w, n, err)
		WriteInt64(o.(int64), w, n, err)
	case uint64:
		WriteByte(typeUInt64, w, n, err)
		WriteUInt64(o.(uint64), w, n, err)
	case int:
		WriteByte(typeVarInt, w, n, err)
		WriteVarInt(o.(int), w, n, err)
	case uint:
		WriteByte(typeUVarInt, w, n, err)
		WriteUVarInt(o.(uint), w, n, err)
	case string:
		WriteByte(typeString, w, n, err)
		WriteString(o.(string), w, n, err)
	case []byte:
		WriteByte(typeByteSlice, w, n, err)
		WriteByteSlice(o.([]byte), w, n, err)
	case time.Time:
		WriteByte(typeTime, w, n, err)
		WriteTime(o.(time.Time), w, n, err)
	default:
		panic(fmt.Sprintf("Unsupported type: %v", reflect.TypeOf(o)))
	}
}

func BasicCodecDecoder(r io.Reader, n *int64, err *error) (o interface{}) {
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
		if *err != nil {
			panic(err)
		} else {
			panic(fmt.Sprintf("Unsupported type byte: %X", type_))
		}
	}
	return o
}

func BasicCodecComparator(o1 interface{}, o2 interface{}) int {
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
		panic(fmt.Sprintf("Unsupported type: %v", reflect.TypeOf(o1)))
	}
}

var BasicCodec = Codec{
	Encode:  BasicCodecEncoder,
	Decode:  BasicCodecDecoder,
	Compare: BasicCodecComparator,
}
