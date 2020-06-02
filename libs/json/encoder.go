package json

import (
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strconv"
	"time"
)

var (
	timeType            = reflect.TypeOf(time.Time{})
	jsonMarshalerType   = reflect.TypeOf(new(json.Marshaler)).Elem()
	jsonUnmarshalerType = reflect.TypeOf(new(json.Unmarshaler)).Elem()
	errorType           = reflect.TypeOf(new(error)).Elem()
)

func encodeJSON(w io.Writer, v interface{}) error {
	if v == nil {
		_, err := w.Write([]byte("null"))
		return err
	}
	return encodeReflectJSON(w, reflect.ValueOf(v))
}

// This is the main entrypoint for encoding all types in json form.  This
// function calls encodeReflectJSON*, and generally those functions should
// only call this one, for the disfix wrapper is only written here.
// NOTE: Unlike encodeReflectBinary, rv may be a pointer.
// CONTRACT: rv is valid.
func encodeReflectJSON(w io.Writer, rv reflect.Value) (err error) {
	if !rv.IsValid() {
		return errors.New("invalid reflect value")
	}

	// Recursively dereference value if pointer.
	for rv.Kind() == reflect.Ptr {
		// If the value implements json.Marshaler, defer to stdlib directly. Dereferencing it will
		// break json.Marshaler implementations that take a pointer receiver.
		if rv.Type().Implements(jsonMarshalerType) {
			return encodeJSONStdlib(w, rv.Interface())
		}
		// If nil, we can't dereference by definition.
		if rv.IsNil() {
			return writeStr(w, `null`)
		}
		rv = rv.Elem()
	}

	// Pre-process times by converting to UTC.
	if rv.Type() == timeType {
		rv = reflect.ValueOf(rv.Interface().(time.Time).Round(0).UTC())
	}

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
	// Scalars

	// 64-bit integers are emitted as strings, to avoid precision problems with e.g.
	// Javascript which casts these to 64-bit floats (with 53-bit precision).
	case reflect.Int64, reflect.Int:
		return writeStr(w, `"`+strconv.FormatInt(rv.Int(), 10)+`"`)

	case reflect.Uint64, reflect.Uint:
		return writeStr(w, `"`+strconv.FormatUint(rv.Uint(), 10)+`"`)

	// For everything else, defer to the stdlib encoding/json encoder
	default:
		return encodeJSONStdlib(w, rv.Interface())
	}
}

func encodeJSONStdlib(w io.Writer, v interface{}) error {
	// Doesn't stream the output because that adds a newline, as per:
	// https://golang.org/pkg/encoding/json/#Encoder.Encode
	blob, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(blob)
	return err
}

func writeStr(w io.Writer, s string) error {
	_, err := w.Write([]byte(s))
	return err
}
