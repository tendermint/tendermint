package account

import (
	"bytes"
	"io"
)

type Signable interface {
	WriteSignBytes(w io.Writer, n *int64, err *error)
}

func SignBytes(o Signable) []byte {
	buf, n, err := new(bytes.Buffer), new(int64), new(error)
	o.WriteSignBytes(buf, n, err)
	if *err != nil {
		panic(err)
	}
	return buf.Bytes()
}
