package server_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/test/fuzz/rpc/jsonrpc/server"
)

const testdataDir = "testdata"

func TestServerOnTestData(t *testing.T) {
	entries, err := os.ReadDir(testdataDir)
	require.NoError(t, err)

	for _, e := range entries {
		entry := e
		t.Run(entry.Name(), func(t *testing.T) {
			defer func() {
				r := recover()
				require.Nilf(t, r, "testdata test panic")
			}()
			f, err := os.Open(filepath.Join(testdataDir, entry.Name()))
			require.NoError(t, err)
			input, err := ioutil.ReadAll(f)
			require.NoError(t, err)
			server.Fuzz(input)
		})
	}
}
