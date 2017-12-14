package common_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tendermint/tmlibs/common"
)

// It is essential that these tests run and never repeat their outputs
// lest we've been pwned and the behavior of our randomness is controlled.
// See Issues:
//  * https://github.com/tendermint/tmlibs/issues/99
//  * https://github.com/tendermint/tendermint/issues/973
func TestUniqueRng(t *testing.T) {
	if os.Getenv("TENDERMINT_INTEGRATION_TESTS") == "" {
		t.Skipf("Can only be run as an integration test")
	}

	// The goal of this test is to invoke the
	// Rand* tests externally with no repeating results, booted up.
	// Any repeated results indicate that the seed is the same or that
	// perhaps we are using math/rand directly.
	tmpDir, err := ioutil.TempDir("", "rng-tests")
	if err != nil {
		t.Fatalf("Creating tempDir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	outpath := filepath.Join(tmpDir, "main.go")
	f, err := os.Create(outpath)
	if err != nil {
		t.Fatalf("Setting up %q err: %v", outpath, err)
	}
	f.Write([]byte(integrationTestProgram))
	if err := f.Close(); err != nil {
		t.Fatalf("Closing: %v", err)
	}

	outputs := make(map[string][]int)
	for i := 0; i < 100; i++ {
		cmd := exec.Command("go", "run", outpath)
		bOutput, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("Run #%d: err: %v output: %s", i, err, bOutput)
			continue
		}
		output := string(bOutput)
		runs, seen := outputs[output]
		if seen {
			t.Errorf("Run #%d's output was already seen in previous runs: %v", i, runs)
		}
		outputs[output] = append(outputs[output], i)
	}
}

const integrationTestProgram = `
package main

import (
  "encoding/json"
  "fmt"
  "math/rand"

  "github.com/tendermint/tmlibs/common"
)

func main() {
  // Set math/rand's Seed so that any direct invocations
  // of math/rand will reveal themselves.
  rand.Seed(1)
  perm := common.RandPerm(10)
  blob, _ := json.Marshal(perm)
  fmt.Printf("perm: %s\n", blob)

  fmt.Printf("randInt: %d\n", common.RandInt())
  fmt.Printf("randUint: %d\n", common.RandUint())
  fmt.Printf("randIntn: %d\n", common.RandIntn(97))
  fmt.Printf("randInt31: %d\n", common.RandInt31())
  fmt.Printf("randInt32: %d\n", common.RandInt32())
  fmt.Printf("randInt63: %d\n", common.RandInt63())
  fmt.Printf("randInt64: %d\n", common.RandInt64())
  fmt.Printf("randUint32: %d\n", common.RandUint32())
  fmt.Printf("randUint64: %d\n", common.RandUint64())
  fmt.Printf("randUint16Exp: %d\n", common.RandUint16Exp())
  fmt.Printf("randUint32Exp: %d\n", common.RandUint32Exp())
  fmt.Printf("randUint64Exp: %d\n", common.RandUint64Exp())
}`

func TestRngConcurrencySafety(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_ = common.RandUint64()
			<-time.After(time.Millisecond * time.Duration(common.RandIntn(100)))
			_ = common.RandPerm(3)
		}()
	}
	wg.Wait()
}
