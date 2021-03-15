package sr25519_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/sr25519"
)

func TestSignAndValidateSr25519(t *testing.T) {

	privKey := sr25519.GenPrivKey()
	pubKey := privKey.PubKey()

	msg := crypto.CRandBytes(128)
	sig, err := privKey.Sign(msg)
	require.Nil(t, err)

	// Test the signature
	assert.True(t, pubKey.VerifySignature(msg, sig))
	assert.True(t, pubKey.VerifySignature(msg, sig))

	// Mutate the signature, just one bit.
	// TODO: Replace this with a much better fuzzer, tendermint/ed25519/issues/10
	sig[7] ^= byte(0x01)

	assert.False(t, pubKey.VerifySignature(msg, sig))
}

func TestBatchSafe(t *testing.T) {
	v := sr25519.NewBatchVerifier()
	for i := 0; i <= 38; i++ {
		priv := sr25519.GenPrivKey()
		pub := priv.PubKey()

		var msg []byte
		if i%2 == 0 {
			msg = []byte("easter")
		} else {
			msg = []byte("egg")
		}

		sig, err := priv.Sign(msg)
		require.NoError(t, err)

		err = v.Add(pub, msg, sig)
		require.NoError(t, err)
	}

	if !v.Verify() {
		t.Error("failed batch verification")
	}
}
