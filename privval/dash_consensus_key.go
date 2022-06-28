package privval

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	"github.com/dashevo/dashd-go/btcjson"

	tmcrypto "github.com/tendermint/tendermint/crypto"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

type dashConsensusPrivateKey struct {
	quorumHash tmcrypto.QuorumHash
	privval    DashPrivValidator
	quorumType btcjson.LLMQType
}

var _ tmcrypto.PrivKey = &dashConsensusPrivateKey{}

func (key dashConsensusPrivateKey) Bytes() []byte {
	quorumType := make([]byte, 8)
	binary.LittleEndian.PutUint64(quorumType, uint64(key.quorumType))
	ourType := key.TypeTag()

	bytes := make([]byte, len(key.quorumHash)+len(ourType)+8)
	bytes = append(bytes, quorumType...)
	bytes = append(bytes, []byte(ourType)...)
	bytes = append(bytes, key.quorumHash...)

	return bytes
}

// Sign implements tmcrypto.PrivKey
func (key dashConsensusPrivateKey) Sign(messageBytes []byte) ([]byte, error) {
	messageHash := tmcrypto.Checksum(messageBytes)

	return key.SignDigest(messageHash)
}

// SignDigest implements tmcrypto.PrivKey
func (key dashConsensusPrivateKey) SignDigest(messageHash []byte) ([]byte, error) {
	requestIDhash := messageHash
	decodedSignature, _, err := key.privval.QuorumSign(context.TODO(), messageHash, requestIDhash, key.quorumType, key.quorumHash)
	return decodedSignature, err
}

// PubKey implements tmcrypto.PrivKey
func (key dashConsensusPrivateKey) PubKey() tmcrypto.PubKey {
	pubkey, err := key.privval.GetPubKey(context.TODO(), key.quorumHash)
	if err != nil {
		panic("cannot retrieve public key: " + err.Error()) // not nice, but this iface doesn;t support error handling
	}

	// return NewDashConsensusPublicKey(pubkey, key.quorumHash, key.quorumType)
	return pubkey
}

// Equals implements tmcrypto.PrivKey
func (key dashConsensusPrivateKey) Equals(other tmcrypto.PrivKey) bool {
	return bytes.Equal(key.Bytes(), other.Bytes())
}

// Type implements tmcrypto.PrivKey
func (key dashConsensusPrivateKey) Type() string {
	return "dashCoreRPCPrivateKey"
}

// TypeTag implements jsontypes.Tagged interface.
func (key dashConsensusPrivateKey) TypeTag() string { return key.Type() }

func (key dashConsensusPrivateKey) String() string {
	return fmt.Sprintf("%s(quorumHash:%s,quorumType:%d)", key.Type(), key.quorumHash.ShortString(), key.quorumType)
}

// DashConesensusPublicKey is a public key that constructs SignID in the background, to avoid this additional step
// when verifying signatures.
type DashConsensusPublicKey struct {
	tmcrypto.PubKey

	quorumHash tmcrypto.QuorumHash
	quorumType btcjson.LLMQType
}

var _ tmcrypto.PubKey = DashConsensusPublicKey{}

// NewDashConsensusPublicKey wraps a public key with transparent handling of SignID according to DIP-7
func NewDashConsensusPublicKey(baseKey tmcrypto.PubKey, quorumHash tmcrypto.QuorumHash, quorumType btcjson.LLMQType) *DashConsensusPublicKey {
	if key, ok := baseKey.(*DashConsensusPublicKey); ok {
		// don't wrap ourselves, but allow change of quorum hash and type
		baseKey = key.PubKey
	}

	return &DashConsensusPublicKey{
		PubKey:     baseKey,
		quorumHash: quorumHash,
		quorumType: quorumType,
	}
}

func (pub DashConsensusPublicKey) VerifySignature(msg []byte, sig []byte) bool {
	hash := tmcrypto.Checksum(msg)
	return pub.VerifySignatureDigest(hash, sig)
}
func (pub DashConsensusPublicKey) VerifySignatureDigest(hash []byte, sig []byte) bool {
	signID := tmcrypto.SignID(
		pub.quorumType,
		tmbytes.Reverse(pub.quorumHash),
		tmbytes.Reverse(hash[:]),
		tmbytes.Reverse(hash[:]),
	)

	return pub.PubKey.VerifySignatureDigest(signID, sig)
}
