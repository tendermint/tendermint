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
	return encodeJSONReflect(w, reflect.ValueOf(v))
}

// This is the main entrypoint for encoding all types in json form.  This
// function calls encodeJSONReflect*, and generally those functions should
// only call this one, for the disfix wrapper is only written here.
// NOTE: Unlike encodeReflectBinary, rv may be a pointer.
// CONTRACT: rv is valid.
func encodeJSONReflect(w io.Writer, rv reflect.Value) error {
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

	// Convert times to UTC.
	if rv.Type() == timeType {
		rv = reflect.ValueOf(rv.Interface().(time.Time).Round(0).UTC())
	}

	switch rv.Type().Kind() {

	//----------------------------------------
	// Complex types

	// Complex types must be recursively encoded.
	// FIXME Handle these
	//case reflect.Interface:
	//return cdc.encodeJSONReflectInterface(w, info, rv)

	case reflect.Array, reflect.Slice:
		return encodeJSONReflectList(w, rv)

	//case reflect.Struct:
	//return cdc.encodeJSONReflectStruct(w, info, rv)

	case reflect.Map:
		return encodeJSONReflectMap(w, rv)

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

func encodeJSONReflectList(w io.Writer, rv reflect.Value) error {
	// Emit nil slices as null.
	if rv.Kind() == reflect.Slice && rv.IsNil() {
		return writeStr(w, `null`)
	}

	// Encode byte slices as base64 with the stdlib encoder.
	if rv.Type().Elem().Kind() == reflect.Uint8 {
		return encodeJSONStdlib(w, rv.Interface())
	}

	// Anything else we recursively encode ourselves.
	length := rv.Len()
	if err := writeStr(w, `[`); err != nil {
		return err
	}
	for i := 0; i < length; i++ {
		if err := encodeJSONReflect(w, rv.Index(i)); err != nil {
			return err
		}
		if i < length-1 {
			if err := writeStr(w, `,`); err != nil {
				return err
			}
		}
	}
	return writeStr(w, `]`)
}

func encodeJSONReflectMap(w io.Writer, rv reflect.Value) error {
	if rv.Type().Key().Kind() != reflect.String {
		return errors.New("map key must be string")
	}

	if err := writeStr(w, `{`); err != nil {
		return err
	}
	writeComma := false
	for _, keyrv := range rv.MapKeys() {
		if writeComma {
			if err := writeStr(w, `,`); err != nil {
				return err
			}
		}
		if err := encodeJSONStdlib(w, keyrv.Interface()); err != nil {
			return err
		}
		if err := writeStr(w, `:`); err != nil {
			return err
		}
		if err := encodeJSONReflect(w, rv.MapIndex(keyrv)); err != nil {
			return err
		}
		writeComma = true
	}
	return writeStr(w, `}`)
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
