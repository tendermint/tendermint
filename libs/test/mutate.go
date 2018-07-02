package test

import (
	cmn "github.com/tendermint/tendermint/libs/common"
)

// Contract: !bytes.Equal(input, output) && len(input) >= len(output)
func MutateByteSlice(bytez []byte) []byte {
	// If bytez is empty, panic
	if len(bytez) == 0 {
		panic("Cannot mutate an empty bytez")
	}

	// Copy bytez
	mBytez := make([]byte, len(bytez))
	copy(mBytez, bytez)
	bytez = mBytez

	// Try a random mutation
	switch cmn.RandInt() % 2 {
	case 0: // Mutate a single byte
		bytez[cmn.RandInt()%len(bytez)] += byte(cmn.RandInt()%255 + 1)
	case 1: // Remove an arbitrary byte
		pos := cmn.RandInt() % len(bytez)
		bytez = append(bytez[:pos], bytez[pos+1:]...)
	}
	return bytez
}
