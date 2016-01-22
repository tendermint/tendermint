package wire

import (
	"io"
	"math"
)

// Float32

func WriteFloat32(f float32, w io.Writer, n *int, err *error) {
	WriteUint32(math.Float32bits(f), w, n, err)
}

func ReadFloat32(r io.Reader, n *int, err *error) float32 {
	x := ReadUint32(r, n, err)
	return math.Float32frombits(x)
}

// Float64

func WriteFloat64(f float64, w io.Writer, n *int, err *error) {
	WriteUint64(math.Float64bits(f), w, n, err)
}

func ReadFloat64(r io.Reader, n *int, err *error) float64 {
	x := ReadUint64(r, n, err)
	return math.Float64frombits(x)
}
