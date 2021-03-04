package sr25519

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/internal/benchmarking"
)

func BenchmarkKeyGeneration(b *testing.B) {
	benchmarkKeygenWrapper := func(reader io.Reader) crypto.PrivKey {
		return genPrivKey(reader)
	}
	benchmarking.BenchmarkKeyGeneration(b, benchmarkKeygenWrapper)
}

func BenchmarkSigning(b *testing.B) {
	priv := GenPrivKey()
	benchmarking.BenchmarkSigning(b, priv)
}

func BenchmarkVerification(b *testing.B) {
	priv := GenPrivKey()
	benchmarking.BenchmarkVerification(b, priv)
}

func BenchmarkVerifyBatch(b *testing.B) {
	for _, n := range []int{1, 8, 64, 1024} {
		n := n
		b.Run(fmt.Sprintf("sig-count-%d", n), func(b *testing.B) {
			b.ReportAllocs()
			v := NewBatchVerifier()
			for i := 0; i < n; i++ {
				priv := GenPrivKey()
				pub := priv.PubKey()
				msg := []byte("BatchVerifyTest")
				sig, _ := priv.Sign(msg)
				err := v.Add(pub, msg, sig)
				require.NoError(b, err)
			}
			// NOTE: dividing by n so that metrics are per-signature
			for i := 0; i < b.N/n; i++ {
				if !v.Verify() {
					b.Fatal("signature set failed batch verification")
				}
			}
		})
	}
}
