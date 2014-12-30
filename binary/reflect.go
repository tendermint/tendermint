package binary

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"

	. "github.com/tendermint/tendermint/common"
)

type TypeInfo struct {
	Type    reflect.Type // The type
	Encoder Encoder      // Optional custom encoder function
	Decoder Decoder      // Optional custom decoder function

	HasTypeByte bool
	TypeByte    byte
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

// NOTE: do not access typeInfos directly, but call GetTypeInfo()
var typeInfosMtx sync.Mutex
var typeInfos = map[reflect.Type]*TypeInfo{}

func GetTypeInfo(rt reflect.Type) *TypeInfo {
	typeInfosMtx.Lock()
	defer typeInfosMtx.Unlock()
	info := typeInfos[rt]
	if info == nil {
		info = &TypeInfo{Type: rt}
		RegisterType(info)
	}
	return info
}

// Registers and possibly modifies the TypeInfo.
// NOTE: not goroutine safe, so only call upon program init.
func RegisterType(info *TypeInfo) *TypeInfo {

	// Also register the underlying struct's info, if info.Type is a pointer.
	// Or, if info.Type is not a pointer, register the pointer.
	var rt, ptrRt reflect.Type
	if info.Type.Kind() == reflect.Ptr {
		rt, ptrRt = info.Type.Elem(), info.Type
	} else {
		rt, ptrRt = info.Type, reflect.PtrTo(info.Type)
	}

	// Register the type info
	typeInfos[rt] = info
	typeInfos[ptrRt] = info

	// See if the type implements HasTypeByte
	if rt.Implements(reflect.TypeOf((*HasTypeByte)(nil)).Elem()) {
		zero := reflect.Zero(rt)
		typeByte := zero.Interface().(HasTypeByte).TypeByte()
		if info.HasTypeByte && info.TypeByte != typeByte {
			panic(fmt.Sprintf("Type %v expected TypeByte of %X", rt, typeByte))
		} else {
			info.HasTypeByte = true
			info.TypeByte = typeByte
		}
	} else if ptrRt.Implements(reflect.TypeOf((*HasTypeByte)(nil)).Elem()) {
		zero := reflect.Zero(ptrRt)
		typeByte := zero.Interface().(HasTypeByte).TypeByte()
		if info.HasTypeByte && info.TypeByte != typeByte {
			panic(fmt.Sprintf("Type %v expected TypeByte of %X", ptrRt, typeByte))
		} else {
			info.HasTypeByte = true
			info.TypeByte = typeByte
		}
	}

	return info
}

func readReflect(rv reflect.Value, rt reflect.Type, r Unreader, n *int64, err *error) {

	log.Debug("Read reflect", "type", rt)

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

	// Get typeInfo
	typeInfo := GetTypeInfo(rt)

	// Custom decoder
	if typeInfo.Decoder != nil {
		decoded := typeInfo.Decoder(r, n, err)
		decodedRv := reflect.Indirect(reflect.ValueOf(decoded))
		rv.Set(decodedRv)
		return
	}

	// Read TypeByte prefix
	if typeInfo.HasTypeByte {
		typeByte := ReadByte(r, n, err)
		log.Debug("Read typebyte", "typeByte", typeByte)
		if typeByte != typeInfo.TypeByte {
			*err = errors.New(fmt.Sprintf("Expected TypeByte of %X but got %X", typeInfo.TypeByte, typeByte))
			return
		}
	}

	switch rt.Kind() {
	case reflect.Slice:
		elemRt := rt.Elem()
		if elemRt.Kind() == reflect.Uint8 {
			// Special case: Byteslices
			byteslice := ReadByteSlice(r, n, err)
			log.Debug("Read byteslice", "bytes", byteslice)
			rv.Set(reflect.ValueOf(byteslice))
		} else {
			// Read length
			length := int(ReadUvarint(r, n, err))
			log.Debug(Fmt("Read length: %v", length))
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
			if field.PkgPath != "" {
				continue
			}
			fieldRv := rv.Field(i)
			readReflect(fieldRv, field.Type, r, n, err)
		}

	case reflect.String:
		str := ReadString(r, n, err)
		log.Debug(Fmt("Read string: %v", str))
		rv.SetString(str)

	case reflect.Int64:
		num := ReadUint64(r, n, err)
		log.Debug(Fmt("Read num: %v", num))
		rv.SetInt(int64(num))

	case reflect.Int32:
		num := ReadUint32(r, n, err)
		log.Debug(Fmt("Read num: %v", num))
		rv.SetInt(int64(num))

	case reflect.Int16:
		num := ReadUint16(r, n, err)
		log.Debug(Fmt("Read num: %v", num))
		rv.SetInt(int64(num))

	case reflect.Int8:
		num := ReadUint8(r, n, err)
		log.Debug(Fmt("Read num: %v", num))
		rv.SetInt(int64(num))

	case reflect.Int:
		num := ReadUvarint(r, n, err)
		log.Debug(Fmt("Read num: %v", num))
		rv.SetInt(int64(num))

	case reflect.Uint64:
		num := ReadUint64(r, n, err)
		log.Debug(Fmt("Read num: %v", num))
		rv.SetUint(uint64(num))

	case reflect.Uint32:
		num := ReadUint32(r, n, err)
		log.Debug(Fmt("Read num: %v", num))
		rv.SetUint(uint64(num))

	case reflect.Uint16:
		num := ReadUint16(r, n, err)
		log.Debug(Fmt("Read num: %v", num))
		rv.SetUint(uint64(num))

	case reflect.Uint8:
		num := ReadUint8(r, n, err)
		log.Debug(Fmt("Read num: %v", num))
		rv.SetUint(uint64(num))

	case reflect.Uint:
		num := ReadUvarint(r, n, err)
		log.Debug(Fmt("Read num: %v", num))
		rv.SetUint(uint64(num))

	default:
		panic(fmt.Sprintf("Unknown field type %v", rt.Kind()))
	}
}

func writeReflect(rv reflect.Value, rt reflect.Type, w io.Writer, n *int64, err *error) {

	// Get typeInfo
	typeInfo := GetTypeInfo(rt)

	// Custom encoder, say for an interface type rt.
	if typeInfo.Encoder != nil {
		typeInfo.Encoder(rv.Interface(), w, n, err)
		return
	}

	// Dereference interface
	if rt.Kind() == reflect.Interface {
		rv = rv.Elem()
		rt = rv.Type()
		// If interface type, get typeInfo of underlying type.
		typeInfo = GetTypeInfo(rt)
	}

	// Dereference pointer
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
		rv = rv.Elem()
		// RegisterType registers the ptr type,
		// so typeInfo is already for the ptr.
	}

	// Write TypeByte prefix
	if typeInfo.HasTypeByte {
		WriteByte(typeInfo.TypeByte, w, n, err)
	}

	switch rt.Kind() {
	case reflect.Slice:
		elemRt := rt.Elem()
		if elemRt.Kind() == reflect.Uint8 {
			// Special case: Byteslices
			byteslice := rv.Bytes()
			WriteByteSlice(byteslice, w, n, err)
		} else {
			// Write length
			length := rv.Len()
			WriteUvarint(uint(length), w, n, err)
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
			if field.PkgPath != "" {
				continue
			}
			fieldRv := rv.Field(i)
			writeReflect(fieldRv, field.Type, w, n, err)
		}

	case reflect.String:
		WriteString(rv.String(), w, n, err)

	case reflect.Int64:
		WriteInt64(rv.Int(), w, n, err)

	case reflect.Int32:
		WriteInt32(int32(rv.Int()), w, n, err)

	case reflect.Int16:
		WriteInt16(int16(rv.Int()), w, n, err)

	case reflect.Int8:
		WriteInt8(int8(rv.Int()), w, n, err)

	case reflect.Int:
		WriteVarint(int(rv.Int()), w, n, err)

	case reflect.Uint64:
		WriteUint64(rv.Uint(), w, n, err)

	case reflect.Uint32:
		WriteUint32(uint32(rv.Uint()), w, n, err)

	case reflect.Uint16:
		WriteUint16(uint16(rv.Uint()), w, n, err)

	case reflect.Uint8:
		WriteUint8(uint8(rv.Uint()), w, n, err)

	case reflect.Uint:
		WriteUvarint(uint(rv.Uint()), w, n, err)

	default:
		panic(fmt.Sprintf("Unknown field type %v", rt.Kind()))
	}
}
