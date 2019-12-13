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

func TestEd25519MarshalBinary(t *testing.T) {
	testCases := []struct {
		in  string
		out string // amino compatible
	}{
		{
			"0000000000000000000000000000000000000000000000000000000000000000",
			"1624DE64200000000000000000000000000000000000000000000000000000000000000000",
		},
		{
			"F85678BD3C00EE053F6255B70A4AF2F645151C2884C6189F7646C199B282310A",
			"1624DE6420F85678BD3C00EE053F6255B70A4AF2F645151C2884C6189F7646C199B282310A",
		},
	}

	for _, tc := range testCases {
		pubkeyBz, err := hex.DecodeString(tc.in)
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("%X", len(pubkeyBz)), "20")

		var pubkey ed25519.PubKeyEd25519
		copy(pubkey[:], pubkeyBz)

		bz, err := pubkey.MarshalBinary()
		require.NoError(t, err)
		require.Equal(t, tc.out, fmt.Sprintf("%X", bz))
	}
}

func TestEd25519UnmarshalBinary(t *testing.T) {
	testCases := []struct {
		in        string // amino compatible encoding
		out       string
		expectErr bool
	}{
		{
			"1624DE6420F85678BD3C00EE053F6255B70A4AF2F645151C2884C6189F7646C199B282310A",
			"F85678BD3C00EE053F6255B70A4AF2F645151C2884C6189F7646C199B282310A",
			false,
		},
		{
			"1624DE64200000000000000000000000000000000000000000000000000000000000000000",
			"0000000000000000000000000000000000000000000000000000000000000000",
			false,
		},
		{
			"0DFB100520F85678BD3C00EE053F6255B70A4AF2F645151C2884C6189F7646C199B282310A",
			"0000000000000000000000000000000000000000000000000000000000000000",
			true,
		},
		{
			"1624DE6421F85678BD3C00EE053F6255B70A4AF2F645151C2884C6189F7646C199B282310A",
			"0000000000000000000000000000000000000000000000000000000000000000",
			true,
		},
		{
			"1624DE6420F85678BD3C00EE053F6255B70A4AF2F645151C2884C6189F7646C199B282310AAA",
			"0000000000000000000000000000000000000000000000000000000000000000",
			true,
		},
	}

	for _, tc := range testCases {
		bz, err := hex.DecodeString(tc.in)
		require.NoError(t, err)

		var pubkey ed25519.PubKeyEd25519

		require.Equal(t, tc.expectErr, pubkey.UnmarshalBinary(bz) != nil)
		require.Equal(t, tc.out, fmt.Sprintf("%X", pubkey[:]))
	}
}
