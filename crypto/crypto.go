package crypto

import (
	bytes2 "bytes"
	"github.com/dashevo/dashd-go/btcjson"

	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/libs/bytes"
)

const (
	// AddressSize is the size of a pubkey address.
	AddressSize        = tmhash.TruncatedSize
	DefaultHashSize    = 32
	LargeAppHashSize   = DefaultHashSize
	SmallAppHashSize   = 20
	DefaultAppHashSize = LargeAppHashSize
	ProTxHashSize      = DefaultHashSize
	QuorumHashSize     = DefaultHashSize
)

type KeyType int

const (
	Ed25519 KeyType = iota
	BLS12381
	Secp256k1
	KeyTypeAny
)

// An address is a []byte, but hex-encoded even in JSON.
// []byte leaves us the option to change the address length.
// Use an alias so Unmarshal methods (with ptr receivers) are available too.
type Address = bytes.HexBytes

type ProTxHash = bytes.HexBytes

type QuorumHash = bytes.HexBytes

func AddressHash(bz []byte) Address {
	return Address(tmhash.SumTruncated(bz))
}

func ProTxHashFromSeedBytes(bz []byte) ProTxHash {
	return ProTxHash(tmhash.Sum(bz))
}

func RandProTxHash() ProTxHash {
	return ProTxHash(CRandBytes(ProTxHashSize))
}

func RandQuorumHash() QuorumHash {
	return QuorumHash(CRandBytes(ProTxHashSize))
}

func SmallQuorumType() btcjson.LLMQType {
	return btcjson.LLMQType_5_60
}

type SortProTxHash []ProTxHash

func (sptxh SortProTxHash) Len() int {
	return len(sptxh)
}

func (sptxh SortProTxHash) Less(i, j int) bool {
	return bytes2.Compare(sptxh[i], sptxh[j]) == -1
}

func (sptxh SortProTxHash) Swap(i, j int) {
	sptxh[i], sptxh[j] = sptxh[j], sptxh[i]
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
	TypeValue() KeyType
	String() string
}

type PrivKey interface {
	Bytes() []byte
	Sign(msg []byte) ([]byte, error)
	SignDigest(msg []byte) ([]byte, error)
	PubKey() PubKey
	Equals(PrivKey) bool
	Type() string
	TypeValue() KeyType
}

type Symmetric interface {
	Keygen() []byte
	Encrypt(plaintext []byte, secret []byte) (ciphertext []byte)
	Decrypt(ciphertext []byte, secret []byte) (plaintext []byte, err error)
}
