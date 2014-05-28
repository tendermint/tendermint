package merkle

const (
    TYPE_NIL        = byte(0x00)
    TYPE_BYTE       = byte(0x01)
    TYPE_INT8       = byte(0x02)
    TYPE_UINT8      = byte(0x03)
    TYPE_INT16      = byte(0x04)
    TYPE_UINT16     = byte(0x05)
    TYPE_INT32      = byte(0x06)
    TYPE_UINT32     = byte(0x07)
    TYPE_INT64      = byte(0x08)
    TYPE_UINT64     = byte(0x09)
    TYPE_STRING     = byte(0x10)
    TYPE_BYTESLICE  = byte(0x11)
)

func GetBinaryType(o Binary) byte {
    switch o.(type) {
    case nil:       return TYPE_NIL
    case Byte:      return TYPE_BYTE
    case Int8:      return TYPE_INT8
    case UInt8:     return TYPE_UINT8
    case Int16:     return TYPE_INT16
    case UInt16:    return TYPE_UINT16
    case Int32:     return TYPE_INT32
    case UInt32:    return TYPE_UINT32
    case Int64:     return TYPE_INT64
    case UInt64:    return TYPE_UINT64
    case Int:       panic("Int not supported")
    case UInt:      panic("UInt not supported")
    case String:    return TYPE_STRING
    case ByteSlice: return TYPE_BYTESLICE
    default:        panic("Unsupported type")
    }
}

func LoadBinary(buf []byte, start int) (Binary, int) {
    typeByte := buf[start]
    switch typeByte {
    case TYPE_NIL:      return nil,                       start+1
    case TYPE_BYTE:     return LoadByte(buf[start+1:]),   start+2
    case TYPE_INT8:     return LoadInt8(buf[start+1:]),   start+2
    case TYPE_UINT8:    return LoadUInt8(buf[start+1:]),  start+2
    case TYPE_INT16:    return LoadInt16(buf[start+1:]),  start+3
    case TYPE_UINT16:   return LoadUInt16(buf[start+1:]), start+3
    case TYPE_INT32:    return LoadInt32(buf[start+1:]),  start+5
    case TYPE_UINT32:   return LoadUInt32(buf[start+1:]), start+5
    case TYPE_INT64:    return LoadInt64(buf[start+1:]),  start+9
    case TYPE_UINT64:   return LoadUInt64(buf[start+1:]), start+9
    case TYPE_STRING:   return LoadString(buf, start+1)
    case TYPE_BYTESLICE:return LoadByteSlice(buf, start+1)
    default:            panic("Unsupported type")
    }
}
