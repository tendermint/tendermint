package binary

import "io"

type Binary interface {
	WriteTo(w io.Writer) (int64, error)
}

func WriteTo(w io.Writer, bz []byte, n *int64, err *error) {
	if *err != nil {
		return
	}
	n_, err_ := w.Write(bz)
	*n += int64(n_)
	*err = err_
}

func ReadFull(r io.Reader, buf []byte, n *int64, err *error) {
	if *err != nil {
		return
	}
	n_, err_ := io.ReadFull(r, buf)
	*n += int64(n_)
	*err = err_
}
