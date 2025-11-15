package armor

import (
	"bytes"
	"fmt"
	"io"

	"golang.org/x/crypto/openpgp/armor" //nolint: staticcheck
)

// EncodeArmor encodes the given data as ASCII armor format.
// It wraps the data in an armor block with the specified blockType and optional headers.
// The function panics if encoding fails.
//
// Parameters:
//   - blockType: the type of the armor block (e.g., "MESSAGE", "PRIVATE KEY")
//   - headers: optional metadata headers (can be nil)
//   - data: the raw bytes to encode
//
// Returns the ASCII-armored string representation of the data.
func EncodeArmor(blockType string, headers map[string]string, data []byte) string {
	buf := new(bytes.Buffer)
	w, err := armor.Encode(buf, blockType, headers)
	if err != nil {
		panic(fmt.Errorf("could not encode ascii armor: %s", err))
	}
	_, err = w.Write(data)
	if err != nil {
		panic(fmt.Errorf("could not encode ascii armor: %s", err))
	}
	err = w.Close()
	if err != nil {
		panic(fmt.Errorf("could not encode ascii armor: %s", err))
	}
	return buf.String()
}

// DecodeArmor decodes an ASCII-armored string back into its components.
// It extracts the block type, headers, and raw data from the armor format.
//
// Parameters:
//   - armorStr: the ASCII-armored string to decode
//
// Returns:
//   - blockType: the type of the armor block
//   - headers: the metadata headers from the armor block
//   - data: the decoded raw bytes
//   - err: error if decoding fails
func DecodeArmor(armorStr string) (blockType string, headers map[string]string, data []byte, err error) {
	buf := bytes.NewBufferString(armorStr)
	block, err := armor.Decode(buf)
	if err != nil {
		return "", nil, nil, err
	}
	data, err = io.ReadAll(block.Body)
	if err != nil {
		return "", nil, nil, err
	}
	return block.Type, block.Header, data, nil
}
