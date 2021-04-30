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
	vFail := sr25519.NewBatchVerifier()
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

		switch i % 2 {
		case 0:
			err = vFail.Add(pub, msg, sig)
		case 1:
			msg[2] ^= byte(0x01)
			err = vFail.Add(pub, msg, sig)
		}
		require.NoError(t, err)
	}

	ok, valid := v.Verify()
	require.True(t, ok, "failed batch verification")
	for i, ok := range valid {
		require.Truef(t, ok, "sig[%d] should be marked valid", i)
	}

	ok, valid = vFail.Verify()
	require.False(t, ok, "succeeded batch verification (invalid batch)")
	for i, ok := range valid {
		expected := (i % 2) == 0
		require.Equalf(t, expected, ok, "sig[%d] should be %v", i, expected)
	}
}
