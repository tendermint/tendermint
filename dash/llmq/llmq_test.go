package llmq

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
)

func Test100Members(t *testing.T) {
	testCases := []struct {
		name      string
		n         int
		threshold int
		omit      int
	}{
		{
			name:      "test 100 members at threshold",
			n:         100,
			threshold: 67,
			omit:      33,
		},
		{
			name:      "test 100 members over threshold",
			n:         100,
			threshold: 67,
			omit:      rand.Intn(34),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			llmq, err := Generate(crypto.RandProTxHashes(tc.n), WithThreshold(tc.threshold))
			require.NoError(t, err)
			proTxHashesBytes := make([][]byte, tc.n)
			for i, proTxHash := range llmq.ProTxHashes {
				proTxHashesBytes[i] = proTxHash
			}
			signID := crypto.CRandBytes(32)
			signatures := make([][]byte, tc.n)
			for i, privKey := range llmq.PrivKeyShares {
				signatures[i], err = privKey.SignDigest(signID)
				require.NoError(t, err)
			}
			offset := 0
			if tc.omit > 0 {
				offset = rand.Intn(tc.omit)
			}
			check := tc.n - tc.omit
			require.True(t, check > tc.threshold-1)
			sig, err := bls12381.RecoverThresholdSignatureFromShares(
				signatures[offset:check+offset], proTxHashesBytes[offset:check+offset],
			)
			require.NoError(t, err)
			verified := llmq.ThresholdPubKey.VerifySignatureDigest(signID, sig)
			require.True(t, verified, "offset %d check %d", offset, check)
		})
	}
}
