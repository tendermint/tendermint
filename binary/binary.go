package binary

import "io"

type Binary interface {
	WriteTo(w io.Writer) (int64, error)
}

func WriteBinary(w io.Writer, b Binary, n *int64, err *error) {
	if *err != nil {
		return
	}
	n_, err_ := b.WriteTo(w)
	*n += int64(n_)
	*err = err_
}

// Write all of bz to w
// Increment n and set err accordingly.
func WriteTo(w io.Writer, bz []byte, n *int64, err *error) {
	if *err != nil {
		return
	}
	n_, err_ := w.Write(bz)
	*n += int64(n_)
	*err = err_
}

// Read len(buf) from r
// Increment n and set err accordingly.
func ReadFull(r io.Reader, buf []byte, n *int64, err *error) {
	if *err != nil {
		return
	}
	n_, err_ := io.ReadFull(r, buf)
	*n += int64(n_)
	*err = err_
}
