package tmdecoding

import (
	"encoding/binary"
	"errors"
	"math"
	"time"
)

// Implementation of the TMDecoder interface.
type TMDecoderPure struct {
}

var _ TMDecoder = TMDecoderPure{}

const errorBytesRead int = -1

func (e TMDecoderPure) DecodeBool(inp []byte) (b bool, bytesRead int, err error) {
	const primitiveSize int = 1
	if len(inp) < primitiveSize {
		return
	}
	switch inp[0] {
	case 0:
		bytesRead = primitiveSize
	case 1:
		bytesRead = primitiveSize
		b = true
	default:
		err = errors.New("Invalid bool")
		bytesRead = errorBytesRead
	}
	return
}

func (e TMDecoderPure) DecodeFloat32(inp []byte) (f float32, bytesRead int, err error) {
	const primitiveSize int = 4
	if len(inp) < primitiveSize {
		return
	}
	i := uint32(binary.BigEndian.Uint32(inp[:primitiveSize]))
	f = math.Float32frombits(i)
	bytesRead = primitiveSize
	return
}

func (e TMDecoderPure) DecodeFloat64(inp []byte) (f float64, bytesRead int, err error) {
	const primitiveSize int = 8
	if len(inp) < primitiveSize {
		return
	}
	i := uint64(binary.BigEndian.Uint64(inp[:primitiveSize]))
	f = math.Float64frombits(i)
	bytesRead = primitiveSize
	return
}

func (e TMDecoderPure) DecodeInt8(inp []byte) (i int8, bytesRead int, err error) {
	const primitiveSize int = 1
	if len(inp) < primitiveSize {
		return
	}
	i = int8(inp[0])
	bytesRead = primitiveSize
	return
}

func (e TMDecoderPure) DecodeInt16(inp []byte) (i int16, bytesRead int, err error) {
	const primitiveSize int = 2
	if len(inp) < primitiveSize {
		return
	}
	i = int16(binary.BigEndian.Uint16(inp[:primitiveSize]))
	bytesRead = primitiveSize
	return
}

func (e TMDecoderPure) DecodeInt32(inp []byte) (i int32, bytesRead int, err error) {
	const primitiveSize int = 4
	if len(inp) < primitiveSize {
		return
	}
	i = int32(binary.BigEndian.Uint32(inp[:primitiveSize]))
	bytesRead = primitiveSize
	return
}

func (e TMDecoderPure) DecodeInt64(inp []byte) (i int64, bytesRead int, err error) {
	const primitiveSize int = 8
	if len(inp) < primitiveSize {
		return
	}
	i = int64(binary.BigEndian.Uint64(inp[:primitiveSize]))
	bytesRead = primitiveSize
	return
}
func (e TMDecoderPure) DecodeOctet(inp []byte) (b byte, bytesRead int, err error) {
	const primitiveSize int = 1
	if len(inp) < primitiveSize {
		return
	}
	b = inp[0]
	bytesRead = primitiveSize
	return
}

func (e TMDecoderPure) DecodeOctets(inp []byte) (b []byte, bytesRead int, err error) {
	bytesRead = len(inp)
	if bytesRead > 0 {
		b = make([]byte, bytesRead)
		copy(b, inp)
	}
	return
}

func (e TMDecoderPure) DecodeTime(inp []byte) (t time.Time, bytesRead int, err error) {
	const primitiveSize int = 8
	if len(inp) < primitiveSize {
		return
	}
	i := int64(binary.BigEndian.Uint64(inp[:primitiveSize]))
	t = time.Unix(0, i)
	bytesRead = primitiveSize
	return
}

func (e TMDecoderPure) DecodeUint8(inp []byte) (i uint8, bytesRead int, err error) {
	const primitiveSize int = 1
	if len(inp) < primitiveSize {
		return
	}
	i = uint8(inp[0])
	bytesRead = primitiveSize
	return
}
func (e TMDecoderPure) DecodeUint16(inp []byte) (i uint16, bytesRead int, err error) {
	const primitiveSize int = 2
	if len(inp) < primitiveSize {
		return
	}
	i = uint16(binary.BigEndian.Uint16(inp[:primitiveSize]))
	bytesRead = primitiveSize
	return
}

func (e TMDecoderPure) DecodeUint16s(inp []byte) (iz []uint16, bytesRead int, err error) {
	if len(inp) < 4 {
		return
	}
	count := int(uint32(binary.BigEndian.Uint32(inp[:4])))
	wantedBytes := int(4 + 2*count)
	if len(inp) < wantedBytes {
		return
	}
	iz = make([]uint16, count)
	bytesRead = wantedBytes
	for i := 0; i < count; i += 1 {
		iz[i] = uint16(binary.BigEndian.Uint16(inp[2*i : 2*i+1]))
	}
	return
}

func (e TMDecoderPure) DecodeUint32(inp []byte) (i uint32, bytesRead int, err error) {
	const primitiveSize int = 4
	if len(inp) < primitiveSize {
		return
	}
	i = uint32(binary.BigEndian.Uint32(inp[:primitiveSize]))
	bytesRead = primitiveSize
	return
}

func (e TMDecoderPure) DecodeUint64(inp []byte) (i uint64, bytesRead int, err error) {
	const primitiveSize int = 8
	if len(inp) < primitiveSize {
		return
	}
	i = uint64(binary.BigEndian.Uint64(inp[:primitiveSize]))
	bytesRead = primitiveSize
	return
}

func (e TMDecoderPure) DecodeUvarint(inp []byte) (i uint, bytesRead int, err error) {
	inpSize := len(inp)
	if inpSize < 1 {
		return
	}
	var size = uint8(inp[0])
	if size > 8 {
		err = errors.New("Uvarint overflow")
		bytesRead = errorBytesRead
		return
	}
	if size == 0 {
		bytesRead = 1
		// i = 0 // already 0
		return
	}
	if int(size+1) > inpSize {
		return
	}
	var buf [8]byte
	copy(buf[(8-size):], inp[1:])
	i = uint(binary.BigEndian.Uint64(buf[:]))
	return
}

func (e TMDecoderPure) DecodeVarint(inp []byte) (i int, bytesRead int, err error) {
	inpSize := len(inp)
	polarity := 1
	if inpSize < 1 {
		return
	}
	var size = uint8(inp[0])
	if size>>4 == 0x0F {
		size &= 0x0F
		polarity = -1
	}
	if size > 8 {
		err = errors.New("Varint overflow")
		bytesRead = errorBytesRead
		return
	}
	if size == 0 {
		if polarity == -1 {
			err = errors.New("Varint does not allow negative zero")
			bytesRead = errorBytesRead
		} else {
			bytesRead = 1
			// i = 0 // already 0
		}
		return
	}
	if int(size+1) > inpSize {
		return
	}
	var buf [8]byte
	copy(buf[(8-size):], inp[1:])
	i = polarity * int(uint(binary.BigEndian.Uint64(buf[:])))
	return
}
