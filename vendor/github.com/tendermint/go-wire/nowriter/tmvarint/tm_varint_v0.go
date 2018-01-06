package tmvarint

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"github.com/tendermint/go-wire/nowriter/tmlegacy"
)

type TMVarintV0 struct {
}

var _ TMVarint = (*TMVarintV0)(nil)
var legacy = tmlegacy.TMEncoderLegacy{}

func (e TMVarintV0) EncodeUvarint(i uint) []byte {
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	var inst_n int
	n := &inst_n
	var inst_err error
	err := &inst_err

	var size = uvarintSize(uint64(i))
	legacy.WriteUint8(uint8(size), w, n, err)
	if size > 0 {
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(i))
		legacy.WriteTo(buf[(8-size):], w, n, err)
	}

	return b.Bytes()
}

func (e TMVarintV0) EncodeVarint(i int) []byte {
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	var inst_n int
	n := &inst_n
	var inst_err error
	err := &inst_err

	var negate = false
	if i < 0 {
		negate = true
		i = -i
	}
	var size = uvarintSize(uint64(i))
	if negate {
		// e.g. 0xF1 for a single negative byte
		legacy.WriteUint8(uint8(size+0xF0), w, n, err)
	} else {
		legacy.WriteUint8(uint8(size), w, n, err)
	}
	if size > 0 {
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(i))
		legacy.WriteTo(buf[(8-size):], w, n, err)
	}

	return b.Bytes()
}

func uvarintSize(i uint64) int {
	if i == 0 {
		return 0
	}
	if i < 1<<8 {
		return 1
	}
	if i < 1<<16 {
		return 2
	}
	if i < 1<<24 {
		return 3
	}
	if i < 1<<32 {
		return 4
	}
	if i < 1<<40 {
		return 5
	}
	if i < 1<<48 {
		return 6
	}
	if i < 1<<56 {
		return 7
	}
	return 8
}
