package wire

import (
	"encoding/json"
	"errors"
	"io"
	"reflect"

	. "github.com/tendermint/tendermint/common"
)

// TODO document and maybe make it configurable.
const MaxBinaryReadSize = 21 * 1024 * 1024

var ErrBinaryReadSizeOverflow = errors.New("Error: binary read size overflow")
var ErrBinaryReadSizeUnderflow = errors.New("Error: binary read size underflow")

func ReadBinary(o interface{}, r io.Reader, n *int64, err *error) interface{} {
	rv, rt := reflect.ValueOf(o), reflect.TypeOf(o)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			// This allows ReadBinaryObject() to return a nil pointer,
			// if the value read is nil.
			rvPtr := reflect.New(rt)
			ReadBinaryPtr(rvPtr.Interface(), r, n, err)
			return rvPtr.Elem().Interface()
		} else {
			readReflectBinary(rv, rt, Options{}, r, n, err)
			return o
		}
	} else {
		ptrRv := reflect.New(rt)
		readReflectBinary(ptrRv.Elem(), rt, Options{}, r, n, err)
		return ptrRv.Elem().Interface()
	}
}

func ReadBinaryPtr(o interface{}, r io.Reader, n *int64, err *error) interface{} {
	rv, rt := reflect.ValueOf(o), reflect.TypeOf(o)
	if rv.Kind() == reflect.Ptr {
		readReflectBinary(rv.Elem(), rt.Elem(), Options{}, r, n, err)
	} else {
		PanicSanity("ReadBinaryPtr expects o to be a pointer")
	}
	return o
}

func WriteBinary(o interface{}, w io.Writer, n *int64, err *error) {
	rv := reflect.ValueOf(o)
	rt := reflect.TypeOf(o)
	writeReflectBinary(rv, rt, Options{}, w, n, err)
}

func ReadJSON(o interface{}, bytes []byte, err *error) interface{} {
	var object interface{}
	*err = json.Unmarshal(bytes, &object)
	if *err != nil {
		return o
	}

	return ReadJSONObject(o, object, err)
}

func ReadJSONPtr(o interface{}, bytes []byte, err *error) interface{} {
	var object interface{}
	*err = json.Unmarshal(bytes, &object)
	if *err != nil {
		return o
	}

	return ReadJSONObjectPtr(o, object, err)
}

// o is the ultimate destination, object is the result of json unmarshal
func ReadJSONObject(o interface{}, object interface{}, err *error) interface{} {
	rv, rt := reflect.ValueOf(o), reflect.TypeOf(o)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			// This allows ReadJSONObject() to return a nil pointer
			// if the value read is nil.
			rvPtr := reflect.New(rt)
			ReadJSONObjectPtr(rvPtr.Interface(), object, err)
			return rvPtr.Elem().Interface()
		} else {
			readReflectJSON(rv, rt, object, err)
			return o
		}
	} else {
		ptrRv := reflect.New(rt)
		readReflectJSON(ptrRv.Elem(), rt, object, err)
		return ptrRv.Elem().Interface()
	}
}

func ReadJSONObjectPtr(o interface{}, object interface{}, err *error) interface{} {
	rv, rt := reflect.ValueOf(o), reflect.TypeOf(o)
	if rv.Kind() == reflect.Ptr {
		readReflectJSON(rv.Elem(), rt.Elem(), object, err)
	} else {
		PanicSanity("ReadJSON(Object)Ptr expects o to be a pointer")
	}
	return o
}

func WriteJSON(o interface{}, w io.Writer, n *int64, err *error) {
	rv := reflect.ValueOf(o)
	rt := reflect.TypeOf(o)
	if rv.Kind() == reflect.Ptr {
		rv, rt = rv.Elem(), rt.Elem()
	}
	writeReflectJSON(rv, rt, w, n, err)
}

// Write all of bz to w
// Increment n and set err accordingly.
func WriteTo(bz []byte, w io.Writer, n *int64, err *error) {
	if *err != nil {
		return
	}
	n_, err_ := w.Write(bz)
	*n += int64(n_)
	*err = err_
}

// Read len(buf) from r
// Increment n and set err accordingly.
func ReadFull(buf []byte, r io.Reader, n *int64, err *error) {
	if *err != nil {
		return
	}
	n_, err_ := io.ReadFull(r, buf)
	*n += int64(n_)
	*err = err_
}
