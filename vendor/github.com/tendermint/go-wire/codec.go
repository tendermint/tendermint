package wire

import (
	"bytes"
	"errors"
	"fmt"
	. "github.com/tendermint/go-common"
	"io"
	"reflect"
	"time"
)

type Encoder func(o interface{}, w io.Writer, n *int, err *error)
type Decoder func(r io.Reader, n *int, err *error) interface{}
type Comparator func(o1 interface{}, o2 interface{}) int

type Codec struct {
	Encode  Encoder
	Decode  Decoder
	Compare Comparator
}

const (
	typeByte = byte(0x01)
	typeInt8 = byte(0x02)
	// typeUint8     = byte(0x03)
	typeInt16     = byte(0x04)
	typeUint16    = byte(0x05)
	typeInt32     = byte(0x06)
	typeUint32    = byte(0x07)
	typeInt64     = byte(0x08)
	typeUint64    = byte(0x09)
	typeVarint    = byte(0x0A)
	typeUvarint   = byte(0x0B)
	typeString    = byte(0x10)
	typeByteSlice = byte(0x11)
	typeTime      = byte(0x20)
)

func BasicCodecEncoder(o interface{}, w io.Writer, n *int, err *error) {
	switch o := o.(type) {
	case nil:
		PanicSanity("nil type unsupported")
	case byte:
		WriteByte(typeByte, w, n, err)
		WriteByte(o, w, n, err)
	case int8:
		WriteByte(typeInt8, w, n, err)
		WriteInt8(o, w, n, err)
	//case uint8:
	//	WriteByte( typeUint8, w, n, err)
	//	WriteUint8( o, w, n, err)
	case int16:
		WriteByte(typeInt16, w, n, err)
		WriteInt16(o, w, n, err)
	case uint16:
		WriteByte(typeUint16, w, n, err)
		WriteUint16(o, w, n, err)
	case int32:
		WriteByte(typeInt32, w, n, err)
		WriteInt32(o, w, n, err)
	case uint32:
		WriteByte(typeUint32, w, n, err)
		WriteUint32(o, w, n, err)
	case int64:
		WriteByte(typeInt64, w, n, err)
		WriteInt64(o, w, n, err)
	case uint64:
		WriteByte(typeUint64, w, n, err)
		WriteUint64(o, w, n, err)
	case int:
		WriteByte(typeVarint, w, n, err)
		WriteVarint(o, w, n, err)
	case uint:
		WriteByte(typeUvarint, w, n, err)
		WriteUvarint(o, w, n, err)
	case string:
		WriteByte(typeString, w, n, err)
		WriteString(o, w, n, err)
	case []byte:
		WriteByte(typeByteSlice, w, n, err)
		WriteByteSlice(o, w, n, err)
	case time.Time:
		WriteByte(typeTime, w, n, err)
		WriteTime(o, w, n, err)
	default:
		PanicSanity(fmt.Sprintf("Unsupported type: %v", reflect.TypeOf(o)))
	}
}

func BasicCodecDecoder(r io.Reader, n *int, err *error) (o interface{}) {
	type_ := ReadByte(r, n, err)
	if *err != nil {
		return
	}
	switch type_ {
	case typeByte:
		o = ReadByte(r, n, err)
	case typeInt8:
		o = ReadInt8(r, n, err)
	//case typeUint8:
	//	o = ReadUint8(r, n, err)
	case typeInt16:
		o = ReadInt16(r, n, err)
	case typeUint16:
		o = ReadUint16(r, n, err)
	case typeInt32:
		o = ReadInt32(r, n, err)
	case typeUint32:
		o = ReadUint32(r, n, err)
	case typeInt64:
		o = ReadInt64(r, n, err)
	case typeUint64:
		o = ReadUint64(r, n, err)
	case typeVarint:
		o = ReadVarint(r, n, err)
	case typeUvarint:
		o = ReadUvarint(r, n, err)
	case typeString:
		o = ReadString(r, 0, n, err)
	case typeByteSlice:
		o = ReadByteSlice(r, 0, n, err)
	case typeTime:
		o = ReadTime(r, n, err)
	default:
		*err = errors.New(Fmt("Unsupported type byte: %X", type_))
	}
	return
}

// Contract: Caller must ensure that types match.
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
		PanicSanity(Fmt("Unsupported type: %v", reflect.TypeOf(o1)))
	}
	return 0
}

var BasicCodec = Codec{
	Encode:  BasicCodecEncoder,
	Decode:  BasicCodecDecoder,
	Compare: BasicCodecComparator,
}

//----------------------------------------

func BytesCodecEncoder(o interface{}, w io.Writer, n *int, err *error) {
	WriteByteSlice(o.([]byte), w, n, err)
}

func BytesCodecDecoder(r io.Reader, n *int, err *error) (o interface{}) {
	o = ReadByteSlice(r, 0, n, err)
	return
}

func BytesCodecComparator(o1 interface{}, o2 interface{}) int {
	return bytes.Compare(o1.([]byte), o2.([]byte))
}

var BytesCodec = Codec{
	Encode:  BytesCodecEncoder,
	Decode:  BytesCodecDecoder,
	Compare: BytesCodecComparator,
}
