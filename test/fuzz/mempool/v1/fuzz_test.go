package v1_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	mempoolv1 "github.com/tendermint/tendermint/test/fuzz/mempool/v1"
)

const testdataCasesDir = "testdata/cases"

func TestMempoolTestdataCases(t *testing.T) {
	entries, err := os.ReadDir(testdataCasesDir)
	require.NoError(t, err)

	for _, e := range entries {
		entry := e
		t.Run(entry.Name(), func(t *testing.T) {
			defer func() {
				r := recover()
				require.Nilf(t, r, "testdata/cases test panic")
			}()
			f, err := os.Open(filepath.Join(testdataCasesDir, entry.Name()))
			require.NoError(t, err)
			input, err := io.ReadAll(f)
			require.NoError(t, err)
			mempoolv1.Fuzz(input)
		})
	}
}
