package wire

import (
	"io"

	. "github.com/tendermint/go-common"
)

func WriteByteSlice(bz []byte, w io.Writer, n *int, err *error) {
	WriteVarint(len(bz), w, n, err)
	WriteTo(bz, w, n, err)
}

func ReadByteSlice(r io.Reader, lmt int, n *int, err *error) []byte {
	length := ReadVarint(r, n, err)
	if *err != nil {
		return nil
	}
	if length < 0 {
		*err = ErrBinaryReadInvalidLength
		return nil
	}
	if lmt != 0 && lmt < MaxInt(length, *n+length) {
		*err = ErrBinaryReadOverflow
		return nil
	}

	buf := make([]byte, length)
	ReadFull(buf, r, n, err)
	return buf
}

func PutByteSlice(buf []byte, bz []byte) (n int, err error) {
	n_, err := PutVarint(buf, len(bz))
	if err != nil {
		return 0, err
	}
	buf = buf[n_:]
	n += n_
	if len(buf) < len(bz) {
		return 0, ErrBinaryWriteOverflow
	}
	copy(buf, bz)
	return n + len(bz), nil
}

func GetByteSlice(buf []byte) (bz []byte, n int, err error) {
	length, n_, err := GetVarint(buf)
	if err != nil {
		return nil, 0, err
	}
	buf = buf[n_:]
	n += n_
	if length < 0 {
		return nil, 0, ErrBinaryReadInvalidLength
	}
	if len(buf) < length {
		return nil, 0, ErrBinaryReadOverflow
	}
	buf2 := make([]byte, length)
	copy(buf2, buf)
	return buf2, n + length, nil
}

// Returns the total encoded size of a byteslice
func ByteSliceSize(bz []byte) int {
	return UvarintSize(uint64(len(bz))) + len(bz)
}

//-----------------------------------------------------------------------------

func WriteByteSlices(bzz [][]byte, w io.Writer, n *int, err *error) {
	WriteVarint(len(bzz), w, n, err)
	for _, bz := range bzz {
		WriteByteSlice(bz, w, n, err)
		if *err != nil {
			return
		}
	}
}

func ReadByteSlices(r io.Reader, lmt int, n *int, err *error) [][]byte {
	length := ReadVarint(r, n, err)
	if *err != nil {
		return nil
	}
	if length < 0 {
		*err = ErrBinaryReadInvalidLength
		return nil
	}
	if lmt != 0 && lmt < MaxInt(length, *n+length) {
		*err = ErrBinaryReadOverflow
		return nil
	}

	bzz := make([][]byte, length)
	for i := 0; i < length; i++ {
		bz := ReadByteSlice(r, lmt, n, err)
		if *err != nil {
			return nil
		}
		bzz[i] = bz
	}
	return bzz
}
