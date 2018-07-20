package ed25519_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
)

func TestGeneratePrivKey(t *testing.T) {
	testPriv := ed25519.GenPrivKey()
	testGenerate := testPriv.Generate(1)
	signBytes := []byte("something to sign")
	pub := testGenerate.PubKey()
	sig, err := testGenerate.Sign(signBytes)
	assert.NoError(t, err)
	assert.True(t, pub.VerifyBytes(signBytes, sig))
}

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
	sigEd := sig.(ed25519.SignatureEd25519)
	sigEd[7] ^= byte(0x01)
	sig = sigEd

	assert.False(t, pubKey.VerifyBytes(msg, sig))
}
