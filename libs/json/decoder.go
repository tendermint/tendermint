package json

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

func decodeJSON(bz []byte, v interface{}) error {
	if len(bz) == 0 {
		return errors.New("cannot decode empty bytes")
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr {
		return errors.New("must decode into a pointer")
	}
	rv = rv.Elem()
	return decodeJSONReflect(bz, rv)
}

func decodeJSONReflect(bz []byte, rv reflect.Value) error {
	if !rv.CanAddr() {
		return errors.New("value is not addressable")
	}

	// Handle null for slices, interfaces, and pointers
	// FIXME Test this
	if bytes.Equal(bz, []byte("null")) {
		rv.Set(reflect.Zero(rv.Type()))
		return nil
	}

	// Dereference-and-construct pointers, to handle nested pointers.
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		rv = rv.Elem()
	}

	// Special case:
	/*if rv.Type() == timeType {
		// Amino time strips the timezone, so must end with Z.
		if len(bz) >= 2 && bz[0] == '"' && bz[len(bz)-1] == '"' {
			if bz[len(bz)-2] != 'Z' {
				return fmt.Errorf("amino:JSON time must be UTC and end with 'Z' but got %s", bz)
			}
		} else {
			return fmt.Errorf("amino:JSON time must be an RFC3339Nano string, but got %s", bz)
		}
	}*/

	// Handle override if a pointer to rv implements json.Unmarshaler.
	/*if rv.Addr().Type().Implements(jsonUnmarshalerType) {
		return rv.Addr().Interface().(json.Unmarshaler).UnmarshalJSON(bz)
	}*/

	// Handle override if a pointer to rv implements UnmarshalAmino.
	/*if info.IsAminoUnmarshaler {
		// First, decode repr instance from bytes.
		rrv := reflect.New(info.AminoUnmarshalReprType).Elem()
		var rinfo *TypeInfo
		rinfo, err = cdc.getTypeInfoWlock(info.AminoUnmarshalReprType)
		if err != nil {
			return
		}
		err = cdc.decodeReflectJSON(bz, rinfo, rrv, fopts)
		if err != nil {
			return
		}
		// Then, decode from repr instance.
		uwrm := rv.Addr().MethodByName("UnmarshalAmino")
		uwouts := uwrm.Call([]reflect.Value{rrv})
		erri := uwouts[0].Interface()
		if erri != nil {
			err = erri.(error)
		}
		return
	}*/

	switch rv.Type().Kind() {
	//case reflect.Interface:
	//	err = cdc.decodeReflectJSONInterface(bz, info, rv, fopts)

	//case reflect.Array:
	//	err = cdc.decodeReflectJSONArray(bz, info, rv, fopts)

	case reflect.Slice:
		return decodeJSONReflectSlice(bz, rv)

	//case reflect.Struct:
	//	err = cdc.decodeReflectJSONStruct(bz, info, rv, fopts)

	//case reflect.Map:
	//	err = cdc.decodeReflectJSONMap(bz, info, rv, fopts)*/

	//----------------------------------------
	// Signed, Unsigned

	// For 64-bit integers, unwrap expected string and defer to stdlib for integer decoding.
	case reflect.Int64, reflect.Int, reflect.Uint64, reflect.Uint:
		if bz[0] != '"' || bz[len(bz)-1] != '"' {
			return fmt.Errorf("invalid 64-bit integer encoding %q, expected string", string(bz))
		}
		bz = bz[1 : len(bz)-1]
		fallthrough

	// Anything else we defer to the stdlib.
	default:
		return decodeJSONStdlib(bz, rv)
	}
}

func decodeJSONReflectSlice(bz []byte, rv reflect.Value) error {
	if !rv.CanAddr() {
		return errors.New("value is not addressable")
	}

	switch rv.Type().Elem().Kind() {
	// Decode base64-encoded byte slices using stdlib decoder.
	case reflect.Uint8:
		if err := decodeJSONStdlib(bz, rv); err != nil {
			return err
		}

	// Decode anything else into a raw JSON slice, and decode values recursively.
	default:
		var rawSlice []json.RawMessage
		if err := json.Unmarshal(bz, &rawSlice); err != nil {
			return err
		}
		slice := reflect.MakeSlice(reflect.SliceOf(rv.Type().Elem()), len(rawSlice), len(rawSlice))
		for i, bz := range rawSlice {
			if err := decodeJSONReflect(bz, slice.Index(i)); err != nil {
				return err
			}
		}
		rv.Set(slice)
	}

	// Replace empty slices with nil slices, for Amino compatiblity
	if rv.Len() == 0 {
		rv.Set(reflect.Zero(rv.Type()))
	}
	return nil
}

func decodeJSONStdlib(bz []byte, rv reflect.Value) error {
	if !rv.CanAddr() && rv.Kind() != reflect.Ptr {
		return errors.New("value must be addressable or pointer")
	}

	// Make sure we are unmarshaling into a pointer.
	target := rv
	if rv.Kind() != reflect.Ptr {
		target = reflect.New(rv.Type())
	}
	if err := json.Unmarshal(bz, target.Interface()); err != nil {
		return err
	}
	rv.Set(target.Elem())
	return nil
}
