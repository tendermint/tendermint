package crypto

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dashevo/dashd-go/btcjson"

	"github.com/tendermint/tendermint/internal/jsontypes"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

const (
	// HashSize is the size in bytes of an AddressHash.
	HashSize = sha256.Size

	// AddressSize is the size of a pubkey address.
	AddressSize        = 20
	DefaultHashSize    = 32
	LargeAppHashSize   = DefaultHashSize
	SmallAppHashSize   = 20
	DefaultAppHashSize = LargeAppHashSize
	ProTxHashSize      = DefaultHashSize
	QuorumHashSize     = DefaultHashSize
)

type KeyType int

var (
	// ErrInvalidProTxHash uses in proTxHash validation
	ErrInvalidProTxHash = errors.New("proTxHash is invalid")
)

const (
	Ed25519 KeyType = iota
	BLS12381
	Secp256k1
	KeyTypeAny
)

// Address is an address is a []byte, but hex-encoded even in JSON.
// []byte leaves us the option to change the address length.
// Use an alias so Unmarshal methods (with ptr receivers) are available too.
type Address = tmbytes.HexBytes

type ProTxHash = tmbytes.HexBytes

type QuorumHash = tmbytes.HexBytes

// AddressHash computes a truncated SHA-256 hash of bz for use as
// a peer address.
//
// See: https://docs.tendermint.com/master/spec/core/data_structures.html#address
func AddressHash(bz []byte) Address {
	h := sha256.Sum256(bz)
	return Address(h[:AddressSize])
}

// Checksum returns the SHA256 of the bz.
func Checksum(bz []byte) []byte {
	h := sha256.Sum256(bz)
	return h[:]
}

func ProTxHashFromSeedBytes(bz []byte) ProTxHash {
	return Checksum(bz)
}

func RandProTxHash() ProTxHash {
	return CRandBytes(ProTxHashSize)
}

// RandProTxHashes generates and returns a list of N random generated proTxHashes
func RandProTxHashes(n int) []ProTxHash {
	proTxHashes := make([]ProTxHash, n)
	for i := 0; i < n; i++ {
		proTxHashes[i] = RandProTxHash()
	}
	return proTxHashes
}

// ProTxHashValidate validates the proTxHash value
func ProTxHashValidate(val ProTxHash) error {
	if len(val) != ProTxHashSize {
		return fmt.Errorf(
			"incorrect size actual %d, expected %d: %w",
			len(val),
			ProTxHashSize,
			ErrInvalidProTxHash,
		)
	}
	return nil
}

func RandQuorumHash() QuorumHash {
	return CRandBytes(ProTxHashSize)
}

func SmallQuorumType() btcjson.LLMQType {
	return btcjson.LLMQType_5_60
}

type SortProTxHash []ProTxHash

func (sptxh SortProTxHash) Len() int {
	return len(sptxh)
}

func (sptxh SortProTxHash) Less(i, j int) bool {
	return bytes.Compare(sptxh[i], sptxh[j]) == -1
}

func (sptxh SortProTxHash) Swap(i, j int) {
	sptxh[i], sptxh[j] = sptxh[j], sptxh[i]
}

type QuorumKeys struct {
	PrivKey            PrivKey
	PubKey             PubKey
	ThresholdPublicKey PubKey
}

type quorumKeysJSON struct {
	PrivKey            json.RawMessage `json:"priv_key"`
	PubKey             json.RawMessage `json:"pub_key"`
	ThresholdPublicKey json.RawMessage `json:"threshold_public_key"`
}

func (pvKey QuorumKeys) MarshalJSON() ([]byte, error) {
	var keys quorumKeysJSON
	var err error
	keys.PrivKey, err = jsontypes.Marshal(pvKey.PrivKey)
	if err != nil {
		return nil, err
	}
	keys.PubKey, err = jsontypes.Marshal(pvKey.PubKey)
	if err != nil {
		return nil, err
	}
	keys.ThresholdPublicKey, err = jsontypes.Marshal(pvKey.ThresholdPublicKey)
	if err != nil {
		return nil, err
	}
	return json.Marshal(keys)
}

func (pvKey *QuorumKeys) UnmarshalJSON(data []byte) error {
	var keys quorumKeysJSON
	err := json.Unmarshal(data, &keys)
	if err != nil {
		return err
	}
	err = jsontypes.Unmarshal(keys.PrivKey, &pvKey.PrivKey)
	if err != nil {
		return err
	}
	err = jsontypes.Unmarshal(keys.PubKey, &pvKey.PubKey)
	if err != nil {
		return err
	}
	return jsontypes.Unmarshal(keys.ThresholdPublicKey, &pvKey.ThresholdPublicKey)
}

// Validator is a validator interface
type Validator interface {
	Validate() error
}

type PubKey interface {
	Address() Address
	Bytes() []byte
	VerifySignature(msg []byte, sig []byte) bool
	VerifySignatureDigest(hash []byte, sig []byte) bool
	AggregateSignatures(sigSharesData [][]byte, messages [][]byte) ([]byte, error)
	VerifyAggregateSignature(msgs [][]byte, sig []byte) bool
	Equals(PubKey) bool
	Type() string

	// Implementations must support tagged encoding in JSON.
	jsontypes.Tagged
	fmt.Stringer
	HexStringer
}

type PrivKey interface {
	Bytes() []byte
	Sign(msg []byte) ([]byte, error)
	SignDigest(msg []byte) ([]byte, error)
	PubKey() PubKey
	Equals(PrivKey) bool
	Type() string

	// Implementations must support tagged encoding in JSON.
	jsontypes.Tagged
}

type Symmetric interface {
	Keygen() []byte
	Encrypt(plaintext []byte, secret []byte) (ciphertext []byte)
	Decrypt(ciphertext []byte, secret []byte) (plaintext []byte, err error)
}

// HexStringer ...
type HexStringer interface {
	HexString() string
}

// BatchVerifier If a new key type implements batch verification,
// the key type must be registered in github.com/tendermint/tendermint/crypto/batch
type BatchVerifier interface {
	// Add appends an entry into the BatchVerifier.
	Add(key PubKey, message, signature []byte) error
	// Verify verifies all the entries in the BatchVerifier, and returns
	// if every signature in the batch is valid, and a vector of bools
	// indicating the verification status of each signature (in the order
	// that signatures were added to the batch).
	Verify() (bool, []bool)
}
