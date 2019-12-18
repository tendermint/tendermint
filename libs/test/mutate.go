package test

import (
	tmrand "github.com/tendermint/tendermint/libs/rand"
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
	switch tmrand.Int() % 2 {
	case 0: // Mutate a single byte
		bytez[tmrand.Int()%len(bytez)] += byte(tmrand.Int()%255 + 1)
	case 1: // Remove an arbitrary byte
		pos := tmrand.Int() % len(bytez)
		bytez = append(bytez[:pos], bytez[pos+1:]...)
	}
	return bytez
}
