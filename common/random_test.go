package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

// It is essential that these tests run and never repeat their outputs
// lest we've been pwned and the behavior of our randomness is controlled.
// See Issues:
//  * https://github.com/tendermint/tmlibs/issues/99
//  * https://github.com/tendermint/tendermint/issues/973
func TestUniqueRng(t *testing.T) {
	buf := new(bytes.Buffer)
	outputs := make(map[string][]int)
	for i := 0; i < 100; i++ {
		testThemAll(buf)
		output := buf.String()
		buf.Reset()
		runs, seen := outputs[output]
		if seen {
			t.Errorf("Run #%d's output was already seen in previous runs: %v", i, runs)
		}
		outputs[output] = append(outputs[output], i)
	}
}

func testThemAll(out io.Writer) {
	// Reset the internal PRNG
	reset()

	// Set math/rand's Seed so that any direct invocations
	// of math/rand will reveal themselves.
	mrand.Seed(1)
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
	fmt.Fprintf(out, "randUint16Exp: %d\n", RandUint16Exp())
	fmt.Fprintf(out, "randUint32Exp: %d\n", RandUint32Exp())
	fmt.Fprintf(out, "randUint64Exp: %d\n", RandUint64Exp())
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
