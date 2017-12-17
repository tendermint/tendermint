package common

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// The main purpose of Bytes is to enable HEX-encoding for json/encoding.
type Bytes []byte

// Marshal needed for protobuf compatibility
func (b Bytes) Marshal() ([]byte, error) {
	return b, nil
}

// Unmarshal needed for protobuf compatibility
func (b *Bytes) Unmarshal(data []byte) error {
	*b = data
	return nil
}

// This is the point of Bytes.
func (b Bytes) MarshalJSON() ([]byte, error) {
	s := strings.ToUpper(hex.EncodeToString(b))
	jb := make([]byte, len(s)+2)
	jb[0] = '"'
	copy(jb[1:], []byte(s))
	jb[1] = '"'
	return jb, nil
}

// This is the point of Bytes.
func (b *Bytes) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("Invalid hex string: %s", data)
	}
	bytes, err := hex.DecodeString(string(data[1 : len(data)-1]))
	if err != nil {
		return err
	}
	*b = bytes
	return nil
}

// Allow it to fulfill various interfaces in light-client, etc...
func (b Bytes) Bytes() []byte {
	return b
}

func (b Bytes) String() string {
	return strings.ToUpper(hex.EncodeToString(b))
}
