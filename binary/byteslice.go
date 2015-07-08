package binary

import (
	. "github.com/tendermint/tendermint/common"
	"io"
)

const (
	ByteSliceChunk = 1024
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
	if MaxBinaryReadSize < *n+int64(length) {
		*err = ErrMaxBinaryReadSizeReached
		return nil
	}

	var buf, tmpBuf []byte
	// read one ByteSliceChunk at a time and append
	for i := 0; i*ByteSliceChunk < length; i++ {
		tmpBuf = make([]byte, MinInt(ByteSliceChunk, length-i*ByteSliceChunk))
		ReadFull(tmpBuf, r, n, err)
		if *err != nil {
			return nil
		}
		buf = append(buf, tmpBuf...)
	}
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
	if MaxBinaryReadSize < *n+int64(length) {
		*err = ErrMaxBinaryReadSizeReached
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
