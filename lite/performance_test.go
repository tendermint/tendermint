package lite_test

import (
	"fmt"
	"math/rand"
	"sync"
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
		resHash := []byte(fmt.Sprintf("res=%d", h))
		keys.GenCommit(chainID, h, nil, vals, appHash, []byte("params"), resHash, 0, len(keys))
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
	cert := lite.NewStaticCertifier(chainID, vals)
	check := keys.GenCommit(chainID, 123, nil, vals, []byte("foo"), []byte("params"), []byte("res"), 0, len(keys))
	for i := 0; i < b.N; i++ {
		err := cert.Certify(check)
		if err != nil {
			panic(err)
		}
	}

}

type algo bool

const (
	linearSearch = true
	binarySearch = false
)

// Lazy load the commits
var fcs5, fcs50, fcs100, fcs500, fcs1000 []lite.FullCommit
var h5, h50, h100, h500, h1000 []int64
var commitsOnce sync.Once

func lazyGenerateFullCommits() {
	commitsOnce.Do(func() {
		fcs5, h5 = genFullCommits(nil, nil, 5)
		fcs50, h50 = genFullCommits(fcs5, h5, 50)
		fcs100, h100 = genFullCommits(fcs50, h50, 100)
		fcs500, h500 = genFullCommits(fcs100, h100, 500)
		fcs1000, h1000 = genFullCommits(fcs500, h500, 1000)
	})
}

func BenchmarkMemStoreProviderGetByHeightLinearSearch5(b *testing.B) {
	benchmarkMemStoreProviderGetByHeight(b, fcs5, h5, linearSearch)
}

func BenchmarkMemStoreProviderGetByHeightLinearSearch50(b *testing.B) {
	benchmarkMemStoreProviderGetByHeight(b, fcs50, h50, linearSearch)
}

func BenchmarkMemStoreProviderGetByHeightLinearSearch100(b *testing.B) {
	benchmarkMemStoreProviderGetByHeight(b, fcs100, h100, linearSearch)
}

func BenchmarkMemStoreProviderGetByHeightLinearSearch500(b *testing.B) {
	benchmarkMemStoreProviderGetByHeight(b, fcs500, h500, linearSearch)
}

func BenchmarkMemStoreProviderGetByHeightLinearSearch1000(b *testing.B) {
	benchmarkMemStoreProviderGetByHeight(b, fcs1000, h1000, linearSearch)
}

func BenchmarkMemStoreProviderGetByHeightBinarySearch5(b *testing.B) {
	benchmarkMemStoreProviderGetByHeight(b, fcs5, h5, binarySearch)
}

func BenchmarkMemStoreProviderGetByHeightBinarySearch50(b *testing.B) {
	benchmarkMemStoreProviderGetByHeight(b, fcs50, h50, binarySearch)
}

func BenchmarkMemStoreProviderGetByHeightBinarySearch100(b *testing.B) {
	benchmarkMemStoreProviderGetByHeight(b, fcs100, h100, binarySearch)
}

func BenchmarkMemStoreProviderGetByHeightBinarySearch500(b *testing.B) {
	benchmarkMemStoreProviderGetByHeight(b, fcs500, h500, binarySearch)
}

func BenchmarkMemStoreProviderGetByHeightBinarySearch1000(b *testing.B) {
	benchmarkMemStoreProviderGetByHeight(b, fcs1000, h1000, binarySearch)
}

var rng = rand.New(rand.NewSource(10))

func benchmarkMemStoreProviderGetByHeight(b *testing.B, fcs []lite.FullCommit, fHeights []int64, algo algo) {
	lazyGenerateFullCommits()

	b.StopTimer()
	mp := lite.NewMemStoreProvider()
	for i, fc := range fcs {
		if err := mp.StoreCommit(fc); err != nil {
			b.Fatalf("FullCommit #%d: err: %v", i, err)
		}
	}
	qHeights := make([]int64, len(fHeights))
	copy(qHeights, fHeights)
	// Append some non-existent heights to trigger the worst cases.
	qHeights = append(qHeights, 19, -100, -10000, 1e7, -17, 31, -1e9)

	searchFn := mp.GetByHeight
	if algo == binarySearch { // nolint
		searchFn = mp.(interface {
			GetByHeightBinarySearch(h int64) (lite.FullCommit, error)
		}).GetByHeightBinarySearch
	}

	hPerm := rng.Perm(len(qHeights))
	b.StartTimer()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, j := range hPerm {
			h := qHeights[j]
			if _, err := searchFn(h); err != nil {
			}
		}
	}
	b.ReportAllocs()
}

func genFullCommits(prevFC []lite.FullCommit, prevH []int64, want int) ([]lite.FullCommit, []int64) {
	fcs := make([]lite.FullCommit, len(prevFC))
	copy(fcs, prevFC)
	heights := make([]int64, len(prevH))
	copy(heights, prevH)

	appHash := []byte("benchmarks")
	chainID := "benchmarks-gen-full-commits"
	n := want
	keys := lite.GenValKeys(2 + (n / 3))
	for i := 0; i < n; i++ {
		vals := keys.ToValidators(10, int64(n/2))
		h := int64(20 + 10*i)
		fcs = append(fcs, keys.GenFullCommit(chainID, h, nil, vals, appHash, []byte("params"), []byte("results"), 0, 5))
		heights = append(heights, h)
	}
	return fcs, heights
}
