package bls12381_test

import (
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
)

func TestSignAndValidateBLS12381(t *testing.T) {

	privKey := bls12381.GenPrivKey()
	pubKey := privKey.PubKey()

	msg := crypto.CRandBytes(128)
	sig, err := privKey.Sign(msg)
	require.Nil(t, err)

	// Test the signature
	assert.True(t, pubKey.VerifySignature(msg, sig))
}

func TestBLSAddress(t *testing.T) {
	decodedPrivateKeyBytes, err := base64.StdEncoding.DecodeString("RokcLOxJWTyBkh5HPbdIACng/B65M8a5PYH1Nw6xn70=")
	require.Nil(t, err)
	publicKeyBytesString := "F5BjXeh0DppqaxX7a3LzoWr6CXPZcZeba6VHYdbiUCxQ23b00mFD8FRZpCz9Ug1E"
	decodedPublicKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyBytesString)
	require.Nil(t, err)
	decodedAddressBytes, err := hex.DecodeString("DDAD59BB10A10088C5A9CA219C3CF5BB4599B54E")
	require.Nil(t, err)
	privKey := bls12381.PrivKey(decodedPrivateKeyBytes)
	pubKey := privKey.PubKey()
	address := pubKey.Address()
	assert.EqualValues(t, decodedPublicKeyBytes, pubKey)
	assert.EqualValues(t, decodedAddressBytes, address)
}

func TestRecoverThresholdPublicKeyFromPublicKeys4(t *testing.T) {
	proTxHashStrings := make([]string, 4)
	proTxHashStrings[0] = "FDC09407DA9473CDC5E5AFCBB55712C95765343B2AF900B28BE4004E69CEDBB3" //e2e test validator1 protxhash
	proTxHashStrings[1] = "02AE8AAAF330949260BA537B05E408CEFA162FFA7CBB04C6C95BE2F922650A9D" //e2e test validator2 protxhash
	proTxHashStrings[2] = "036BCAEE159C09B75FD6404FAD6E76620AF595EF74734572E9D3DB4C226466FD" //e2e test validator3 protxhash
	proTxHashStrings[3] = "04FFEFD49498E2FC51A6FEB7FC695ED614EEA24B1C42F62AECDCA36B0FDCC026" //e2e test validator4 protxhash
	proTxHashes := make([][]byte, 4)
	for i, proTxHashString := range proTxHashStrings {
		decodedProTxHash, err := hex.DecodeString(proTxHashString)
		require.NoError(t, err)
		proTxHashes[i] = decodedProTxHash
	}
	privateKeyStrings := make([]string, 4)
	privateKeyStrings[0] = "BT0evsfM4r7Cc5lvbrVjBZuo1FYjMeIFg/6u7gb35M4=" //e2e test validator1 privateKey
	privateKeyStrings[1] = "A+Kn7ACPalXguwaBim+uLrPoYm9TnOTL8M9sdcu6xfs=" //e2e test validator2 privateKey
	privateKeyStrings[2] = "aFK6bDN2X3S/67ESBlrCg/kOHPyjRAtHLeor6aEk/mI=" //e2e test validator3 privateKey
	privateKeyStrings[3] = "Si++rgi0fAxhDAajuTXdPBoBWxzbvHPSvF8EeFm5b9A=" //e2e test validator4 privateKey
	privateKeys := make([]crypto.PrivKey, 4)
	for i, privateKeyString := range privateKeyStrings {
		decodedPrivateKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyString)
		require.NoError(t, err)
		privateKeys[i] = bls12381.PrivKey(decodedPrivateKeyBytes)
	}
	publicKeyStrings := make([]string, 4)
	publicKeyStrings[0] = "l/cGlqBfWFP3LggkKGBjh3PAOYi5vNRrTjjaey9mxUuMHHegpGMDayxKhkaWq1vr" //e2e test validator1 publicKey
	publicKeyStrings[1] = "AmOAMHT33gNypyJGEFm2EEj8c9xbAddlAsVgDWEKevVZ7OIZEUCICZPgD3ES4lTV" //e2e test validator2 publicKey
	publicKeyStrings[2] = "F/hszrhryPzK1FyeLPNY1QH7zwS8R6nysbNZWYq0wTwSAm8Yw430zO9ydhRMSU5L" //e2e test validator3 publicKey
	publicKeyStrings[3] = "Cm+p57XbZwromhMf9QmynKgD1Gtp4ZB8O6WKE2IUZIIj+2LmJE+Ib+m5ZCbVA/2c" //e2e test validator4 publicKey
	publicKeys := make([]crypto.PubKey, 4)
	for i, publicKeyString := range publicKeyStrings {
		decodedPublicKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyString)
		require.NoError(t, err)
		publicKeys[i] = bls12381.PubKey(decodedPublicKeyBytes)
		require.Equal(t, privateKeys[i].PubKey().Bytes(), publicKeys[i].Bytes())
	}
	thresholdPublicKey, err := bls12381.RecoverThresholdPublicKeyFromPublicKeys(publicKeys, proTxHashes)
	require.NoError(t, err)
	expectedThresholdPublicKeyString := "hXu9c6m/mA0TkJuc65caeHthsyIAXJnbNtsa7RwfZZtoqPlRtfNdLfY90E5QS+gz"
	encodedThresholdPublicKey := base64.StdEncoding.EncodeToString(thresholdPublicKey.Bytes())
	require.Equal(t, expectedThresholdPublicKeyString, encodedThresholdPublicKey)
}

func TestRecoverThresholdPublicKeyFromPublicKeys5(t *testing.T) {
	proTxHashStrings := make([]string, 5)
	proTxHashStrings[0] = "FDC09407DA9473CDC5E5AFCBB55712C95765343B2AF900B28BE4004E69CEDBB3" //e2e test validator1 protxhash
	proTxHashStrings[1] = "02AE8AAAF330949260BA537B05E408CEFA162FFA7CBB04C6C95BE2F922650A9D" //e2e test validator2 protxhash
	proTxHashStrings[2] = "036BCAEE159C09B75FD6404FAD6E76620AF595EF74734572E9D3DB4C226466FD" //e2e test validator3 protxhash
	proTxHashStrings[3] = "04FFEFD49498E2FC51A6FEB7FC695ED614EEA24B1C42F62AECDCA36B0FDCC026" //e2e test validator4 protxhash
	proTxHashStrings[4] = "0552e56ba3564c124ac8a6a0fa481219d100478566e81c7943507296b14484ae" //e2e test validator5 protxhash
	proTxHashes := make([][]byte, 5)
	for i, proTxHashString := range proTxHashStrings {
		decodedProTxHash, err := hex.DecodeString(proTxHashString)
		require.NoError(t, err)
		proTxHashes[i] = decodedProTxHash
	}
	privateKeyStrings := make([]string, 5)
	privateKeyStrings[0] = "BT0evsfM4r7Cc5lvbrVjBZuo1FYjMeIFg/6u7gb35M4=" //e2e test validator1 privateKey
	privateKeyStrings[1] = "A+Kn7ACPalXguwaBim+uLrPoYm9TnOTL8M9sdcu6xfs=" //e2e test validator2 privateKey
	privateKeyStrings[2] = "aFK6bDN2X3S/67ESBlrCg/kOHPyjRAtHLeor6aEk/mI=" //e2e test validator3 privateKey
	privateKeyStrings[3] = "Si++rgi0fAxhDAajuTXdPBoBWxzbvHPSvF8EeFm5b9A=" //e2e test validator4 privateKey
	privateKeyStrings[4] = "TI+nrLn/rJbOT/Q44QGNTX5ElmnTMT2Mb6E5mujOLYY=" //e2e test validator5 privateKey
	privateKeys := make([]crypto.PrivKey, 5)
	for i, privateKeyString := range privateKeyStrings {
		decodedPrivateKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyString)
		require.NoError(t, err)
		privateKeys[i] = bls12381.PrivKey(decodedPrivateKeyBytes)
	}
	publicKeyStrings := make([]string, 5)
	publicKeyStrings[0] = "l/cGlqBfWFP3LggkKGBjh3PAOYi5vNRrTjjaey9mxUuMHHegpGMDayxKhkaWq1vr" //e2e test validator1 publicKey
	publicKeyStrings[1] = "AmOAMHT33gNypyJGEFm2EEj8c9xbAddlAsVgDWEKevVZ7OIZEUCICZPgD3ES4lTV" //e2e test validator2 publicKey
	publicKeyStrings[2] = "F/hszrhryPzK1FyeLPNY1QH7zwS8R6nysbNZWYq0wTwSAm8Yw430zO9ydhRMSU5L" //e2e test validator3 publicKey
	publicKeyStrings[3] = "Cm+p57XbZwromhMf9QmynKgD1Gtp4ZB8O6WKE2IUZIIj+2LmJE+Ib+m5ZCbVA/2c" //e2e test validator4 publicKey
	publicKeyStrings[4] = "DBPbXPvwnlId5bBdsTYvjTGnpGe7HHAaOkEXn4yrHHgbzz/pfxNgCoASf9l1v5tg" //e2e test validator5 publicKey
	publicKeys := make([]crypto.PubKey, 5)
	for i, publicKeyString := range publicKeyStrings {
		decodedPublicKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyString)
		require.NoError(t, err)
		publicKeys[i] = bls12381.PubKey(decodedPublicKeyBytes)
		require.Equal(t, privateKeys[i].PubKey().Bytes(), publicKeys[i].Bytes())
	}
	thresholdPublicKey, err := bls12381.RecoverThresholdPublicKeyFromPublicKeys(publicKeys, proTxHashes)
	require.NoError(t, err)
	expectedThresholdPublicKeyString := "FvssjZ2t8CeKdNz2mYa/zuJYEVbCGW9GGKucaXlTUJscgBeScgGf3StNkVauL9pQ"
	encodedThresholdPublicKey := base64.StdEncoding.EncodeToString(thresholdPublicKey.Bytes())
	require.Equal(t, expectedThresholdPublicKeyString, encodedThresholdPublicKey)
}

func TestPublicKeyGeneration(t *testing.T) {
	decodedPrivateKeyBytes, err := base64.StdEncoding.DecodeString("BAo7smfbXWCycH2gctnV2aKTFWcNk/lCXrLahGZeay4=")
	require.NoError(t, err)
	privateKey := bls12381.PrivKey(decodedPrivateKeyBytes)
	expectedPublicKeyString := "BBdEXubJCrsGbU3vFyYpfQs1F9iuj6YBB6mc6ntizjX7bh8mnEWk3NkBEs/cVVfN"
	encodedPublicKeyString := base64.StdEncoding.EncodeToString(privateKey.PubKey().Bytes())
	require.Equal(t, expectedPublicKeyString, encodedPublicKeyString)
}

func TestAggregationDiffMessages(t *testing.T) {
	privKey := bls12381.GenPrivKey()
	pubKey := privKey.PubKey()
	msg1 := crypto.CRandBytes(128)
	msg2 := crypto.CRandBytes(128)
	msg3 := crypto.CRandBytes(128)
	sig1, err := privKey.Sign(msg1)
	require.Nil(t, err)
	sig2, err := privKey.Sign(msg2)
	require.Nil(t, err)
	sig3, err := privKey.Sign(msg3)
	require.Nil(t, err)

	// Test the signature
	assert.True(t, pubKey.VerifySignature(msg1, sig1))
	assert.True(t, pubKey.VerifySignature(msg2, sig2))
	assert.True(t, pubKey.VerifySignature(msg3, sig3))

	var signatures [][]byte
	var wrongSignatures [][]byte
	var messages [][]byte
	var wrongMessages [][]byte
	signatures = append(signatures, sig1)
	signatures = append(signatures, sig2)
	wrongSignatures = append(wrongSignatures, sig1)
	wrongSignatures = append(wrongSignatures, sig3)
	messages = append(messages, msg1)
	messages = append(messages, msg2)
	wrongMessages = append(wrongMessages, msg1)
	wrongMessages = append(wrongMessages, msg3)

	aggregateSignature, err := pubKey.AggregateSignatures(signatures, messages)
	require.Nil(t, err)
	wrongAggregateSignature, err := pubKey.AggregateSignatures(wrongSignatures, messages)
	require.Nil(t, err)

	assert.True(t, pubKey.VerifyAggregateSignature(messages, aggregateSignature))
	assert.False(t, pubKey.VerifyAggregateSignature(wrongMessages, aggregateSignature))
	assert.False(t, pubKey.VerifyAggregateSignature(messages, wrongAggregateSignature))
}
