package bytes

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// HexBytes enables HEX-encoding for json/encoding.
type HexBytes []byte

var (
	_ json.Marshaler   = HexBytes{}
	_ json.Unmarshaler = &HexBytes{}
)

// Marshal needed for protobuf compatibility
func (bz HexBytes) Marshal() ([]byte, error) {
	return bz, nil
}

// Unmarshal needed for protobuf compatibility
func (bz *HexBytes) Unmarshal(data []byte) error {
	*bz = data
	return nil
}

// MarshalJSON implements the json.Marshaler interface. The encoding is a JSON
// quoted string of hexadecimal digits.
func (bz HexBytes) MarshalJSON() ([]byte, error) {
	size := hex.EncodedLen(len(bz)) + 2 // +2 for quotation marks
	buf := make([]byte, size)
	hex.Encode(buf[1:], []byte(bz))
	buf[0] = '"'
	buf[size-1] = '"'

	// Ensure letter digits are capitalized.
	for i := 1; i < size-1; i++ {
		if buf[i] >= 'a' && buf[i] <= 'f' {
			buf[i] = 'A' + (buf[i] - 'a')
		}
	}
	return buf, nil
}

// UnmarshalJSON implements the json.Umarshaler interface.
func (bz *HexBytes) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		return nil
	}

	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("invalid hex string: %s", data)
	}

	bz2, err := hex.DecodeString(string(data[1 : len(data)-1]))
	if err != nil {
		return err
	}

	*bz = bz2

	return nil
}

// Bytes fulfills various interfaces in light-client, etc...
func (bz HexBytes) Bytes() []byte {
	return bz
}

func (bz HexBytes) String() string {
	return strings.ToUpper(hex.EncodeToString(bz))
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
