package multisig

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
)

func TestThresholdMultisig(t *testing.T) {
	msg := []byte{1, 2, 3, 4}
	pubkeys, sigs := generatePubKeysAndSignatures(5, msg)
	multisigKey := NewThresholdMultiSignaturePubKey(2, pubkeys)
	multisignature := NewMultisig(5)
	require.False(t, multisigKey.VerifyBytes(msg, multisignature.Marshal()))
	multisignature.AddSignatureFromPubkey(sigs[0], pubkeys[0], pubkeys)
	require.False(t, multisigKey.VerifyBytes(msg, multisignature.Marshal()))
	// Make sure adding the same signature twice doesn't make the signature pass
	multisignature.AddSignatureFromPubkey(sigs[0], pubkeys[0], pubkeys)
	require.False(t, multisigKey.VerifyBytes(msg, multisignature.Marshal()))

	// Adding two signatures should make it pass, as k = 2
	multisignature.AddSignatureFromPubkey(sigs[3], pubkeys[3], pubkeys)
	require.True(t, multisigKey.VerifyBytes(msg, multisignature.Marshal()))

	// Adding a third invalid signature should make verification fail.
	multisignature.AddSignatureFromPubkey(sigs[0], pubkeys[4], pubkeys)
	require.False(t, multisigKey.VerifyBytes(msg, multisignature.Marshal()))

	// try adding the invalid signature one signature before
	// first reset the multisig
	multisignature.BitArray.SetIndex(4, false)
	multisignature.Sigs = multisignature.Sigs[:2]
	multisignature.AddSignatureFromPubkey(sigs[0], pubkeys[2], pubkeys)
	require.False(t, multisigKey.VerifyBytes(msg, multisignature.Marshal()))
}

func TestMultiSigPubkeyEquality(t *testing.T) {
	msg := []byte{1, 2, 3, 4}
	pubkeys, _ := generatePubKeysAndSignatures(5, msg)
	multisigKey := NewThresholdMultiSignaturePubKey(2, pubkeys)
	var unmarshalledMultisig *ThresholdMultiSignaturePubKey
	cdc.MustUnmarshalBinary(multisigKey.Bytes(), &unmarshalledMultisig)
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
