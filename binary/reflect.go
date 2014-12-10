package binary

import (
	"fmt"
	"io"
	"reflect"
)

type TypeInfo struct {
	Type    reflect.Type // The type
	Encoder Encoder      // Optional custom encoder function
	Decoder Decoder      // Optional custom decoder function
}

// If a type implements TypeByte, the byte is included
// as the first byte for encoding.  This is used to encode
// interfaces/union types.  In this case the decoding should
// be done manually with a switch statement, and so the
// reflection-based decoder provided here does not expect this
// prefix byte.
// See the reactor implementations for use-cases.
type HasTypeByte interface {
	TypeByte() byte
}

var typeInfos = map[reflect.Type]*TypeInfo{}

func RegisterType(info *TypeInfo) bool {

	// Register the type info
	typeInfos[info.Type] = info

	// Also register the underlying struct's info, if info.Type is a pointer.
	if info.Type.Kind() == reflect.Ptr {
		rt := info.Type.Elem()
		typeInfos[rt] = info
	}

	return true
}

func readReflect(rv reflect.Value, rt reflect.Type, r io.Reader, n *int64, err *error) {

	// First, create a new struct if rv is nil pointer.
	if rt.Kind() == reflect.Ptr && rv.IsNil() {
		newRv := reflect.New(rt.Elem())
		rv.Set(newRv)
		rv = newRv
	}

	// Dereference pointer
	// Still addressable, thus settable!
	if rv.Kind() == reflect.Ptr {
		rv, rt = rv.Elem(), rt.Elem()
	}

	// Custom decoder
	typeInfo := typeInfos[rt]
	if typeInfo != nil && typeInfo.Decoder != nil {
		decoded := typeInfo.Decoder(r, n, err)
		decodedRv := reflect.Indirect(reflect.ValueOf(decoded))
		rv.Set(decodedRv)
		return
	}

	switch rt.Kind() {
	case reflect.Slice:
		elemRt := rt.Elem()
		if elemRt.Kind() == reflect.Uint8 {
			// Special case: Byteslices
			byteslice := ReadByteSlice(r, n, err)
			rv.Set(reflect.ValueOf(byteslice))
		} else {
			// Read length
			length := int(ReadUVarInt(r, n, err))
			sliceRv := reflect.MakeSlice(rt, length, length)
			// Read elems
			for i := 0; i < length; i++ {
				elemRv := sliceRv.Index(i)
				readReflect(elemRv, elemRt, r, n, err)
			}
			rv.Set(sliceRv)
		}

	case reflect.Struct:
		numFields := rt.NumField()
		for i := 0; i < numFields; i++ {
			field := rt.Field(i)
			if field.Anonymous {
				continue
			}
			fieldRv := rv.Field(i)
			readReflect(fieldRv, field.Type, r, n, err)
		}

	case reflect.String:
		str := ReadString(r, n, err)
		rv.SetString(str)

	case reflect.Uint64:
		num := ReadUInt64(r, n, err)
		rv.SetUint(uint64(num))

	case reflect.Uint32:
		num := ReadUInt32(r, n, err)
		rv.SetUint(uint64(num))

	case reflect.Uint16:
		num := ReadUInt16(r, n, err)
		rv.SetUint(uint64(num))

	case reflect.Uint8:
		num := ReadUInt8(r, n, err)
		rv.SetUint(uint64(num))

	case reflect.Uint:
		num := ReadUVarInt(r, n, err)
		rv.SetUint(uint64(num))

	default:
		panic(fmt.Sprintf("Unknown field type %v", rt.Kind()))
	}
}

func writeReflect(rv reflect.Value, rt reflect.Type, w io.Writer, n *int64, err *error) {

	// Custom encoder
	typeInfo := typeInfos[rt]
	if typeInfo != nil && typeInfo.Encoder != nil {
		typeInfo.Encoder(rv.Interface(), w, n, err)
		return
	}

	// Dereference pointer or interface
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
		rv = rv.Elem()
	} else if rt.Kind() == reflect.Interface {
		rv = rv.Elem()
		rt = rv.Type()
	}

	// Write TypeByte prefix
	if rt.Implements(reflect.TypeOf((*HasTypeByte)(nil)).Elem()) {
		WriteByte(rv.Interface().(HasTypeByte).TypeByte(), w, n, err)
	}

	switch rt.Kind() {
	case reflect.Slice:
		elemRt := rt.Elem()
		if elemRt.Kind() == reflect.Uint8 {
			// Special case: Byteslices
			byteslice := rv.Interface().([]byte)
			WriteByteSlice(byteslice, w, n, err)
		} else {
			// Write length
			length := rv.Len()
			WriteUVarInt(uint(length), w, n, err)
			// Write elems
			for i := 0; i < length; i++ {
				elemRv := rv.Index(i)
				writeReflect(elemRv, elemRt, w, n, err)
			}
		}

	case reflect.Struct:
		numFields := rt.NumField()
		for i := 0; i < numFields; i++ {
			field := rt.Field(i)
			if field.Anonymous {
				continue
			}
			fieldRv := rv.Field(i)
			writeReflect(fieldRv, field.Type, w, n, err)
		}

	case reflect.String:
		WriteString(rv.String(), w, n, err)

	case reflect.Uint64:
		WriteUInt64(rv.Uint(), w, n, err)

	case reflect.Uint32:
		WriteUInt32(uint32(rv.Uint()), w, n, err)

	case reflect.Uint16:
		WriteUInt16(uint16(rv.Uint()), w, n, err)

	case reflect.Uint8:
		WriteUInt8(uint8(rv.Uint()), w, n, err)

	case reflect.Uint:
		WriteUVarInt(uint(rv.Uint()), w, n, err)

	default:
		panic(fmt.Sprintf("Unknown field type %v", rt.Kind()))
	}
}
