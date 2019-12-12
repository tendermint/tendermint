package crypto

import (
	"fmt"

	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/libs/bytes"
)

const (
	// AddressSize is the size of a pubkey address.
	AddressSize = tmhash.TruncatedSize
)

// An address is a []byte, but hex-encoded even in JSON.
// []byte leaves us the option to change the address length.
// Use an alias so Unmarshal methods (with ptr receivers) are available too.
type Address = bytes.HexBytes

func AddressHash(bz []byte) Address {
	return Address(tmhash.SumTruncated(bz))
}

type PubKey interface {
	Address() Address
	Bytes() []byte
	VerifyBytes(msg []byte, sig []byte) bool
	Equals(PubKey) bool
}

type PrivKey interface {
	Bytes() []byte
	Sign(msg []byte) ([]byte, error)
	PubKey() PubKey
	Equals(PrivKey) bool
}

type Symmetric interface {
	Keygen() []byte
	Encrypt(plaintext []byte, secret []byte) (ciphertext []byte)
	Decrypt(ciphertext []byte, secret []byte) (plaintext []byte, err error)
}

// KeyEncoding defines key encoding information where each key type has a unique
// name and prefix. A KeyEncoding object is meant to be used in a lookup table
// to get encoding information for a specific key type.
type KeyEncoding struct {
	Type   string
	Name   string
	Prefix []byte
	Length []byte
}

var keyTypeTable = map[string]KeyEncoding{}

// GetKeyEncoding looks up a KeyEncoding by key type. Not thread-safe.
func GetKeyEncoding(keyType string) (KeyEncoding, bool) {
	ke, ok := keyTypeTable[keyType]
	return ke, ok
}

// RegisterKeyEncoding attempts to register a KeyEncoding type. If the type is
// already registered, an error will be returned. Not thread-safe.
func RegisterKeyEncoding(ke KeyEncoding) error {
	if _, ok := GetKeyEncoding(ke.Name); ok {
		return fmt.Errorf("key encoding with name '%s' already registered", ke.Name)
	}

	keyTypeTable[ke.Type] = ke
	return nil
}
