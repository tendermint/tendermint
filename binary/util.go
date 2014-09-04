package binary

import (
	"bytes"
	"crypto/sha256"
)

func BinaryBytes(b Binary) []byte {
	buf := bytes.NewBuffer(nil)
	b.WriteTo(buf)
	return buf.Bytes()
}

// NOTE: does not care about the type, only the binary representation.
func BinaryEqual(a, b Binary) bool {
	aBytes := BinaryBytes(a)
	bBytes := BinaryBytes(b)
	return bytes.Equal(aBytes, bBytes)
}

// NOTE: does not care about the type, only the binary representation.
func BinaryCompare(a, b Binary) int {
	aBytes := BinaryBytes(a)
	bBytes := BinaryBytes(b)
	return bytes.Compare(aBytes, bBytes)
}

func BinaryHash(b Binary) []byte {
	hasher := sha256.New()
	_, err := b.WriteTo(hasher)
	if err != nil {
		panic(err)
	}
	return hasher.Sum(nil)
}
