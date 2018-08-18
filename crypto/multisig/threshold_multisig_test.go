package multisig

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
)

// This tests multisig functionality, but it expects the first k signatures to be valid
// TODO: Adapt it to give more flexibility about first k signatures being valid
func TestThresholdMultisigValidCases(t *testing.T) {
	pkSet1, sigSet1 := generatePubKeysAndSignatures(5, []byte{1, 2, 3, 4})
	cases := []struct {
		msg            []byte
		k              int
		pubkeys        []crypto.PubKey
		signingIndices []int
		// signatures should be the same size as signingIndices.
		signatures           [][]byte
		passAfterKSignatures []bool
	}{
		{
			msg:                  []byte{1, 2, 3, 4},
			k:                    2,
			pubkeys:              pkSet1,
			signingIndices:       []int{0, 3, 1},
			signatures:           sigSet1,
			passAfterKSignatures: []bool{false},
		},
	}
	for tcIndex, tc := range cases {
		multisigKey := NewThresholdMultiSignaturePubKey(tc.k, tc.pubkeys)
		multisignature := NewMultisig(len(tc.pubkeys))
		for i := 0; i < tc.k-1; i++ {
			signingIndex := tc.signingIndices[i]
			multisignature.AddSignatureFromPubkey(tc.signatures[signingIndex], tc.pubkeys[signingIndex], tc.pubkeys)
			require.False(t, multisigKey.VerifyBytes(tc.msg, multisignature.Marshal()),
				"multisig passed when i < k, tc %d, i %d", tcIndex, i)
			multisignature.AddSignatureFromPubkey(tc.signatures[signingIndex], tc.pubkeys[signingIndex], tc.pubkeys)
			require.Equal(t, i+1, len(multisignature.Sigs),
				"adding a signature for the same pubkey twice increased signature count by 2, tc %d", tcIndex)
		}
		require.False(t, multisigKey.VerifyBytes(tc.msg, multisignature.Marshal()),
			"multisig passed with k - 1 sigs, tc %d", tcIndex)
		multisignature.AddSignatureFromPubkey(tc.signatures[tc.signingIndices[tc.k]], tc.pubkeys[tc.signingIndices[tc.k]], tc.pubkeys)
		require.True(t, multisigKey.VerifyBytes(tc.msg, multisignature.Marshal()),
			"multisig failed after k good signatures, tc %d", tcIndex)
		for i := tc.k + 1; i < len(tc.signingIndices); i++ {
			signingIndex := tc.signingIndices[i]
			multisignature.AddSignatureFromPubkey(tc.signatures[signingIndex], tc.pubkeys[signingIndex], tc.pubkeys)
			require.Equal(t, tc.passAfterKSignatures[i-tc.k-1],
				multisigKey.VerifyBytes(tc.msg, multisignature.Marshal()),
				"multisig didn't verify as expected after k sigs, tc %d, i %d", tcIndex, i)

			multisignature.AddSignatureFromPubkey(tc.signatures[signingIndex], tc.pubkeys[signingIndex], tc.pubkeys)
			require.Equal(t, i+1, len(multisignature.Sigs),
				"adding a signature for the same pubkey twice increased signature count by 2, tc %d", tcIndex)
		}
	}
}

// TODO: Fully replace this test with table driven tests
func TestThresholdMultisigDuplicateSignatures(t *testing.T) {
	msg := []byte{1, 2, 3, 4, 5}
	pubkeys, sigs := generatePubKeysAndSignatures(5, msg)
	multisigKey := NewThresholdMultiSignaturePubKey(2, pubkeys)
	multisignature := NewMultisig(5)
	require.False(t, multisigKey.VerifyBytes(msg, multisignature.Marshal()))
	multisignature.AddSignatureFromPubkey(sigs[0], pubkeys[0], pubkeys)
	// Add second signature manually
	multisignature.Sigs = append(multisignature.Sigs, sigs[0])
	require.False(t, multisigKey.VerifyBytes(msg, multisignature.Marshal()))
}

// TODO: Fully replace this test with table driven tests
func TestMultiSigPubkeyEquality(t *testing.T) {
	msg := []byte{1, 2, 3, 4}
	pubkeys, _ := generatePubKeysAndSignatures(5, msg)
	multisigKey := NewThresholdMultiSignaturePubKey(2, pubkeys)
	var unmarshalledMultisig *ThresholdMultiSignaturePubKey
	cdc.MustUnmarshalBinaryBare(multisigKey.Bytes(), &unmarshalledMultisig)
	require.True(t, multisigKey.Equals(unmarshalledMultisig))

	// Ensure that reordering pubkeys is treated as a different pubkey
	pubkeysCpy := make([]crypto.PubKey, 5)
	copy(pubkeysCpy, pubkeys)
	pubkeysCpy[4] = pubkeys[3]
	pubkeysCpy[3] = pubkeys[4]
	multisigKey2 := NewThresholdMultiSignaturePubKey(2, pubkeysCpy)
	require.False(t, multisigKey.Equals(multisigKey2))
}

func generatePubKeysAndSignatures(n int, msg []byte) (pubkeys []crypto.PubKey, signatures [][]byte) {
	pubkeys = make([]crypto.PubKey, n)
	signatures = make([][]byte, n)
	for i := 0; i < n; i++ {
		var privkey crypto.PrivKey
		if rand.Int63()%2 == 0 {
			privkey = ed25519.GenPrivKey()
		} else {
			privkey = secp256k1.GenPrivKey()
		}
		pubkeys[i] = privkey.PubKey()
		signatures[i], _ = privkey.Sign(msg)
	}
	return
}
