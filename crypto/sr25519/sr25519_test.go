package sr25519_test

import (
	"encoding/hex"
	"fmt"
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
	assert.True(t, pubKey.VerifyBytes(msg, sig))
	assert.True(t, pubKey.VerifyBytes(msg, sig))

	// Mutate the signature, just one bit.
	// TODO: Replace this with a much better fuzzer, tendermint/ed25519/issues/10
	sig[7] ^= byte(0x01)

	assert.False(t, pubKey.VerifyBytes(msg, sig))
}

func TestPrivKeySr25519MarshalBinary(t *testing.T) {
	testCases := []struct {
		in  string
		out string // amino compatible
	}{
		{
			"AB7E8ADFE3CC73F01E20AC521DB28F2A1383A563F24C2F28D51133A407E8000C",
			"2F82D78B20AB7E8ADFE3CC73F01E20AC521DB28F2A1383A563F24C2F28D51133A407E8000C",
		},
		{
			"0000000000000000000000000000000000000000000000000000000000000000",
			"2F82D78B200000000000000000000000000000000000000000000000000000000000000000",
		},
	}

	for _, tc := range testCases {
		privkeyBz, err := hex.DecodeString(tc.in)
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("%X", len(privkeyBz)), "20")

		var privkey sr25519.PrivKeySr25519
		copy(privkey[:], privkeyBz)

		bz, err := privkey.MarshalBinary()
		require.NoError(t, err)
		require.Equal(t, tc.out, fmt.Sprintf("%X", bz))
		require.Equal(t, tc.out, fmt.Sprintf("%X", privkey.Bytes()))
	}
}

func TestPrivKeySr25519UnmarshalBinary(t *testing.T) {
	testCases := []struct {
		in        string // amino compatible encoding
		out       string
		expectErr bool
	}{
		{
			"2F82D78B20AB7E8ADFE3CC73F01E20AC521DB28F2A1383A563F24C2F28D51133A407E8000C",
			"AB7E8ADFE3CC73F01E20AC521DB28F2A1383A563F24C2F28D51133A407E8000C",
			false,
		},
		{
			"2F82D78B200000000000000000000000000000000000000000000000000000000000000000",
			"0000000000000000000000000000000000000000000000000000000000000000",
			false,
		},
		{
			"0F82D78B20AB7E8ADFE3CC73F01E20AC521DB28F2A1383A563F24C2F28D51133A407E8000C",
			"0000000000000000000000000000000000000000000000000000000000000000",
			true,
		},
		{
			"2F82D78B21AB7E8ADFE3CC73F01E20AC521DB28F2A1383A563F24C2F28D51133A407E8000C",
			"0000000000000000000000000000000000000000000000000000000000000000",
			true,
		},
		{
			"2F82D78B20AB7E8ADFE3CC73F01E20AC521DB28F2A1383A563F24C2F28D51133A407E8000C00",
			"0000000000000000000000000000000000000000000000000000000000000000",
			true,
		},
	}

	for _, tc := range testCases {
		bz, err := hex.DecodeString(tc.in)
		require.NoError(t, err)

		var privkey sr25519.PrivKeySr25519

		require.Equal(t, tc.expectErr, privkey.UnmarshalBinary(bz) != nil)
		require.Equal(t, tc.out, fmt.Sprintf("%X", privkey[:]))
	}
}

func TestPubKeySr25519MarshalBinary(t *testing.T) {
	testCases := []struct {
		in  string
		out string // amino compatible
	}{
		{
			"0000000000000000000000000000000000000000000000000000000000000000",
			"0DFB1005200000000000000000000000000000000000000000000000000000000000000000",
		},
		{
			"342BDD4C05DBB522A4DE85A74F9B951AA8912E4DABF2DAAB88617AECC09DF322",
			"0DFB100520342BDD4C05DBB522A4DE85A74F9B951AA8912E4DABF2DAAB88617AECC09DF322",
		},
	}

	for _, tc := range testCases {
		pubkeyBz, err := hex.DecodeString(tc.in)
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("%X", len(pubkeyBz)), "20")

		var pubkey sr25519.PubKeySr25519
		copy(pubkey[:], pubkeyBz)

		bz, err := pubkey.MarshalBinary()
		require.NoError(t, err)
		require.Equal(t, tc.out, fmt.Sprintf("%X", bz))
		require.Equal(t, tc.out, fmt.Sprintf("%X", pubkey.Bytes()))
	}
}

func TestPubKeySr25519UnmarshalBinary(t *testing.T) {
	testCases := []struct {
		in        string // amino compatible encoding
		out       string
		expectErr bool
	}{
		{
			"0DFB100520342BDD4C05DBB522A4DE85A74F9B951AA8912E4DABF2DAAB88617AECC09DF322",
			"342BDD4C05DBB522A4DE85A74F9B951AA8912E4DABF2DAAB88617AECC09DF322",
			false,
		},
		{
			"0DFB1005200000000000000000000000000000000000000000000000000000000000000000",
			"0000000000000000000000000000000000000000000000000000000000000000",
			false,
		},
		{
			"FDFB100520342BDD4C05DBB522A4DE85A74F9B951AA8912E4DABF2DAAB88617AECC09DF322",
			"0000000000000000000000000000000000000000000000000000000000000000",
			true,
		},
		{
			"0DFB100521342BDD4C05DBB522A4DE85A74F9B951AA8912E4DABF2DAAB88617AECC09DF322",
			"0000000000000000000000000000000000000000000000000000000000000000",
			true,
		},
		{
			"0DFB100520342BDD4C05DBB522A4DE85A74F9B951AA8912E4DABF2DAAB88617AECC09DF32200",
			"0000000000000000000000000000000000000000000000000000000000000000",
			true,
		},
	}

	for _, tc := range testCases {
		bz, err := hex.DecodeString(tc.in)
		require.NoError(t, err)

		var pubkey sr25519.PubKeySr25519

		require.Equal(t, tc.expectErr, pubkey.UnmarshalBinary(bz) != nil)
		require.Equal(t, tc.out, fmt.Sprintf("%X", pubkey[:]))
	}
}
