package binary

import (
	"encoding/json"
	"io"
	"reflect"
)

func ReadBinary(o interface{}, r io.Reader, n *int64, err *error) interface{} {
	rv, rt := reflect.ValueOf(o), reflect.TypeOf(o)
	if rv.Kind() == reflect.Ptr {
		readReflect(rv.Elem(), rt.Elem(), r, n, err)
		return o
	} else {
		ptrRv := reflect.New(rt)
		readReflect(ptrRv.Elem(), rt, r, n, err)
		return ptrRv.Elem().Interface()
	}
}

func WriteBinary(o interface{}, w io.Writer, n *int64, err *error) {
	rv := reflect.ValueOf(o)
	rt := reflect.TypeOf(o)
	writeReflect(rv, rt, w, n, err)
}

func ReadJSON(o interface{}, bytes []byte, err *error) interface{} {
	var parsed interface{}
	*err = json.Unmarshal(bytes, &parsed)
	if *err != nil {
		return o
	}

	rv, rt := reflect.ValueOf(o), reflect.TypeOf(o)
	if rv.Kind() == reflect.Ptr {
		readReflectJSON(rv.Elem(), rt.Elem(), parsed, err)
		return o
	} else {
		ptrRv := reflect.New(rt)
		readReflectJSON(ptrRv.Elem(), rt, parsed, err)
		return ptrRv.Elem().Interface()
	}
}

func WriteJSON(o interface{}, w io.Writer, n *int64, err *error) {
	rv := reflect.ValueOf(o)
	rt := reflect.TypeOf(o)
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
