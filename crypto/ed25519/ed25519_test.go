package ed25519_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	amino "github.com/tendermint/go-amino"
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

func TestEd25519MarshalBinaryLengthPrefixed(t *testing.T) {
	pubkeyStr := "F85678BD3C00EE053F6255B70A4AF2F645151C2884C6189F7646C199B282310A"
	pubkeyBz, err := hex.DecodeString(pubkeyStr)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("%x", len(pubkeyBz)), "20")

	var pubkey ed25519.PubKeyEd25519
	copy(pubkey[:], pubkeyBz)

	cdc := amino.NewCodec()
	ed25519.RegisterCodec(cdc)

	bz, err := cdc.MarshalBinaryLengthPrefixed(pubkey)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("%X", bz), "251624DE6420F85678BD3C00EE053F6255B70A4AF2F645151C2884C6189F7646C199B282310A")
	// => 2A0A041624DE6412220A20F85678BD3C00EE053F6255B70A4AF2F645151C2884C6189F7646C199B282310A
	// => 251624DE6420F85678BD3C00EE053F6255B70A4AF2F645151C2884C6189F7646C199B282310A
}
