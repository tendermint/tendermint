package bytes

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// HexBytes is a wrapper around []byte that encodes data as hexadecimal strings
// for use in JSON.
type HexBytes []byte

// Marshal needed for protobuf compatibility
func (bz HexBytes) Marshal() ([]byte, error) {
	return bz, nil
}

// Unmarshal needed for protobuf compatibility
func (bz *HexBytes) Unmarshal(data []byte) error {
	*bz = data
	return nil
}

// MarshalText encodes a HexBytes value as hexadecimal digits.
// This method is used by json.Marshal.
func (bz HexBytes) MarshalText() ([]byte, error) {
	enc := hex.EncodeToString([]byte(bz))
	return []byte(strings.ToUpper(enc)), nil
}

// UnmarshalText handles decoding of HexBytes from JSON strings.
// This method is used by json.Unmarshal.
// It allows decoding of both hex and base64-encoded byte arrays.
func (bz *HexBytes) UnmarshalText(data []byte) error {
	input := string(data)
	if input == "" || input == "null" {
		return nil
	}
	dec, err := hex.DecodeString(input)
	if err != nil {
		dec, err = base64.StdEncoding.DecodeString(input)

		if err != nil {
			return err
		}
	}
	*bz = HexBytes(dec)
	return nil
}

// Bytes fulfills various interfaces in light-client, etc...
func (bz HexBytes) Bytes() []byte {
	return bz
}

func (bz HexBytes) ShortString() string {
	if len(bz) < 3 {
		return ""
	}
	return strings.ToUpper(hex.EncodeToString(bz[:3]))
}

func (bz HexBytes) String() string {
	return strings.ToUpper(hex.EncodeToString(bz))
}

// ReverseBytes returns a reversed sequence bytes of the current slice of byte
func (bz HexBytes) ReverseBytes() HexBytes {
	return Reverse(bz)
}

// Format writes either address of 0th element in a slice in base 16 notation,
// with leading 0x (%p), or casts HexBytes to bytes and writes as hexadecimal
// string to s.
func (bz HexBytes) Format(s fmt.State, verb rune) {
	switch verb {
	case 'p':
		s.Write([]byte(fmt.Sprintf("%p", bz)))
	default:
		s.Write([]byte(fmt.Sprintf("%X", []byte(bz))))
	}
}

// Copy creates a deep copy of HexBytes. It allocates new buffer and copies data into it.
func (bz HexBytes) Copy() HexBytes {
	if bz == nil {
		return nil
	}
	copied := make(HexBytes, len(bz))
	copy(copied, bz)
	return copied
}

func (bz HexBytes) Equal(b []byte) bool {
	return bytes.Equal(bz, b)
}

// Reverse returns a reversed sequence bytes of passed slice
func Reverse(bz []byte) []byte {
	l := len(bz)
	s := make([]byte, l)
	for i, j := 0, l-1; i <= j; i, j = i+1, j-1 {
		s[i], s[j] = bz[j], bz[i]
	}
	return s
}
