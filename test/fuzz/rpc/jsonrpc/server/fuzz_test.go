package server_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/test/fuzz/rpc/jsonrpc/server"
)

const testdataCasesDir = "testdata/cases"

func TestServerTestdataCases(t *testing.T) {
	entries, err := os.ReadDir(testdataCasesDir)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
			require.NoError(t, server.Fuzz(ctx, input))
		})
	}
}
