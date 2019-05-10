// +build !libsecp256k1

package secp256k1

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/secp256k1"
)

// Ensure that signature verification works, and that
// non-canonical signatures fail.
// // Note: run with CGO_ENABLED=0 or go test -tags !cgo.
func TestSignatureVerificationAndRejectUpperS(t *testing.T) {
	msg := []byte("We have lingered long enough on the shores of the cosmic ocean.")
	for i := 0; i < 500; i++ {
		priv := GenPrivKey()
		sigStr, err := priv.Sign(msg)
		require.NoError(t, err)
		sig := signatureFromBytes(sigStr)
		require.False(t, sig.S.Cmp(secp256k1halfN) > 0)

		pub := priv.PubKey()
		addr := pub.Address()
		t.Log("address ", addr)

		require.True(t, pub.VerifyBytes(msg, sigStr))

		// malleate:
		sig.S.Sub(secp256k1.S256().CurveParams.N, sig.S)
		require.True(t, sig.S.Cmp(secp256k1halfN) > 0)
		malSigStr := serializeSig(sig)

		require.False(t, pub.VerifyBytes(msg, malSigStr),
			"VerifyBytes incorrect with malleated & invalid S. sig=%v, key=%v",
			sig,
			priv,
		)
	}
}

func TestGenEthPrivKey(t *testing.T) {
	msg := []byte("We have lingered long enough on the shores of the cosmic ocean.")
	priv := GenPrivKey()
	t.Log("privkey ", priv)
	sigStr, err := priv.Sign(msg)
	require.NoError(t, err)
	pub := priv.PubKey()
	addr := pub.Address()
	t.Log("address ", addr)
	t.Log("pub ", pub)
	t.Log("SigStr ", sigStr)
}
