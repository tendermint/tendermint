package common

import (
	"bytes"
)

// Fingerprint returns the first 6 bytes of a byte slice.
// If the slice is less than 6 bytes, the fingerprint
// contains trailing zeroes.
func Fingerprint(slice []byte) []byte {
	fingerprint := make([]byte, 6)
	copy(fingerprint, slice)
	return fingerprint
}

func IsZeros(slice []byte) bool {
	for _, byt := range slice {
		if byt != byte(0) {
			return false
		}
	}
	return true
}

func RightPadBytes(slice []byte, l int) []byte {
	if l < len(slice) {
		return slice
	}
	padded := make([]byte, l)
	copy(padded[0:len(slice)], slice)
	return padded
}

func LeftPadBytes(slice []byte, l int) []byte {
	if l < len(slice) {
		return slice
	}
	padded := make([]byte, l)
	copy(padded[l-len(slice):], slice)
	return padded
}

func TrimmedString(b []byte) string {
	trimSet := string([]byte{0})
	return string(bytes.TrimLeft(b, trimSet))

}
