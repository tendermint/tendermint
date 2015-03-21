package vm

import (
	"encoding/binary"
)

func Uint64ToWord(i uint64) Word {
	word := Word{}
	PutUint64(word[:], i)
	return word
}

func BytesToWord(bz []byte) Word {
	word := Word{}
	copy(word[:], bz)
	return word
}

func LeftPadWord(bz []byte) (word Word) {
	copy(word[32-len(bz):], bz)
	return
}

func RightPadWord(bz []byte) (word Word) {
	copy(word[:], bz)
	return
}

func GetUint64(word Word) uint64 {
	return binary.LittleEndian.Uint64(word[:])
}

func PutUint64(dest []byte, i uint64) {
	binary.LittleEndian.PutUint64(dest, i)
}
