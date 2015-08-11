package wire

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"sync"
	"time"

	. "github.com/tendermint/tendermint/common"
)

const (
	ReflectSliceChunk = 1024
)

type TypeInfo struct {
	Type reflect.Type // The type

	// If Type is kind reflect.Interface, is registered
	IsRegisteredInterface bool
	ByteToType            map[byte]reflect.Type
	TypeToByte            map[reflect.Type]byte

	// If Type is concrete
	Byte byte

	// If Type is kind reflect.Struct
	Fields []StructFieldInfo
}

type Options struct {
	JSONName string // (JSON) Corresponding JSON field name. (override with `json=""`)
	Varint   bool   // (Binary) Use length-prefixed encoding for (u)int*
}

func getOptionsFromField(field reflect.StructField) (skip bool, opts Options) {
	jsonName := field.Tag.Get("json")
	if jsonName == "-" {
		skip = true
		return
	} else if jsonName == "" {
		jsonName = field.Name
	}
	varint := false
	binTag := field.Tag.Get("binary")
	if binTag == "varint" { // TODO: extend
		varint = true
	}
	opts = Options{
		JSONName: jsonName,
		Varint:   varint,
	}
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
		PanicSanity("Unexpected number of fields in struct-wrapped declaration of type")
	}
	return rt.Field(0).Type
}

func SetByteForType(typeByte byte, rt reflect.Type) {
	typeInfo := GetTypeInfo(rt)
	if typeInfo.Byte != 0x00 && typeInfo.Byte != typeByte {
		PanicSanity(Fmt("Type %v already registered with type byte %X", rt, typeByte))
	}
	typeInfo.Byte = typeByte
	// If pointer, we need to set it for the concrete type as well.
	if rt.Kind() == reflect.Ptr {
		SetByteForType(typeByte, rt.Elem())
	}
}

// Predeclaration of common types
var (
	timeType = GetTypeFromStructDeclaration(struct{ time.Time }{})
)

const (
	rfc2822 = "Mon Jan 02 15:04:05 -0700 2006"
)

// NOTE: do not access typeInfos directly, but call GetTypeInfo()
var typeInfosMtx sync.Mutex
var typeInfos = map[reflect.Type]*TypeInfo{}

func GetTypeInfo(rt reflect.Type) *TypeInfo {
	typeInfosMtx.Lock()
	defer typeInfosMtx.Unlock()
	info := typeInfos[rt]
	if info == nil {
		info = MakeTypeInfo(rt)
		typeInfos[rt] = info
	}
	return info
}

// For use with the RegisterInterface declaration
type ConcreteType struct {
	O    interface{}
	Byte byte
}

// Must use this to register an interface to properly decode the
// underlying concrete type.
func RegisterInterface(o interface{}, ctypes ...ConcreteType) *TypeInfo {
	it := GetTypeFromStructDeclaration(o)
	if it.Kind() != reflect.Interface {
		PanicSanity("RegisterInterface expects an interface")
	}
	toType := make(map[byte]reflect.Type, 0)
	toByte := make(map[reflect.Type]byte, 0)
	for _, ctype := range ctypes {
		crt := reflect.TypeOf(ctype.O)
		typeByte := ctype.Byte
		SetByteForType(typeByte, crt)
		if typeByte == 0x00 {
			PanicSanity(Fmt("Byte of 0x00 is reserved for nil (%v)", ctype))
		}
		if toType[typeByte] != nil {
			PanicSanity(Fmt("Duplicate Byte for type %v and %v", ctype, toType[typeByte]))
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
	}

	return info
}

// Contract: Caller must ensure that rt is supported
// (e.g. is recursively composed of supported native types, and structs and slices.)
func readReflectBinary(rv reflect.Value, rt reflect.Type, opts Options, r io.Reader, n *int64, err *error) {

	// Get typeInfo
	typeInfo := GetTypeInfo(rt)

	if rt.Kind() == reflect.Interface {
		if !typeInfo.IsRegisteredInterface {
			// There's no way we can read such a thing.
			*err = errors.New(Fmt("Cannot read unregistered interface type %v", rt))
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
			*err = errors.New(Fmt("Unexpected type byte %X for type %v", typeByte, rt))
			return
		}
		crv := reflect.New(crt).Elem()
		r = NewPrefixedReader([]byte{typeByte}, r)
		readReflectBinary(crv, crt, opts, r, n, err)
		rv.Set(crv) // NOTE: orig rv is ignored.
		return
	}

	if rt.Kind() == reflect.Ptr {
		typeByte := ReadByte(r, n, err)
		if *err != nil {
			return
		}
		if typeByte == 0x00 {
			return // nil
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
		if typeInfo.Byte != 0x00 {
			r = NewPrefixedReader([]byte{typeByte}, r)
		} else if typeByte != 0x01 {
			*err = errors.New(Fmt("Unexpected type byte %X for ptr of untyped thing", typeByte))
			return
		}
		// continue...
	}

	// Read Byte prefix
	if typeInfo.Byte != 0x00 {
		typeByte := ReadByte(r, n, err)
		if typeByte != typeInfo.Byte {
			*err = errors.New(Fmt("Expected Byte of %X but got %X", typeInfo.Byte, typeByte))
			return
		}
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
			log.Info("Read bytearray", "bytes", buf)
			reflect.Copy(rv, reflect.ValueOf(buf))
		} else {
			for i := 0; i < length; i++ {
				elemRv := rv.Index(i)
				readReflectBinary(elemRv, elemRt, opts, r, n, err)
				if *err != nil {
					return
				}
				if MaxBinaryReadSize < *n {
					*err = ErrBinaryReadSizeOverflow
					return
				}
			}
			log.Info(Fmt("Read %v-array", elemRt), "length", length)
		}

	case reflect.Slice:
		elemRt := rt.Elem()
		if elemRt.Kind() == reflect.Uint8 {
			// Special case: Byteslices
			byteslice := ReadByteSlice(r, n, err)
			log.Info("Read byteslice", "bytes", byteslice)
			rv.Set(reflect.ValueOf(byteslice))
		} else {
			var sliceRv reflect.Value
			// Read length
			length := ReadVarint(r, n, err)
			log.Info(Fmt("Read length: %v", length))
			sliceRv = reflect.MakeSlice(rt, 0, 0)
			// read one ReflectSliceChunk at a time and append
			for i := 0; i*ReflectSliceChunk < length; i++ {
				l := MinInt(ReflectSliceChunk, length-i*ReflectSliceChunk)
				tmpSliceRv := reflect.MakeSlice(rt, l, l)
				for j := 0; j < l; j++ {
					elemRv := tmpSliceRv.Index(j)
					readReflectBinary(elemRv, elemRt, opts, r, n, err)
					if *err != nil {
						return
					}
					if MaxBinaryReadSize < *n {
						*err = ErrBinaryReadSizeOverflow
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
			log.Info(Fmt("Read time: %v", t))
			rv.Set(reflect.ValueOf(t))
		} else {
			for _, fieldInfo := range typeInfo.Fields {
				i, fieldType, opts := fieldInfo.unpack()
				fieldRv := rv.Field(i)
				readReflectBinary(fieldRv, fieldType, opts, r, n, err)
			}
		}

	case reflect.String:
		str := ReadString(r, n, err)
		log.Info(Fmt("Read string: %v", str))
		rv.SetString(str)

	case reflect.Int64:
		if opts.Varint {
			num := ReadVarint(r, n, err)
			log.Info(Fmt("Read num: %v", num))
			rv.SetInt(int64(num))
		} else {
			num := ReadInt64(r, n, err)
			log.Info(Fmt("Read num: %v", num))
			rv.SetInt(int64(num))
		}

	case reflect.Int32:
		num := ReadUint32(r, n, err)
		log.Info(Fmt("Read num: %v", num))
		rv.SetInt(int64(num))

	case reflect.Int16:
		num := ReadUint16(r, n, err)
		log.Info(Fmt("Read num: %v", num))
		rv.SetInt(int64(num))

	case reflect.Int8:
		num := ReadUint8(r, n, err)
		log.Info(Fmt("Read num: %v", num))
		rv.SetInt(int64(num))

	case reflect.Int:
		num := ReadVarint(r, n, err)
		log.Info(Fmt("Read num: %v", num))
		rv.SetInt(int64(num))

	case reflect.Uint64:
		if opts.Varint {
			num := ReadVarint(r, n, err)
			log.Info(Fmt("Read num: %v", num))
			rv.SetUint(uint64(num))
		} else {
			num := ReadUint64(r, n, err)
			log.Info(Fmt("Read num: %v", num))
			rv.SetUint(uint64(num))
		}

	case reflect.Uint32:
		num := ReadUint32(r, n, err)
		log.Info(Fmt("Read num: %v", num))
		rv.SetUint(uint64(num))

	case reflect.Uint16:
		num := ReadUint16(r, n, err)
		log.Info(Fmt("Read num: %v", num))
		rv.SetUint(uint64(num))

	case reflect.Uint8:
		num := ReadUint8(r, n, err)
		log.Info(Fmt("Read num: %v", num))
		rv.SetUint(uint64(num))

	case reflect.Uint:
		num := ReadVarint(r, n, err)
		log.Info(Fmt("Read num: %v", num))
		rv.SetUint(uint64(num))

	case reflect.Bool:
		num := ReadUint8(r, n, err)
		log.Info(Fmt("Read bool: %v", num))
		rv.SetBool(num > 0)

	default:
		PanicSanity(Fmt("Unknown field type %v", rt.Kind()))
	}
}

// rv: the reflection value of the thing to write
// rt: the type of rv as declared in the container, not necessarily rv.Type().
func writeReflectBinary(rv reflect.Value, rt reflect.Type, opts Options, w io.Writer, n *int64, err *error) {

	// Get typeInfo
	typeInfo := GetTypeInfo(rt)

	if rt.Kind() == reflect.Interface {
		if rv.IsNil() {
			// XXX ensure that typeByte 0 is reserved.
			WriteByte(0x00, w, n, err)
			return
		}
		crv := rv.Elem()  // concrete reflection value
		crt := crv.Type() // concrete reflection type
		if typeInfo.IsRegisteredInterface {
			// See if the crt is registered.
			// If so, we're more restrictive.
			_, ok := typeInfo.TypeToByte[crt]
			if !ok {
				switch crt.Kind() {
				case reflect.Ptr:
					*err = errors.New(Fmt("Unexpected pointer type %v. Was it registered as a value receiver rather than as a pointer receiver?", crt))
				case reflect.Struct:
					*err = errors.New(Fmt("Unexpected struct type %v. Was it registered as a pointer receiver rather than as a value receiver?", crt))
				default:
					*err = errors.New(Fmt("Unexpected type %v.", crt))
				}
				return
			}
		} else {
			// We support writing unsafely for convenience.
		}
		// We don't have to write the typeByte here,
		// the writeReflectBinary() call below will write it.
		writeReflectBinary(crv, crt, opts, w, n, err)
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
			WriteByte(0x00, w, n, err)
			return
		}
		if typeInfo.Byte == 0x00 {
			WriteByte(0x01, w, n, err)
			// continue...
		} else {
			// continue...
		}
	}

	// Write type byte
	if typeInfo.Byte != 0x00 {
		WriteByte(typeInfo.Byte, w, n, err)
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
				reflect.Copy(reflect.ValueOf(buf), rv)
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
				i, fieldType, opts := fieldInfo.unpack()
				fieldRv := rv.Field(i)
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

	default:
		PanicSanity(Fmt("Unknown field type %v", rt.Kind()))
	}
}

//-----------------------------------------------------------------------------

func readByteJSON(o interface{}) (typeByte byte, rest interface{}, err error) {
	oSlice, ok := o.([]interface{})
	if !ok {
		err = errors.New(Fmt("Expected type [Byte,?] but got type %v", reflect.TypeOf(o)))
		return
	}
	if len(oSlice) != 2 {
		err = errors.New(Fmt("Expected [Byte,?] len 2 but got len %v", len(oSlice)))
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
func readReflectJSON(rv reflect.Value, rt reflect.Type, o interface{}, err *error) {

	// Get typeInfo
	typeInfo := GetTypeInfo(rt)

	if rt.Kind() == reflect.Interface {
		if !typeInfo.IsRegisteredInterface {
			// There's no way we can read such a thing.
			*err = errors.New(Fmt("Cannot read unregistered interface type %v", rt))
			return
		}
		if o == nil {
			return // nil
		}
		typeByte, _, err_ := readByteJSON(o)
		if err_ != nil {
			*err = err_
			return
		}
		crt, ok := typeInfo.ByteToType[typeByte]
		if !ok {
			*err = errors.New(Fmt("Byte %X not registered for interface %v", typeByte, rt))
			return
		}
		crv := reflect.New(crt).Elem()
		readReflectJSON(crv, crt, o, err)
		rv.Set(crv) // NOTE: orig rv is ignored.
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

	// Read Byte prefix
	if typeInfo.Byte != 0x00 {
		typeByte, rest, err_ := readByteJSON(o)
		if err_ != nil {
			*err = err_
			return
		}
		if typeByte != typeInfo.Byte {
			*err = errors.New(Fmt("Expected Byte of %X but got %X", typeInfo.Byte, byte(typeByte)))
			return
		}
		o = rest
	}

	switch rt.Kind() {
	case reflect.Array:
		elemRt := rt.Elem()
		length := rt.Len()
		if elemRt.Kind() == reflect.Uint8 {
			// Special case: Bytearrays
			oString, ok := o.(string)
			if !ok {
				*err = errors.New(Fmt("Expected string but got type %v", reflect.TypeOf(o)))
				return
			}
			buf, err_ := hex.DecodeString(oString)
			if err_ != nil {
				*err = err_
				return
			}
			if len(buf) != length {
				*err = errors.New(Fmt("Expected bytearray of length %v but got %v", length, len(buf)))
				return
			}
			log.Info("Read bytearray", "bytes", buf)
			reflect.Copy(rv, reflect.ValueOf(buf))
		} else {
			oSlice, ok := o.([]interface{})
			if !ok {
				*err = errors.New(Fmt("Expected array of %v but got type %v", rt, reflect.TypeOf(o)))
				return
			}
			if len(oSlice) != length {
				*err = errors.New(Fmt("Expected array of length %v but got %v", length, len(oSlice)))
				return
			}
			for i := 0; i < length; i++ {
				elemRv := rv.Index(i)
				readReflectJSON(elemRv, elemRt, oSlice[i], err)
			}
			log.Info(Fmt("Read %v-array", elemRt), "length", length)
		}

	case reflect.Slice:
		elemRt := rt.Elem()
		if elemRt.Kind() == reflect.Uint8 {
			// Special case: Byteslices
			oString, ok := o.(string)
			if !ok {
				*err = errors.New(Fmt("Expected string but got type %v", reflect.TypeOf(o)))
				return
			}
			byteslice, err_ := hex.DecodeString(oString)
			if err_ != nil {
				*err = err_
				return
			}
			log.Info("Read byteslice", "bytes", byteslice)
			rv.Set(reflect.ValueOf(byteslice))
		} else {
			// Read length
			oSlice, ok := o.([]interface{})
			if !ok {
				*err = errors.New(Fmt("Expected array of %v but got type %v", rt, reflect.TypeOf(o)))
				return
			}
			length := len(oSlice)
			log.Info(Fmt("Read length: %v", length))
			sliceRv := reflect.MakeSlice(rt, length, length)
			// Read elems
			for i := 0; i < length; i++ {
				elemRv := sliceRv.Index(i)
				readReflectJSON(elemRv, elemRt, oSlice[i], err)
			}
			rv.Set(sliceRv)
		}

	case reflect.Struct:
		if rt == timeType {
			// Special case: time.Time
			str, ok := o.(string)
			if !ok {
				*err = errors.New(Fmt("Expected string but got type %v", reflect.TypeOf(o)))
				return
			}
			log.Info(Fmt("Read time: %v", str))
			t, err_ := time.Parse(rfc2822, str)
			if err_ != nil {
				*err = err_
				return
			}
			rv.Set(reflect.ValueOf(t))
		} else {
			oMap, ok := o.(map[string]interface{})
			if !ok {
				*err = errors.New(Fmt("Expected map but got type %v", reflect.TypeOf(o)))
				return
			}
			// TODO: ensure that all fields are set?
			// TODO: disallow unknown oMap fields?
			for _, fieldInfo := range typeInfo.Fields {
				i, fieldType, opts := fieldInfo.unpack()
				value, ok := oMap[opts.JSONName]
				if !ok {
					continue // Skip missing fields.
				}
				fieldRv := rv.Field(i)
				readReflectJSON(fieldRv, fieldType, value, err)
			}
		}

	case reflect.String:
		str, ok := o.(string)
		if !ok {
			*err = errors.New(Fmt("Expected string but got type %v", reflect.TypeOf(o)))
			return
		}
		log.Info(Fmt("Read string: %v", str))
		rv.SetString(str)

	case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
		num, ok := o.(float64)
		if !ok {
			*err = errors.New(Fmt("Expected numeric but got type %v", reflect.TypeOf(o)))
			return
		}
		log.Info(Fmt("Read num: %v", num))
		rv.SetInt(int64(num))

	case reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint:
		num, ok := o.(float64)
		if !ok {
			*err = errors.New(Fmt("Expected numeric but got type %v", reflect.TypeOf(o)))
			return
		}
		if num < 0 {
			*err = errors.New(Fmt("Expected unsigned numeric but got %v", num))
			return
		}
		log.Info(Fmt("Read num: %v", num))
		rv.SetUint(uint64(num))

	case reflect.Bool:
		bl, ok := o.(bool)
		if !ok {
			*err = errors.New(Fmt("Expected boolean but got type %v", reflect.TypeOf(o)))
			return
		}
		log.Info(Fmt("Read boolean: %v", bl))
		rv.SetBool(bl)

	default:
		PanicSanity(Fmt("Unknown field type %v", rt.Kind()))
	}
}

func writeReflectJSON(rv reflect.Value, rt reflect.Type, w io.Writer, n *int64, err *error) {
	log.Info(Fmt("writeReflectJSON(%v, %v, %v, %v, %v)", rv, rt, w, n, err))

	// Get typeInfo
	typeInfo := GetTypeInfo(rt)

	if rt.Kind() == reflect.Interface {
		if rv.IsNil() {
			// XXX ensure that typeByte 0 is reserved.
			WriteTo([]byte("null"), w, n, err)
			return
		}
		crv := rv.Elem()  // concrete reflection value
		crt := crv.Type() // concrete reflection type
		if typeInfo.IsRegisteredInterface {
			// See if the crt is registered.
			// If so, we're more restrictive.
			_, ok := typeInfo.TypeToByte[crt]
			if !ok {
				switch crt.Kind() {
				case reflect.Ptr:
					*err = errors.New(Fmt("Unexpected pointer type %v. Was it registered as a value receiver rather than as a pointer receiver?", crt))
				case reflect.Struct:
					*err = errors.New(Fmt("Unexpected struct type %v. Was it registered as a pointer receiver rather than as a value receiver?", crt))
				default:
					*err = errors.New(Fmt("Unexpected type %v.", crt))
				}
				return
			}
		} else {
			// We support writing unsafely for convenience.
		}
		// We don't have to write the typeByte here,
		// the writeReflectJSON() call below will write it.
		writeReflectJSON(crv, crt, w, n, err)
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

	// Write Byte
	if typeInfo.Byte != 0x00 {
		WriteTo([]byte(Fmt("[%v,", typeInfo.Byte)), w, n, err)
		defer WriteTo([]byte("]"), w, n, err)
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
			WriteTo([]byte(Fmt("\"%X\"", bytearray.Interface())), w, n, err)
		} else {
			WriteTo([]byte("["), w, n, err)
			// Write elems
			for i := 0; i < length; i++ {
				elemRv := rv.Index(i)
				writeReflectJSON(elemRv, elemRt, w, n, err)
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
			WriteTo([]byte(Fmt("\"%X\"", byteslice)), w, n, err)
		} else {
			WriteTo([]byte("["), w, n, err)
			// Write elems
			length := rv.Len()
			for i := 0; i < length; i++ {
				elemRv := rv.Index(i)
				writeReflectJSON(elemRv, elemRt, w, n, err)
				if i < length-1 {
					WriteTo([]byte(","), w, n, err)
				}
			}
			WriteTo([]byte("]"), w, n, err)
		}

	case reflect.Struct:
		if rt == timeType {
			// Special case: time.Time
			t := rv.Interface().(time.Time)
			str := t.Format(rfc2822)
			jsonBytes, err_ := json.Marshal(str)
			if err_ != nil {
				*err = err_
				return
			}
			WriteTo(jsonBytes, w, n, err)
		} else {
			WriteTo([]byte("{"), w, n, err)
			wroteField := false
			for _, fieldInfo := range typeInfo.Fields {
				i, fieldType, opts := fieldInfo.unpack()
				fieldRv := rv.Field(i)
				if wroteField {
					WriteTo([]byte(","), w, n, err)
				} else {
					wroteField = true
				}
				WriteTo([]byte(Fmt("\"%v\":", opts.JSONName)), w, n, err)
				writeReflectJSON(fieldRv, fieldType, w, n, err)
			}
			WriteTo([]byte("}"), w, n, err)
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

	default:
		PanicSanity(Fmt("Unknown field type %v", rt.Kind()))
	}

}
