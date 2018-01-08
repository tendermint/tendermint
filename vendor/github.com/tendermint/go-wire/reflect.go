package wire

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"time"

	cmn "github.com/tendermint/tmlibs/common"
)

type TypeInfo struct {
	Type reflect.Type // The type

	// If Type is kind reflect.Interface, is registered
	IsRegisteredInterface bool
	ByteToType            map[byte]reflect.Type
	TypeToByte            map[reflect.Type]byte

	// If Type is kind reflect.Struct
	Fields []StructFieldInfo
	Unwrap bool // if struct has only one field and its an anonymous interface
}

type Options struct {
	JSONName      string      // (JSON) Corresponding JSON field name. (override with `json=""`)
	JSONOmitEmpty bool        // (JSON) Omit field if value is empty
	Varint        bool        // (Binary) Use length-prefixed encoding for (u)int64
	Unsafe        bool        // (JSON/Binary) Explicitly enable support for floats or maps
	ZeroValue     interface{} // Prototype zero object
}

func getOptionsFromField(field reflect.StructField) (skip bool, opts Options) {
	jsonTag := field.Tag.Get("json")
	binTag := field.Tag.Get("binary")
	wireTag := field.Tag.Get("wire")
	if jsonTag == "-" {
		skip = true
		return
	}
	jsonTagParts := strings.Split(jsonTag, ",")
	if jsonTagParts[0] == "" {
		opts.JSONName = field.Name
	} else {
		opts.JSONName = jsonTagParts[0]
	}
	if len(jsonTagParts) > 1 {
		if jsonTagParts[1] == "omitempty" {
			opts.JSONOmitEmpty = true
		}
	}
	if binTag == "varint" { // TODO: extend
		opts.Varint = true
	}
	if wireTag == "unsafe" {
		opts.Unsafe = true
	}
	opts.ZeroValue = reflect.Zero(field.Type).Interface()
	return
}

type StructFieldInfo struct {
	Index   int          // Struct field index
	Type    reflect.Type // Struct field type
	Options              // Encoding options
}

func (info StructFieldInfo) unpack() (int, reflect.Type, Options) {
	return info.Index, info.Type, info.Options
}

// e.g. If o is struct{Foo}{}, return is the Foo reflection type.
func GetTypeFromStructDeclaration(o interface{}) reflect.Type {
	rt := reflect.TypeOf(o)
	if rt.NumField() != 1 {
		cmn.PanicSanity("Unexpected number of fields in struct-wrapped declaration of type")
	}
	return rt.Field(0).Type
}

// Predeclaration of common types
var (
	timeType = GetTypeFromStructDeclaration(struct{ time.Time }{})
)

const (
	RFC3339Millis = "2006-01-02T15:04:05.000Z" // forced microseconds
)

// NOTE: do not access typeInfos directly, but call GetTypeInfo()
var typeInfosMtx sync.RWMutex
var typeInfos = map[reflect.Type]*TypeInfo{}

func GetTypeInfo(rt reflect.Type) *TypeInfo {
	typeInfosMtx.RLock()
	info := typeInfos[rt]
	typeInfosMtx.RUnlock()
	if info == nil {
		info = MakeTypeInfo(rt)
		typeInfosMtx.Lock()
		typeInfos[rt] = info
		typeInfosMtx.Unlock()
	}
	return info
}

// For use with the RegisterInterface declaration
type ConcreteType struct {
	O    interface{}
	Byte byte
}

// This function should be used to register the receiving interface that will
// be used to decode an underlying concrete type. The interface MUST be embedded
// in a struct, and the interface MUST be the only field and it MUST be exported.
// For example:
//      RegisterInterface(struct{ Animal }{}, ConcreteType{&foo, 0x01})
func RegisterInterface(o interface{}, ctypes ...ConcreteType) *TypeInfo {
	it := GetTypeFromStructDeclaration(o)
	if it.Kind() != reflect.Interface {
		cmn.PanicSanity("RegisterInterface expects an interface")
	}
	toType := make(map[byte]reflect.Type, 0)
	toByte := make(map[reflect.Type]byte, 0)
	for _, ctype := range ctypes {
		crt := reflect.TypeOf(ctype.O)
		typeByte := ctype.Byte
		if typeByte == 0x00 {
			cmn.PanicSanity(cmn.Fmt("Byte of 0x00 is reserved for nil (%v)", ctype))
		}
		if toType[typeByte] != nil {
			cmn.PanicSanity(cmn.Fmt("Duplicate Byte for type %v and %v", ctype, toType[typeByte]))
		}
		toType[typeByte] = crt
		toByte[crt] = typeByte
	}
	typeInfo := &TypeInfo{
		Type: it,
		IsRegisteredInterface: true,
		ByteToType:            toType,
		TypeToByte:            toByte,
	}
	typeInfos[it] = typeInfo
	return typeInfo
}

func MakeTypeInfo(rt reflect.Type) *TypeInfo {
	info := &TypeInfo{Type: rt}

	// If struct, register field name options
	if rt.Kind() == reflect.Struct {
		numFields := rt.NumField()
		structFields := []StructFieldInfo{}
		for i := 0; i < numFields; i++ {
			field := rt.Field(i)
			if field.PkgPath != "" {
				continue
			}
			skip, opts := getOptionsFromField(field)
			if skip {
				continue
			}
			structFields = append(structFields, StructFieldInfo{
				Index:   i,
				Type:    field.Type,
				Options: opts,
			})
		}
		info.Fields = structFields

		// Maybe type is a wrapper.
		if len(structFields) == 1 {
			jsonName := rt.Field(structFields[0].Index).Tag.Get("json")
			if jsonName == "unwrap" {
				info.Unwrap = true
			}
		}
	}

	return info
}

// Contract: Caller must ensure that rt is supported
// (e.g. is recursively composed of supported native types, and structs and slices.)
func readReflectBinary(rv reflect.Value, rt reflect.Type, opts Options, r io.Reader, lmt int, n *int, err *error) {

	// Get typeInfo
	typeInfo := GetTypeInfo(rt)

	if rt.Kind() == reflect.Interface {
		if !typeInfo.IsRegisteredInterface {
			// There's no way we can read such a thing.
			*err = errors.New(cmn.Fmt("Cannot read unregistered interface type %v", rt))
			return
		}
		typeByte := ReadByte(r, n, err)
		if *err != nil {
			return
		}
		if typeByte == 0x00 {
			return // nil
		}
		crt, ok := typeInfo.ByteToType[typeByte]
		if !ok {
			*err = errors.New(cmn.Fmt("Unexpected type byte %X for type %v", typeByte, rt))
			return
		}
		if crt.Kind() == reflect.Ptr {
			crt = crt.Elem()
			crv := reflect.New(crt)
			readReflectBinary(crv.Elem(), crt, opts, r, lmt, n, err)
			rv.Set(crv) // NOTE: orig rv is ignored.
		} else {
			crv := reflect.New(crt).Elem()
			readReflectBinary(crv, crt, opts, r, lmt, n, err)
			rv.Set(crv) // NOTE: orig rv is ignored.
		}
		return
	}

	if rt.Kind() == reflect.Ptr {
		typeByte := ReadByte(r, n, err)
		if *err != nil {
			return
		}
		if typeByte == 0x00 {
			return // nil
		} else if typeByte != 0x01 {
			*err = errors.New(cmn.Fmt("Unexpected type byte %X for ptr of untyped thing", typeByte))
			return
		}
		// Create new if rv is nil.
		if rv.IsNil() {
			newRv := reflect.New(rt.Elem())
			rv.Set(newRv)
			rv = newRv
		}
		// Dereference pointer
		rv, rt = rv.Elem(), rt.Elem()
		typeInfo = GetTypeInfo(rt)
		// continue...
	}

	switch rt.Kind() {
	case reflect.Array:
		elemRt := rt.Elem()
		length := rt.Len()
		if elemRt.Kind() == reflect.Uint8 {
			// Special case: Bytearrays
			buf := make([]byte, length)
			ReadFull(buf, r, n, err)
			if *err != nil {
				return
			}
			//log.Info("Read bytearray", "bytes", buf, "n", *n)
			reflect.Copy(rv, reflect.ValueOf(buf))
		} else {
			for i := 0; i < length; i++ {
				elemRv := rv.Index(i)
				readReflectBinary(elemRv, elemRt, opts, r, lmt, n, err)
				if *err != nil {
					return
				}
				if lmt != 0 && lmt < *n {
					*err = ErrBinaryReadOverflow
					return
				}
			}
			//log.Info("Read x-array", "x", elemRt, "length", length, "n", *n)
		}

	case reflect.Slice:
		elemRt := rt.Elem()
		if elemRt.Kind() == reflect.Uint8 {
			// Special case: Byteslices
			byteslice := ReadByteSlice(r, lmt, n, err)
			//log.Info("Read byteslice", "bytes", byteslice, "n", *n)
			rv.Set(reflect.ValueOf(byteslice))
		} else {
			var sliceRv reflect.Value
			// Read length
			length := ReadVarint(r, n, err)
			//log.Info("Read slice", "length", length, "n", *n)
			sliceRv = reflect.MakeSlice(rt, 0, 0)
			// read one ReadSliceChunkSize at a time and append
			for i := 0; i*ReadSliceChunkSize < length; i++ {
				l := cmn.MinInt(ReadSliceChunkSize, length-i*ReadSliceChunkSize)
				tmpSliceRv := reflect.MakeSlice(rt, l, l)
				for j := 0; j < l; j++ {
					elemRv := tmpSliceRv.Index(j)
					readReflectBinary(elemRv, elemRt, opts, r, lmt, n, err)
					if *err != nil {
						return
					}
					if lmt != 0 && lmt < *n {
						*err = ErrBinaryReadOverflow
						return
					}
				}
				sliceRv = reflect.AppendSlice(sliceRv, tmpSliceRv)
			}

			rv.Set(sliceRv)
		}

	case reflect.Struct:
		if rt == timeType {
			// Special case: time.Time
			t := ReadTime(r, n, err)
			//log.Info("Read time", "t", t, "n", *n)
			rv.Set(reflect.ValueOf(t))
		} else {
			for _, fieldInfo := range typeInfo.Fields {
				fieldIdx, fieldType, opts := fieldInfo.unpack()
				fieldRv := rv.Field(fieldIdx)
				readReflectBinary(fieldRv, fieldType, opts, r, lmt, n, err)
			}
		}

	case reflect.String:
		str := ReadString(r, lmt, n, err)
		//log.Info("Read string", "str", str, "n", *n)
		rv.SetString(str)

	case reflect.Int64:
		if opts.Varint {
			num := ReadVarint(r, n, err)
			//log.Info("Read num", "num", num, "n", *n)
			rv.SetInt(int64(num))
		} else {
			num := ReadInt64(r, n, err)
			//log.Info("Read num", "num", num, "n", *n)
			rv.SetInt(int64(num))
		}

	case reflect.Int32:
		num := ReadUint32(r, n, err)
		//log.Info("Read num", "num", num, "n", *n)
		rv.SetInt(int64(num))

	case reflect.Int16:
		num := ReadUint16(r, n, err)
		//log.Info("Read num", "num", num, "n", *n)
		rv.SetInt(int64(num))

	case reflect.Int8:
		num := ReadUint8(r, n, err)
		//log.Info("Read num", "num", num, "n", *n)
		rv.SetInt(int64(num))

	case reflect.Int:
		num := ReadVarint(r, n, err)
		//log.Info("Read num", "num", num, "n", *n)
		rv.SetInt(int64(num))

	case reflect.Uint64:
		if opts.Varint {
			num := ReadVarint(r, n, err)
			//log.Info("Read num", "num", num, "n", *n)
			rv.SetUint(uint64(num))
		} else {
			num := ReadUint64(r, n, err)
			//log.Info("Read num", "num", num, "n", *n)
			rv.SetUint(uint64(num))
		}

	case reflect.Uint32:
		num := ReadUint32(r, n, err)
		//log.Info("Read num", "num", num, "n", *n)
		rv.SetUint(uint64(num))

	case reflect.Uint16:
		num := ReadUint16(r, n, err)
		//log.Info("Read num", "num", num, "n", *n)
		rv.SetUint(uint64(num))

	case reflect.Uint8:
		num := ReadUint8(r, n, err)
		//log.Info("Read num", "num", num, "n", *n)
		rv.SetUint(uint64(num))

	case reflect.Uint:
		num := ReadVarint(r, n, err)
		//log.Info("Read num", "num", num, "n", *n)
		rv.SetUint(uint64(num))

	case reflect.Bool:
		num := ReadUint8(r, n, err)
		//log.Info("Read bool", "bool", num, "n", *n)
		rv.SetBool(num > 0)

	case reflect.Float64:
		if !opts.Unsafe {
			*err = errors.New("Wire float* support requires `wire:\"unsafe\"`")
			return
		}
		num := ReadFloat64(r, n, err)
		//log.Info("Read num", "num", num, "n", *n)
		rv.SetFloat(float64(num))

	case reflect.Float32:
		if !opts.Unsafe {
			*err = errors.New("Wire float* support requires `wire:\"unsafe\"`")
			return
		}
		num := ReadFloat32(r, n, err)
		//log.Info("Read num", "num", num, "n", *n)
		rv.SetFloat(float64(num))

	default:
		cmn.PanicSanity(cmn.Fmt("Unknown field type %v", rt.Kind()))
	}
}

// rv: the reflection value of the thing to write
// rt: the type of rv as declared in the container, not necessarily rv.Type().
func writeReflectBinary(rv reflect.Value, rt reflect.Type, opts Options, w io.Writer, n *int, err *error) {

	// Get typeInfo
	typeInfo := GetTypeInfo(rt)

	if rt.Kind() == reflect.Interface {
		if rv.IsNil() {
			WriteByte(0x00, w, n, err)
			return
		}
		crv := rv.Elem()  // concrete reflection value
		crt := crv.Type() // concrete reflection type
		if typeInfo.IsRegisteredInterface {
			// See if the crt is registered.
			// If so, we're more restrictive.
			typeByte, ok := typeInfo.TypeToByte[crt]
			if !ok {
				switch crt.Kind() {
				case reflect.Ptr:
					*err = errors.New(cmn.Fmt("Unexpected pointer type %v for registered interface %v. "+
						"Was it registered as a value receiver rather than as a pointer receiver?", crt, rt.Name()))
				case reflect.Struct:
					*err = errors.New(cmn.Fmt("Unexpected struct type %v for registered interface %v. "+
						"Was it registered as a pointer receiver rather than as a value receiver?", crt, rt.Name()))
				default:
					*err = errors.New(cmn.Fmt("Unexpected type %v for registered interface %v. "+
						"If this is intentional, please register it.", crt, rt.Name()))
				}
				return
			}
			if crt.Kind() == reflect.Ptr {
				crv, crt = crv.Elem(), crt.Elem()
				if !crv.IsValid() {
					*err = errors.New(cmn.Fmt("Unexpected nil-pointer of type %v for registered interface %v. "+
						"For compatibility with other languages, nil-pointer interface values are forbidden.", crt, rt.Name()))
					return
				}
			}
			WriteByte(typeByte, w, n, err)
			writeReflectBinary(crv, crt, opts, w, n, err)
		} else {
			// We support writing unregistered interfaces for convenience.
			writeReflectBinary(crv, crt, opts, w, n, err)
		}
		return
	}

	if rt.Kind() == reflect.Ptr {
		// Dereference pointer
		rv, rt = rv.Elem(), rt.Elem()
		typeInfo = GetTypeInfo(rt)
		if !rv.IsValid() {
			WriteByte(0x00, w, n, err)
			return
		} else {
			WriteByte(0x01, w, n, err)
			// continue...
		}
	}

	// All other types
	switch rt.Kind() {
	case reflect.Array:
		elemRt := rt.Elem()
		length := rt.Len()
		if elemRt.Kind() == reflect.Uint8 {
			// Special case: Bytearrays
			if rv.CanAddr() {
				byteslice := rv.Slice(0, length).Bytes()
				WriteTo(byteslice, w, n, err)

			} else {
				buf := make([]byte, length)
				reflect.Copy(reflect.ValueOf(buf), rv) // XXX: looks expensive!
				WriteTo(buf, w, n, err)
			}
		} else {
			// Write elems
			for i := 0; i < length; i++ {
				elemRv := rv.Index(i)
				writeReflectBinary(elemRv, elemRt, opts, w, n, err)
			}
		}

	case reflect.Slice:
		elemRt := rt.Elem()
		if elemRt.Kind() == reflect.Uint8 {
			// Special case: Byteslices
			byteslice := rv.Bytes()
			WriteByteSlice(byteslice, w, n, err)
		} else {
			// Write length
			length := rv.Len()
			WriteVarint(length, w, n, err)
			// Write elems
			for i := 0; i < length; i++ {
				elemRv := rv.Index(i)
				writeReflectBinary(elemRv, elemRt, opts, w, n, err)
			}
		}

	case reflect.Struct:
		if rt == timeType {
			// Special case: time.Time
			WriteTime(rv.Interface().(time.Time), w, n, err)
		} else {
			for _, fieldInfo := range typeInfo.Fields {
				fieldIdx, fieldType, opts := fieldInfo.unpack()
				fieldRv := rv.Field(fieldIdx)
				writeReflectBinary(fieldRv, fieldType, opts, w, n, err)
			}
		}

	case reflect.String:
		WriteString(rv.String(), w, n, err)

	case reflect.Int64:
		if opts.Varint {
			WriteVarint(int(rv.Int()), w, n, err)
		} else {
			WriteInt64(rv.Int(), w, n, err)
		}

	case reflect.Int32:
		WriteInt32(int32(rv.Int()), w, n, err)

	case reflect.Int16:
		WriteInt16(int16(rv.Int()), w, n, err)

	case reflect.Int8:
		WriteInt8(int8(rv.Int()), w, n, err)

	case reflect.Int:
		WriteVarint(int(rv.Int()), w, n, err)

	case reflect.Uint64:
		if opts.Varint {
			WriteUvarint(uint(rv.Uint()), w, n, err)
		} else {
			WriteUint64(rv.Uint(), w, n, err)
		}

	case reflect.Uint32:
		WriteUint32(uint32(rv.Uint()), w, n, err)

	case reflect.Uint16:
		WriteUint16(uint16(rv.Uint()), w, n, err)

	case reflect.Uint8:
		WriteUint8(uint8(rv.Uint()), w, n, err)

	case reflect.Uint:
		WriteUvarint(uint(rv.Uint()), w, n, err)

	case reflect.Bool:
		if rv.Bool() {
			WriteUint8(uint8(1), w, n, err)
		} else {
			WriteUint8(uint8(0), w, n, err)
		}

	case reflect.Float64:
		if !opts.Unsafe {
			*err = errors.New("Wire float* support requires `wire:\"unsafe\"`")
			return
		}
		WriteFloat64(rv.Float(), w, n, err)

	case reflect.Float32:
		if !opts.Unsafe {
			*err = errors.New("Wire float* support requires `wire:\"unsafe\"`")
			return
		}
		WriteFloat32(float32(rv.Float()), w, n, err)

	default:
		cmn.PanicSanity(cmn.Fmt("Unknown field type %v", rt.Kind()))
	}
}

//-----------------------------------------------------------------------------

func readByteJSON(o interface{}) (typeByte byte, rest interface{}, err error) {
	oSlice, ok := o.([]interface{})
	if !ok {
		err = errors.New(cmn.Fmt("Expected type [Byte,?] but got type %v", reflect.TypeOf(o)))
		return
	}
	if len(oSlice) != 2 {
		err = errors.New(cmn.Fmt("Expected [Byte,?] len 2 but got len %v", len(oSlice)))
		return
	}
	typeByte_, ok := oSlice[0].(float64)
	typeByte = byte(typeByte_)
	rest = oSlice[1]
	return
}

// Contract: Caller must ensure that rt is supported
// (e.g. is recursively composed of supported native types, and structs and slices.)
// rv and rt refer to the object we're unmarhsaling into, whereas o is the result of naiive json unmarshal (map[string]interface{})
func readReflectJSON(rv reflect.Value, rt reflect.Type, opts Options, o interface{}, err *error) {

	// Get typeInfo
	typeInfo := GetTypeInfo(rt)

	if rt.Kind() == reflect.Interface {
		if !typeInfo.IsRegisteredInterface {
			// There's no way we can read such a thing.
			*err = errors.New(cmn.Fmt("Cannot read unregistered interface type %v", rt))
			return
		}
		if o == nil {
			return // nil
		}
		typeByte, rest, err_ := readByteJSON(o)
		if err_ != nil {
			*err = err_
			return
		}
		crt, ok := typeInfo.ByteToType[typeByte]
		if !ok {
			*err = errors.New(cmn.Fmt("Byte %X not registered for interface %v", typeByte, rt))
			return
		}
		if crt.Kind() == reflect.Ptr {
			crt = crt.Elem()
			crv := reflect.New(crt)
			readReflectJSON(crv.Elem(), crt, opts, rest, err)
			rv.Set(crv) // NOTE: orig rv is ignored.
		} else {
			crv := reflect.New(crt).Elem()
			readReflectJSON(crv, crt, opts, rest, err)
			rv.Set(crv) // NOTE: orig rv is ignored.
		}
		return
	}

	if rt.Kind() == reflect.Ptr {
		if o == nil {
			return // nil
		}
		// Create new struct if rv is nil.
		if rv.IsNil() {
			newRv := reflect.New(rt.Elem())
			rv.Set(newRv)
			rv = newRv
		}
		// Dereference pointer
		rv, rt = rv.Elem(), rt.Elem()
		typeInfo = GetTypeInfo(rt)
		// continue...
	}

	switch rt.Kind() {
	case reflect.Array:
		elemRt := rt.Elem()
		length := rt.Len()
		if elemRt.Kind() == reflect.Uint8 {
			// Special case: Bytearrays
			oString, ok := o.(string)
			if !ok {
				*err = errors.New(cmn.Fmt("Expected string but got type %v", reflect.TypeOf(o)))
				return
			}
			buf, err_ := hex.DecodeString(oString)
			if err_ != nil {
				*err = err_
				return
			}
			if len(buf) != length {
				*err = errors.New(cmn.Fmt("Expected bytearray of length %v but got %v", length, len(buf)))
				return
			}
			//log.Info("Read bytearray", "bytes", buf)
			reflect.Copy(rv, reflect.ValueOf(buf))
		} else {
			oSlice, ok := o.([]interface{})
			if !ok {
				*err = errors.New(cmn.Fmt("Expected array of %v but got type %v", rt, reflect.TypeOf(o)))
				return
			}
			if len(oSlice) != length {
				*err = errors.New(cmn.Fmt("Expected array of length %v but got %v", length, len(oSlice)))
				return
			}
			for i := 0; i < length; i++ {
				elemRv := rv.Index(i)
				readReflectJSON(elemRv, elemRt, opts, oSlice[i], err)
			}
			//log.Info("Read x-array", "x", elemRt, "length", length)
		}

	case reflect.Slice:
		elemRt := rt.Elem()
		if elemRt.Kind() == reflect.Uint8 {
			// Special case: Byteslices
			oString, ok := o.(string)
			if !ok {
				*err = errors.New(cmn.Fmt("Expected string but got type %v", reflect.TypeOf(o)))
				return
			}
			byteslice, err_ := hex.DecodeString(oString)
			if err_ != nil {
				*err = err_
				return
			}
			//log.Info("Read byteslice", "bytes", byteslice)
			rv.Set(reflect.ValueOf(byteslice))
		} else {
			// Read length
			oSlice, ok := o.([]interface{})
			if !ok {
				*err = errors.New(cmn.Fmt("Expected array of %v but got type %v", rt, reflect.TypeOf(o)))
				return
			}
			length := len(oSlice)
			//log.Info("Read slice", "length", length)
			sliceRv := reflect.MakeSlice(rt, length, length)
			// Read elems
			for i := 0; i < length; i++ {
				elemRv := sliceRv.Index(i)
				readReflectJSON(elemRv, elemRt, opts, oSlice[i], err)
			}
			rv.Set(sliceRv)
		}

	case reflect.Struct:
		if rt == timeType {
			// Special case: time.Time
			str, ok := o.(string)
			if !ok {
				*err = errors.New(cmn.Fmt("Expected string but got type %v", reflect.TypeOf(o)))
				return
			}
			// try three ways, seconds, milliseconds, or microseconds...
			t, err_ := time.Parse(time.RFC3339Nano, str)
			if err_ != nil {
				*err = err_
				return
			}
			rv.Set(reflect.ValueOf(t))
		} else {
			if typeInfo.Unwrap {
				fieldIdx, fieldType, opts := typeInfo.Fields[0].unpack()
				fieldRv := rv.Field(fieldIdx)
				readReflectJSON(fieldRv, fieldType, opts, o, err)
			} else {
				oMap, ok := o.(map[string]interface{})
				if !ok {
					*err = errors.New(cmn.Fmt("Expected map but got type %v", reflect.TypeOf(o)))
					return
				}
				// TODO: ensure that all fields are set?
				// TODO: disallow unknown oMap fields?
				for _, fieldInfo := range typeInfo.Fields {
					fieldIdx, fieldType, opts := fieldInfo.unpack()
					value, ok := oMap[opts.JSONName]
					if !ok {
						continue // Skip missing fields.
					}
					fieldRv := rv.Field(fieldIdx)
					readReflectJSON(fieldRv, fieldType, opts, value, err)
				}
			}
		}

	case reflect.String:
		str, ok := o.(string)
		if !ok {
			*err = errors.New(cmn.Fmt("Expected string but got type %v", reflect.TypeOf(o)))
			return
		}
		//log.Info("Read string", "str", str)
		rv.SetString(str)

	case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
		num, ok := o.(float64)
		if !ok {
			*err = errors.New(cmn.Fmt("Expected numeric but got type %v", reflect.TypeOf(o)))
			return
		}
		//log.Info("Read num", "num", num)
		rv.SetInt(int64(num))

	case reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint:
		num, ok := o.(float64)
		if !ok {
			*err = errors.New(cmn.Fmt("Expected numeric but got type %v", reflect.TypeOf(o)))
			return
		}
		if num < 0 {
			*err = errors.New(cmn.Fmt("Expected unsigned numeric but got %v", num))
			return
		}
		//log.Info("Read num", "num", num)
		rv.SetUint(uint64(num))

	case reflect.Float64, reflect.Float32:
		if !opts.Unsafe {
			*err = errors.New("Wire float* support requires `wire:\"unsafe\"`")
			return
		}
		num, ok := o.(float64)
		if !ok {
			*err = errors.New(cmn.Fmt("Expected numeric but got type %v", reflect.TypeOf(o)))
			return
		}
		//log.Info("Read num", "num", num)
		rv.SetFloat(num)

	case reflect.Bool:
		bl, ok := o.(bool)
		if !ok {
			*err = errors.New(cmn.Fmt("Expected boolean but got type %v", reflect.TypeOf(o)))
			return
		}
		//log.Info("Read boolean", "boolean", bl)
		rv.SetBool(bl)

	default:
		cmn.PanicSanity(cmn.Fmt("Unknown field type %v", rt.Kind()))
	}
}

func writeReflectJSON(rv reflect.Value, rt reflect.Type, opts Options, w io.Writer, n *int, err *error) {
	//log.Info(cmn.Fmt("writeReflectJSON(%v, %v, %v, %v, %v)", rv, rt, w, n, err))

	// Get typeInfo
	typeInfo := GetTypeInfo(rt)

	if rt.Kind() == reflect.Interface {
		if rv.IsNil() {
			WriteTo([]byte("null"), w, n, err)
			return
		}
		crv := rv.Elem()  // concrete reflection value
		crt := crv.Type() // concrete reflection type
		if typeInfo.IsRegisteredInterface {
			// See if the crt is registered.
			// If so, we're more restrictive.
			typeByte, ok := typeInfo.TypeToByte[crt]
			if !ok {
				switch crt.Kind() {
				case reflect.Ptr:
					*err = errors.New(cmn.Fmt("Unexpected pointer type %v for registered interface %v. "+
						"Was it registered as a value receiver rather than as a pointer receiver?", crt, rt.Name()))
				case reflect.Struct:
					*err = errors.New(cmn.Fmt("Unexpected struct type %v for registered interface %v. "+
						"Was it registered as a pointer receiver rather than as a value receiver?", crt, rt.Name()))
				default:
					*err = errors.New(cmn.Fmt("Unexpected type %v for registered interface %v. "+
						"If this is intentional, please register it.", crt, rt.Name()))
				}
				return
			}
			if crt.Kind() == reflect.Ptr {
				crv, crt = crv.Elem(), crt.Elem()
				if !crv.IsValid() {
					*err = errors.New(cmn.Fmt("Unexpected nil-pointer of type %v for registered interface %v. "+
						"For compatibility with other languages, nil-pointer interface values are forbidden.", crt, rt.Name()))
					return
				}
			}
			WriteTo([]byte(cmn.Fmt("[%v,", typeByte)), w, n, err)
			writeReflectJSON(crv, crt, opts, w, n, err)
			WriteTo([]byte("]"), w, n, err)
		} else {
			// We support writing unregistered interfaces for convenience.
			writeReflectJSON(crv, crt, opts, w, n, err)
		}
		return
	}

	if rt.Kind() == reflect.Ptr {
		// Dereference pointer
		rv, rt = rv.Elem(), rt.Elem()
		typeInfo = GetTypeInfo(rt)
		if !rv.IsValid() {
			// For better compatibility with other languages,
			// as far as tendermint/wire is concerned,
			// pointers to nil values are the same as nil.
			WriteTo([]byte("null"), w, n, err)
			return
		}
		// continue...
	}

	// All other types
	switch rt.Kind() {
	case reflect.Array:
		elemRt := rt.Elem()
		length := rt.Len()
		if elemRt.Kind() == reflect.Uint8 {
			// Special case: Bytearray
			bytearray := reflect.ValueOf(make([]byte, length))
			reflect.Copy(bytearray, rv)
			WriteTo([]byte(cmn.Fmt("\"%X\"", bytearray.Interface())), w, n, err)
		} else {
			WriteTo([]byte("["), w, n, err)
			// Write elems
			for i := 0; i < length; i++ {
				elemRv := rv.Index(i)
				writeReflectJSON(elemRv, elemRt, opts, w, n, err)
				if i < length-1 {
					WriteTo([]byte(","), w, n, err)
				}
			}
			WriteTo([]byte("]"), w, n, err)
		}

	case reflect.Slice:
		elemRt := rt.Elem()
		if elemRt.Kind() == reflect.Uint8 {
			// Special case: Byteslices
			byteslice := rv.Bytes()
			WriteTo([]byte(cmn.Fmt("\"%X\"", byteslice)), w, n, err)
		} else {
			WriteTo([]byte("["), w, n, err)
			// Write elems
			length := rv.Len()
			for i := 0; i < length; i++ {
				elemRv := rv.Index(i)
				writeReflectJSON(elemRv, elemRt, opts, w, n, err)
				if i < length-1 {
					WriteTo([]byte(","), w, n, err)
				}
			}
			WriteTo([]byte("]"), w, n, err)
		}

	case reflect.Struct:
		if rt == timeType {
			// Special case: time.Time
			t := rv.Interface().(time.Time).UTC()
			str := t.Format(RFC3339Millis)
			jsonBytes, err_ := json.Marshal(str)
			if err_ != nil {
				*err = err_
				return
			}
			WriteTo(jsonBytes, w, n, err)
		} else {
			if typeInfo.Unwrap {
				fieldIdx, fieldType, opts := typeInfo.Fields[0].unpack()
				fieldRv := rv.Field(fieldIdx)
				writeReflectJSON(fieldRv, fieldType, opts, w, n, err)
			} else {
				WriteTo([]byte("{"), w, n, err)
				wroteField := false
				for _, fieldInfo := range typeInfo.Fields {
					fieldIdx, fieldType, opts := fieldInfo.unpack()
					fieldRv := rv.Field(fieldIdx)
					if opts.JSONOmitEmpty && isEmpty(fieldType, fieldRv, opts) { // Skip zero value if omitempty
						continue
					}
					if wroteField {
						WriteTo([]byte(","), w, n, err)
					} else {
						wroteField = true
					}
					WriteTo([]byte(cmn.Fmt("\"%v\":", opts.JSONName)), w, n, err)
					writeReflectJSON(fieldRv, fieldType, opts, w, n, err)
				}
				WriteTo([]byte("}"), w, n, err)
			}
		}

	case reflect.String:
		fallthrough
	case reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint:
		fallthrough
	case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
		fallthrough
	case reflect.Bool:
		jsonBytes, err_ := json.Marshal(rv.Interface())
		if err_ != nil {
			*err = err_
			return
		}
		WriteTo(jsonBytes, w, n, err)

	case reflect.Float64, reflect.Float32:
		if !opts.Unsafe {
			*err = errors.New("Wire float* support requires `wire:\"unsafe\"`")
			return
		}
		jsonBytes, err_ := json.Marshal(rv.Interface())
		if err_ != nil {
			*err = err_
			return
		}
		WriteTo(jsonBytes, w, n, err)

	default:
		cmn.PanicSanity(cmn.Fmt("Unknown field type %v", rt.Kind()))
	}

}

func isEmpty(rt reflect.Type, rv reflect.Value, opts Options) bool {
	if rt.Comparable() {
		// if its comparable we can check directly
		if rv.Interface() == opts.ZeroValue {
			return true
		}
		return false
	} else {
		// TODO: A faster alternative might be to call writeReflectJSON
		// onto a buffer and check if its "{}" or not.
		switch rt.Kind() {
		case reflect.Struct:
			// check fields
			typeInfo := GetTypeInfo(rt)
			for _, fieldInfo := range typeInfo.Fields {
				fieldIdx, fieldType, opts := fieldInfo.unpack()
				fieldRv := rv.Field(fieldIdx)
				if !isEmpty(fieldType, fieldRv, opts) { // Skip zero value if omitempty
					return false
				}
			}
			return true

		default:
			if rv.Len() == 0 {
				return true
			}
			return false
		}
	}
	return false
}
