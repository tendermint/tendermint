package wire

import (
	"io"

	. "github.com/tendermint/tendermint/common"
)

func WriteByteSlice(bz []byte, w io.Writer, n *int64, err *error) {
	WriteVarint(len(bz), w, n, err)
	WriteTo(bz, w, n, err)
}

func ReadByteSlice(r io.Reader, n *int64, err *error) []byte {
	length := ReadVarint(r, n, err)
	if *err != nil {
		return nil
	}
	if length < 0 {
		*err = ErrBinaryReadSizeUnderflow
		return nil
	}
	if MaxBinaryReadSize < MaxInt64(int64(length), *n+int64(length)) {
		*err = ErrBinaryReadSizeOverflow
		return nil
	}

	buf := make([]byte, length)
	ReadFull(buf, r, n, err)
	return buf
}

//-----------------------------------------------------------------------------

func WriteByteSlices(bzz [][]byte, w io.Writer, n *int64, err *error) {
	WriteVarint(len(bzz), w, n, err)
	for _, bz := range bzz {
		WriteByteSlice(bz, w, n, err)
		if *err != nil {
			return
		}
	}
}

func ReadByteSlices(r io.Reader, n *int64, err *error) [][]byte {
	length := ReadVarint(r, n, err)
	if *err != nil {
		return nil
	}
	if length < 0 {
		*err = ErrBinaryReadSizeUnderflow
		return nil
	}
	if MaxBinaryReadSize < MaxInt64(int64(length), *n+int64(length)) {
		*err = ErrBinaryReadSizeOverflow
		return nil
	}

	bzz := make([][]byte, length)
	for i := 0; i < length; i++ {
		bz := ReadByteSlice(r, n, err)
		if *err != nil {
			return nil
		}
		bzz[i] = bz
	}
	return bzz
}
