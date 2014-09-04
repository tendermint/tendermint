package binary

import (
	"bytes"
	"time"
)

type Codec interface {
	Write(interface{}) ([]byte, error)
	Read([]byte) (interface{}, error)
}

const (
	typeNil  = byte(0x00)
	typeByte = byte(0x01)
	typeInt8 = byte(0x02)
	// typeUInt8     = byte(0x03)
	typeInt16     = byte(0x04)
	typeUInt16    = byte(0x05)
	typeInt32     = byte(0x06)
	typeUInt32    = byte(0x07)
	typeInt64     = byte(0x08)
	typeUInt64    = byte(0x09)
	typeString    = byte(0x10)
	typeByteSlice = byte(0x11)
	typeTime      = byte(0x20)
)

var BasicCodec = basicCodec{}

type basicCodec struct{}

func (bc basicCodec) Write(o interface{}) ([]byte, error) {
	n, err, w := new(int64), new(error), new(bytes.Buffer)
	switch o.(type) {
	case nil:
		WriteByte(w, typeNil, n, err)
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
	return w.Bytes(), *err
}

func (bc basicCodec) Read(bz []byte) (interface{}, error) {
	n, err, r, o := new(int64), new(error), bytes.NewBuffer(bz), interface{}(nil)
	type_ := ReadByte(r, n, err)
	switch type_ {
	case typeNil:
		o = nil
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
	case typeString:
		o = ReadString(r, n, err)
	case typeByteSlice:
		o = ReadByteSlice(r, n, err)
	case typeTime:
		o = ReadTime(r, n, err)
	default:
		panic("Unsupported type")
	}
	return o, *err
}
