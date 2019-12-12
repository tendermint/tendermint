package ed25519_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
)

func TestSignAndValidateEd25519(t *testing.T) {
	privKey := ed25519.GenPrivKey()
	pubKey := privKey.PubKey()

	msg := crypto.CRandBytes(128)
	sig, err := privKey.Sign(msg)
	require.Nil(t, err)

	// Test the signature
	assert.True(t, pubKey.VerifyBytes(msg, sig))

	// Mutate the signature, just one bit.
	// TODO: Replace this with a much better fuzzer, tendermint/ed25519/issues/10
	sig[7] ^= byte(0x01)

	assert.False(t, pubKey.VerifyBytes(msg, sig))
}

func TestEd25519MarshalBinaryBare(t *testing.T) {
	pubkeyStr := "F85678BD3C00EE053F6255B70A4AF2F645151C2884C6189F7646C199B282310A"
	pubkeyBz, err := hex.DecodeString(pubkeyStr)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("%x", len(pubkeyBz)), "20")

	var pubkey ed25519.PubKeyEd25519
	copy(pubkey[:], pubkeyBz)

	bz, err := pubkey.MarshalBinaryBare()
	require.NoError(t, err)
	require.Equal(t, "1624DE6420F85678BD3C00EE053F6255B70A4AF2F645151C2884C6189F7646C199B282310A", fmt.Sprintf("%X", bz))
}
