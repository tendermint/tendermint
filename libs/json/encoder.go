package json

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type Car interface {
	Drive() error
}

type Tesla struct {
	Color string
}

func (t *Tesla) Drive() error { return nil }

var (
	timeType          = reflect.TypeOf(time.Time{})
	jsonMarshalerType = reflect.TypeOf(new(json.Marshaler)).Elem()
	//jsonUnmarshalerType = reflect.TypeOf(new(json.Unmarshaler)).Elem()
	//errorType           = reflect.TypeOf(new(error)).Elem()
	typeNames = map[reflect.Type]string{reflect.TypeOf(Tesla{}): "car/tesla"}
)

func lookupRegistered(rv reflect.Value) string {
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return ""
		}
		rv = rv.Elem()
	}
	return typeNames[rv.Type()]
}

func encodeJSON(w io.Writer, v interface{}) error {
	// Bare nil values can't be reflected, so we must handle them here.
	if v == nil {
		_, err := w.Write([]byte("null"))
		return err
	}
	rv := reflect.ValueOf(v)

	// If this is a registered type, defer to interface encoder. This is necessary since reflect
	// will return the type of the concrete type for interface variables, but not within structs.
	// Also, we must do this before calling encodeJSONReflect to avoid infinite loops.
	if lookupRegistered(rv) != "" {
		return encodeJSONReflectInterface(w, rv)
	}

	return encodeJSONReflect(w, rv)
}

func encodeJSONReflect(w io.Writer, rv reflect.Value) error {
	if !rv.IsValid() {
		return errors.New("invalid reflect value")
	}

	// Recursively dereference if pointer.
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return writeStr(w, `null`)
		}
		rv = rv.Elem()
	}

	// Convert times to UTC.
	if rv.Type() == timeType {
		rv = reflect.ValueOf(rv.Interface().(time.Time).Round(0).UTC())
	}

	// If the value implements json.Marshaler, defer to stdlib directly. Since we've already
	// dereferenced, we try implementations with both value receiver and pointer receiver. We must
	// do this after the time normalization above, and thus after dereferencing.
	if rv.Type().Implements(jsonMarshalerType) {
		return encodeJSONStdlib(w, rv.Interface())
	} else if rv.CanAddr() && rv.Addr().Type().Implements(jsonMarshalerType) {
		return encodeJSONStdlib(w, rv.Addr().Interface())
	}

	switch rv.Type().Kind() {
	// Complex types must be recursively encoded.
	case reflect.Interface:
		return encodeJSONReflectInterface(w, rv)

	case reflect.Array, reflect.Slice:
		return encodeJSONReflectList(w, rv)

	case reflect.Map:
		return encodeJSONReflectMap(w, rv)

	case reflect.Struct:
		return encodeJSONReflectStruct(w, rv)

	// 64-bit integers are emitted as strings, to avoid precision problems with e.g.
	// Javascript which uses 64-bit floats (having 53-bit precision).
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

func encodeJSONReflectStruct(w io.Writer, rv reflect.Value) error {
	if err := writeStr(w, `{`); err != nil {
		return err
	}
	length := rv.NumField()
	writeComma := false
	for i := 0; i < length; i++ {
		frv := rv.Field(i)
		// FIXME This needs to be cached
		ftype := rv.Type().Field(i)
		jsonOmitEmpty := false
		jsonName := ftype.Name
		if o := ftype.Tag.Get("json"); o != "" {
			opts := strings.Split(o, ",")
			if opts[0] != "" {
				jsonName = opts[0]
			}
			for _, o := range opts[1:] {
				if o == "omitempty" {
					jsonOmitEmpty = true
				}
			}
		}
		if jsonOmitEmpty && frv.IsZero() && jsonOmitEmpty {
			continue
		}

		if writeComma {
			if err := writeStr(w, `,`); err != nil {
				return err
			}
		}
		if err := encodeJSONStdlib(w, jsonName); err != nil {
			return err
		}
		if err := writeStr(w, `:`); err != nil {
			return err
		}
		if err := encodeJSONReflect(w, frv); err != nil {
			return err
		}
		writeComma = true
	}
	return writeStr(w, `}`)
}

func encodeJSONReflectInterface(w io.Writer, rv reflect.Value) error {
	if rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return writeStr(w, `null`)
		}
		rv = rv.Elem()
	}

	// Get concrete value and dereference pointers.
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return writeStr(w, `null`)
		}
		rv = rv.Elem()
	}

	// Look up the name of the concrete type
	name := lookupRegistered(rv)
	if name == "" {
		return fmt.Errorf("cannot encode unregistered concrete type %v", rv.Type())
	}

	// Write value wrapped in interface envelope
	if err := writeStr(w, fmt.Sprintf(`{"type":%q,"value":`, name)); err != nil {
		return err
	}
	if err := encodeJSONReflect(w, rv); err != nil {
		return err
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
