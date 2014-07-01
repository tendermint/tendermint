package binary

import "io"

type Binary interface {
	WriteTo(w io.Writer) (int64, error)
}

func WriteOnto(b Binary, w io.Writer, n int64, err error) (int64, error) {
	if err != nil {
		return n, err
	}
	var n_ int64
	n_, err = b.WriteTo(w)
	return n + n_, err
}
