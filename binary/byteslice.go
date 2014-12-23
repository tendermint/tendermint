package binary

import (
	"io"
)

func WriteByteSlice(bz []byte, w io.Writer, n *int64, err *error) {
	WriteUvarint(uint(len(bz)), w, n, err)
	WriteTo(bz, w, n, err)
}

func ReadByteSlice(r io.Reader, n *int64, err *error) []byte {
	length := ReadUvarint(r, n, err)
	if *err != nil {
		return nil
	}
	buf := make([]byte, int(length))
	ReadFull(buf, r, n, err)
	return buf
}

//-----------------------------------------------------------------------------

func WriteByteSlices(bzz [][]byte, w io.Writer, n *int64, err *error) {
	WriteUvarint(uint(len(bzz)), w, n, err)
	for _, bz := range bzz {
		WriteByteSlice(bz, w, n, err)
		if *err != nil {
			return
		}
	}
}

func ReadByteSlices(r io.Reader, n *int64, err *error) [][]byte {
	length := ReadUvarint(r, n, err)
	if *err != nil {
		return nil
	}
	bzz := make([][]byte, length)
	for i := uint(0); i < length; i++ {
		bz := ReadByteSlice(r, n, err)
		if *err != nil {
			return nil
		}
		bzz[i] = bz
	}
	return bzz
}
