package encoding_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	pc "github.com/tendermint/tendermint/proto/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/crypto/encoding"
)

// mockUnsupportedPubKey is a mock type to test unsupported key handling
type mockUnsupportedPubKey struct{}

func (mockUnsupportedPubKey) Bytes() []byte {
	return []byte{}
}

func (mockUnsupportedPubKey) Address() bytes.HexBytes {
	return bytes.HexBytes{}
}

func (mockUnsupportedPubKey) Equals(crypto.PubKey) bool {
	return false
}

func (mockUnsupportedPubKey) Type() string {
	return "mockUnsupportedPubKey"
}

func (mockUnsupportedPubKey) VerifySignature(msg []byte, sig []byte) bool {
	return false
}

func TestPubKeyToProto(t *testing.T) {
	// Test Ed25519 PubKey
	edKey := ed25519.PubKey([]byte("test-ed25519-pubkey"))
	edProto, err := encoding.PubKeyToProto(edKey)
	require.NoError(t, err)
	require.IsType(t, &pc.PublicKey_Ed25519{}, edProto.Sum)
	require.Equal(t, edKey, ed25519.PubKey(edProto.GetEd25519())) // 确保类型一致

	// Test Secp256k1 PubKey
	secpKey := secp256k1.PubKey([]byte("test-secp256k1-pubkey"))
	secpProto, err := encoding.PubKeyToProto(secpKey)
	require.NoError(t, err)
	require.IsType(t, &pc.PublicKey_Secp256K1{}, secpProto.Sum)
	require.Equal(t, secpKey, secp256k1.PubKey(secpProto.GetSecp256K1())) // 确保类型一致

	// Test unsupported key type
	unsupportedKey := mockUnsupportedPubKey{}
	_, err = encoding.PubKeyToProto(unsupportedKey)
	require.Error(t, err)
}


func TestPubKeyFromProto(t *testing.T) {
	// Test Ed25519 PubKey
	edKey := ed25519.PubKey(make([]byte, ed25519.PubKeySize)) // 32 字节
	copy(edKey, []byte("12345678901234567890123456789012"))
	edProto := pc.PublicKey{
		Sum: &pc.PublicKey_Ed25519{
			Ed25519: edKey,
		},
	}
	decodedEdKey, err := encoding.PubKeyFromProto(edProto)
	require.NoError(t, err)
	require.Equal(t, edKey, decodedEdKey)

	// Test Secp256k1 PubKey
	secpKey := secp256k1.PubKey(make([]byte, secp256k1.PubKeySize)) // 33 字节
	copy(secpKey, []byte("1234567890123456789012345678901234567890"))
	secpProto := pc.PublicKey{
		Sum: &pc.PublicKey_Secp256K1{
			Secp256K1: secpKey,
		},
	}
	decodedSecpKey, err := encoding.PubKeyFromProto(secpProto)
	require.NoError(t, err)
	require.Equal(t, secpKey, decodedSecpKey)

	// Test invalid Ed25519 PubKey size
	invalidEdProto := pc.PublicKey{
		Sum: &pc.PublicKey_Ed25519{Ed25519: []byte("short-key")},
	}
	_, err = encoding.PubKeyFromProto(invalidEdProto)
	require.Error(t, err)

	// Test invalid Secp256k1 PubKey size
	invalidSecpProto := pc.PublicKey{
		Sum: &pc.PublicKey_Secp256K1{Secp256K1: []byte("short-key")},
	}
	_, err = encoding.PubKeyFromProto(invalidSecpProto)
	require.Error(t, err)

	// Test unsupported key type
	invalidProto := pc.PublicKey{Sum: nil}
	_, err = encoding.PubKeyFromProto(invalidProto)
	require.Error(t, err)
}

