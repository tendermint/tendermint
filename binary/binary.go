package binary

import (
	"encoding/json"
	"errors"
	"io"
	"reflect"
)

// TODO document and maybe make it configurable.
const MaxBinaryReadSize = 21 * 1024 * 1024

var ErrBinaryReadSizeOverflow = errors.New("Error: binary read size overflow")
var ErrBinaryReadSizeUnderflow = errors.New("Error: binary read size underflow")

func ReadBinary(o interface{}, r io.Reader, n *int64, err *error) interface{} {
	rv, rt := reflect.ValueOf(o), reflect.TypeOf(o)
	if rv.Kind() == reflect.Ptr {
		readReflectBinary(rv, rt, Options{}, r, n, err)
		return o
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
		return o
	} else {
		panic("ReadBinaryPtr expects o to be a pointer")
	}
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

func ReadJSONObject(o interface{}, object interface{}, err *error) interface{} {
	rv, rt := reflect.ValueOf(o), reflect.TypeOf(o)
	if rv.Kind() == reflect.Ptr {
		readReflectJSON(rv.Elem(), rt.Elem(), object, err)
		return o
	} else {
		ptrRv := reflect.New(rt)
		readReflectJSON(ptrRv.Elem(), rt, object, err)
		return ptrRv.Elem().Interface()
	}
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
