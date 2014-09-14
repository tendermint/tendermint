package binary

import (
	"io"
)

func WriteByteSlice(w io.Writer, bz []byte, n *int64, err *error) {
	WriteUInt32(w, uint32(len(bz)), n, err)
	WriteTo(w, bz, n, err)
}

func ReadByteSlice(r io.Reader, n *int64, err *error) []byte {
	length := ReadUInt32(r, n, err)
	if *err != nil {
		return nil
	}
	buf := make([]byte, int(length))
	ReadFull(r, buf, n, err)
	return buf
}

//-----------------------------------------------------------------------------

func WriteByteSlices(w io.Writer, bzz [][]byte, n *int64, err *error) {
	WriteUInt32(w, uint32(len(bzz)), n, err)
	for _, bz := range bzz {
		WriteByteSlice(w, bz, n, err)
		if *err != nil {
			return
		}
	}
}

func ReadByteSlices(r io.Reader, n *int64, err *error) [][]byte {
	length := ReadUInt32(r, n, err)
	if *err != nil {
		return nil
	}
	bzz := make([][]byte, length)
	for i := uint32(0); i < length; i++ {
		bz := ReadByteSlice(r, n, err)
		if *err != nil {
			return nil
		}
		bzz[i] = bz
	}
	return bzz
}
