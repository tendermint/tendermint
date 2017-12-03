package lite_test

import (
	"fmt"
	"testing"

	"github.com/tendermint/tendermint/lite"
)

func BenchmarkGenCommit20(b *testing.B) {
	keys := lite.GenValKeys(20)
	benchmarkGenCommit(b, keys)
}

func BenchmarkGenCommit100(b *testing.B) {
	keys := lite.GenValKeys(100)
	benchmarkGenCommit(b, keys)
}

func BenchmarkGenCommitSec20(b *testing.B) {
	keys := lite.GenSecpValKeys(20)
	benchmarkGenCommit(b, keys)
}

func BenchmarkGenCommitSec100(b *testing.B) {
	keys := lite.GenSecpValKeys(100)
	benchmarkGenCommit(b, keys)
}

func benchmarkGenCommit(b *testing.B, keys lite.ValKeys) {
	chainID := fmt.Sprintf("bench-%d", len(keys))
	vals := keys.ToValidators(20, 10)
	for i := 0; i < b.N; i++ {
		h := int64(1 + i)
		appHash := []byte(fmt.Sprintf("h=%d", h))
		keys.GenCommit(chainID, h, nil, vals, appHash, 0, len(keys))
	}
}

// this benchmarks generating one key
func BenchmarkGenValKeys(b *testing.B) {
	keys := lite.GenValKeys(20)
	for i := 0; i < b.N; i++ {
		keys = keys.Extend(1)
	}
}

// this benchmarks generating one key
func BenchmarkGenSecpValKeys(b *testing.B) {
	keys := lite.GenSecpValKeys(20)
	for i := 0; i < b.N; i++ {
		keys = keys.Extend(1)
	}
}

func BenchmarkToValidators20(b *testing.B) {
	benchmarkToValidators(b, 20)
}

func BenchmarkToValidators100(b *testing.B) {
	benchmarkToValidators(b, 100)
}

// this benchmarks constructing the validator set (.PubKey() * nodes)
func benchmarkToValidators(b *testing.B, nodes int) {
	keys := lite.GenValKeys(nodes)
	for i := 1; i <= b.N; i++ {
		keys.ToValidators(int64(2*i), int64(i))
	}
}

func BenchmarkToValidatorsSec100(b *testing.B) {
	benchmarkToValidatorsSec(b, 100)
}

// this benchmarks constructing the validator set (.PubKey() * nodes)
func benchmarkToValidatorsSec(b *testing.B, nodes int) {
	keys := lite.GenSecpValKeys(nodes)
	for i := 1; i <= b.N; i++ {
		keys.ToValidators(int64(2*i), int64(i))
	}
}

func BenchmarkCertifyCommit20(b *testing.B) {
	keys := lite.GenValKeys(20)
	benchmarkCertifyCommit(b, keys)
}

func BenchmarkCertifyCommit100(b *testing.B) {
	keys := lite.GenValKeys(100)
	benchmarkCertifyCommit(b, keys)
}

func BenchmarkCertifyCommitSec20(b *testing.B) {
	keys := lite.GenSecpValKeys(20)
	benchmarkCertifyCommit(b, keys)
}

func BenchmarkCertifyCommitSec100(b *testing.B) {
	keys := lite.GenSecpValKeys(100)
	benchmarkCertifyCommit(b, keys)
}

func benchmarkCertifyCommit(b *testing.B, keys lite.ValKeys) {
	chainID := "bench-certify"
	vals := keys.ToValidators(20, 10)
	cert := lite.NewStatic(chainID, vals)
	check := keys.GenCommit(chainID, 123, nil, vals, []byte("foo"), 0, len(keys))
	for i := 0; i < b.N; i++ {
		err := cert.Certify(check)
		if err != nil {
			panic(err)
		}
	}

}
