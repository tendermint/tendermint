package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	mrand "math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRandStr(t *testing.T) {
	l := 243
	s := RandStr(l)
	assert.Equal(t, l, len(s))
}

func TestRandBytes(t *testing.T) {
	l := 243
	b := RandBytes(l)
	assert.Equal(t, l, len(b))
}

func TestRandIntn(t *testing.T) {
	n := 243
	for i := 0; i < 100; i++ {
		x := RandIntn(n)
		assert.True(t, x < n)
	}
}

// Test to make sure that we never call math.rand().
// We do this by ensuring that outputs are deterministic.
func TestDeterminism(t *testing.T) {
	var firstOutput string

	// Set math/rand's seed for the sake of debugging this test.
	// (It isn't strictly necessary).
	mrand.Seed(1)

	for i := 0; i < 100; i++ {
		output := testThemAll()
		if i == 0 {
			firstOutput = output
		} else if firstOutput != output {
			t.Errorf("Run #%d's output was different from first run.\nfirst: %v\nlast: %v",
				i, firstOutput, output)
		}
	}
}

func testThemAll() string {

	// Such determinism.
	grand.reset(1)

	// Use it.
	out := new(bytes.Buffer)
	perm := RandPerm(10)
	blob, _ := json.Marshal(perm)
	fmt.Fprintf(out, "perm: %s\n", blob)
	fmt.Fprintf(out, "randInt: %d\n", RandInt())
	fmt.Fprintf(out, "randUint: %d\n", RandUint())
	fmt.Fprintf(out, "randIntn: %d\n", RandIntn(97))
	fmt.Fprintf(out, "randInt31: %d\n", RandInt31())
	fmt.Fprintf(out, "randInt32: %d\n", RandInt32())
	fmt.Fprintf(out, "randInt63: %d\n", RandInt63())
	fmt.Fprintf(out, "randInt64: %d\n", RandInt64())
	fmt.Fprintf(out, "randUint32: %d\n", RandUint32())
	fmt.Fprintf(out, "randUint64: %d\n", RandUint64())
	return out.String()
}

func TestRngConcurrencySafety(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_ = RandUint64()
			<-time.After(time.Millisecond * time.Duration(RandIntn(100)))
			_ = RandPerm(3)
		}()
	}
	wg.Wait()
}

func BenchmarkRandBytes10B(b *testing.B) {
	benchmarkRandBytes(b, 10)
}
func BenchmarkRandBytes100B(b *testing.B) {
	benchmarkRandBytes(b, 100)
}
func BenchmarkRandBytes1KiB(b *testing.B) {
	benchmarkRandBytes(b, 1024)
}
func BenchmarkRandBytes10KiB(b *testing.B) {
	benchmarkRandBytes(b, 10*1024)
}
func BenchmarkRandBytes100KiB(b *testing.B) {
	benchmarkRandBytes(b, 100*1024)
}
func BenchmarkRandBytes1MiB(b *testing.B) {
	benchmarkRandBytes(b, 1024*1024)
}

func benchmarkRandBytes(b *testing.B, n int) {
	for i := 0; i < b.N; i++ {
		_ = RandBytes(n)
	}
	b.ReportAllocs()
}
