package rpcserver

import (
	"encoding/base64"
	"encoding/hex"
	"reflect"
	"time"

	"github.com/pkg/errors"

	"github.com/tendermint/go-wire"
	"github.com/tendermint/go-wire/data"
	cmn "github.com/tendermint/tmlibs/common"
)

var (
	timeType = wire.GetTypeFromStructDeclaration(struct{ time.Time }{})
)

func readJSONObjectPtr(o interface{}, object interface{}, err *error) interface{} {
	rv, rt := reflect.ValueOf(o), reflect.TypeOf(o)
	if rv.Kind() == reflect.Ptr {
		readReflectJSON(rv.Elem(), rt.Elem(), wire.Options{}, object, err)
	} else {
		cmn.PanicSanity("ReadJSON(Object)Ptr expects o to be a pointer")
	}
	return o
}

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
func readReflectJSON(rv reflect.Value, rt reflect.Type, opts wire.Options, o interface{}, err *error) {

	// Get typeInfo
	typeInfo := wire.GetTypeInfo(rt)

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
		typeInfo = wire.GetTypeInfo(rt)
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

			// if its data.Bytes, use hex; else use base64
			dbty := reflect.TypeOf(data.Bytes{})
			var buf []byte
			var err_ error
			if rt == dbty {
				buf, err_ = hex.DecodeString(oString)
			} else {
				buf, err_ = base64.StdEncoding.DecodeString(oString)
			}
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
			// if its data.Bytes, use hex; else use base64
			dbty := reflect.TypeOf(data.Bytes{})
			var buf []byte
			var err_ error
			if rt == dbty {
				buf, err_ = hex.DecodeString(oString)
			} else {
				buf, err_ = base64.StdEncoding.DecodeString(oString)
			}
			if err_ != nil {
				*err = err_
				return
			}
			//log.Info("Read byteslice", "bytes", byteslice)
			rv.Set(reflect.ValueOf(buf))
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
				f := typeInfo.Fields[0]
				fieldIdx, fieldType, opts := f.Index, f.Type, f.Options
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
					f := fieldInfo
					fieldIdx, fieldType, opts := f.Index, f.Type, f.Options
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
