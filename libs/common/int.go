package common

import (
	"encoding/binary"
)

//--------------------------------------------------------------------------------

func PutUint64LE(dest []byte, i uint64) {
	binary.LittleEndian.PutUint64(dest, i)
}

func GetUint64LE(src []byte) uint64 {
	return binary.LittleEndian.Uint64(src)
}

func PutUint64BE(dest []byte, i uint64) {
	binary.BigEndian.PutUint64(dest, i)
}

func GetUint64BE(src []byte) uint64 {
	return binary.BigEndian.Uint64(src)
}

func PutInt64LE(dest []byte, i int64) {
	binary.LittleEndian.PutUint64(dest, uint64(i))
}

func GetInt64LE(src []byte) int64 {
	return int64(binary.LittleEndian.Uint64(src))
}

func PutInt64BE(dest []byte, i int64) {
	binary.BigEndian.PutUint64(dest, uint64(i))
}

func GetInt64BE(src []byte) int64 {
	return int64(binary.BigEndian.Uint64(src))
}

// IntInSlice returns true if a is found in the list.
func IntInSlice(a int, list []int) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
