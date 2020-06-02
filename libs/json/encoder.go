package json

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"time"
)

var (
	timeType            = reflect.TypeOf(time.Time{})
	jsonMarshalerType   = reflect.TypeOf(new(json.Marshaler)).Elem()
	jsonUnmarshalerType = reflect.TypeOf(new(json.Unmarshaler)).Elem()
	errorType           = reflect.TypeOf(new(error)).Elem()
)

// This is the main entrypoint for encoding all types in json form.  This
// function calls encodeReflectJSON*, and generally those functions should
// only call this one, for the disfix wrapper is only written here.
// NOTE: Unlike encodeReflectBinary, rv may be a pointer.
// CONTRACT: rv is valid.
func encodeReflectJSON(w io.Writer, rv reflect.Value) (err error) {
	if !rv.IsValid() {
		panic("should not happen")
	}

	// Dereference value if pointer.
	var isNilPtr bool
	rv, _, isNilPtr = derefPointers(rv)

	// Write null if necessary.
	if isNilPtr {
		err = writeStr(w, `null`)
		return
	}

	// Special case:
	if rv.Type() == timeType {
		// Amino time strips the timezone.
		// NOTE: This must be done before json.Marshaler override below.
		ct := rv.Interface().(time.Time).Round(0).UTC()
		rv = reflect.ValueOf(ct)
	}
	// Handle override if rv implements json.Marshaler.
	if rv.CanAddr() { // Try pointer first.
		if rv.Addr().Type().Implements(jsonMarshalerType) {
			err = invokeMarshalJSON(w, rv.Addr())
			return
		}
	} else if rv.Type().Implements(jsonMarshalerType) {
		err = invokeMarshalJSON(w, rv)
		return
	}

	// Handle override if rv implements json.Marshaler.
	// FIXME Handle this somehow
	/*if info.IsAminoMarshaler {
		// First, encode rv into repr instance.
		var (
			rrv   reflect.Value
			rinfo *TypeInfo
		)
		rrv, err = toReprObject(rv)
		if err != nil {
			return
		}
		rinfo, err = cdc.getTypeInfoWlock(info.AminoMarshalReprType)
		if err != nil {
			return
		}
		// Then, encode the repr instance.
		err = cdc.encodeReflectJSON(w, rinfo, rrv)
		return
	}*/

	switch rv.Type().Kind() {

	//----------------------------------------
	// Complex

	// FIXME Handle these
	/*case reflect.Interface:
		return cdc.encodeReflectJSONInterface(w, info, rv)

	case reflect.Array, reflect.Slice:
		return cdc.encodeReflectJSONList(w, info, rv)

	case reflect.Struct:
		return cdc.encodeReflectJSONStruct(w, info, rv)

	case reflect.Map:
		return cdc.encodeReflectJSONMap(w, info, rv)*/

	//----------------------------------------
	// Signed, Unsigned

	case reflect.Int64, reflect.Int:
		_, err = fmt.Fprintf(w, `"%d"`, rv.Int()) // JS can't handle int64
		return

	case reflect.Uint64, reflect.Uint:
		_, err = fmt.Fprintf(w, `"%d"`, rv.Uint()) // JS can't handle uint64
		return

	case reflect.Int32, reflect.Int16, reflect.Int8,
		reflect.Uint32, reflect.Uint16, reflect.Uint8:
		return invokeStdlibJSONMarshal(w, rv.Interface())

	//----------------------------------------
	// Misc

	case reflect.Float64, reflect.Float32:
		// FIXME Is Unsafe enabled by default? If not, we need to retain this.
		//if !fopts.Unsafe {
		return errors.New("amino.JSON float* support requires `amino:\"unsafe\"`")
		//}
		//fallthrough
	case reflect.Bool, reflect.String:
		return invokeStdlibJSONMarshal(w, rv.Interface())

	//----------------------------------------
	// Default

	default:
		panic(fmt.Sprintf("unsupported type %v", rv.Type().Kind()))
	}
}

// CONTRACT: rv implements json.Marshaler.
func invokeMarshalJSON(w io.Writer, rv reflect.Value) error {
	blob, err := rv.Interface().(json.Marshaler).MarshalJSON()
	if err != nil {
		return err
	}
	_, err = w.Write(blob)
	return err
}

func invokeStdlibJSONMarshal(w io.Writer, v interface{}) error {
	// Note: Please don't stream out the output because that adds a newline
	// using json.NewEncoder(w).Encode(data)
	// as per https://golang.org/pkg/encoding/json/#Encoder.Encode
	blob, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(blob)
	return err
}

// Dereference pointer recursively.
// drv: the final non-pointer value (which may be invalid).
// isPtr: whether rv.Kind() == reflect.Ptr.
// isNilPtr: whether a nil pointer at any level.
func derefPointers(rv reflect.Value) (drv reflect.Value, isPtr bool, isNilPtr bool) {
	for rv.Kind() == reflect.Ptr {
		isPtr = true
		if rv.IsNil() {
			isNilPtr = true
			return
		}
		rv = rv.Elem()
	}
	drv = rv
	return
}

func writeStr(w io.Writer, s string) (err error) {
	_, err = w.Write([]byte(s))
	return
}

func _fmt(s string, args ...interface{}) string {
	return fmt.Sprintf(s, args...)
}
