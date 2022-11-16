package main

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

// TestGenerator tests that only valid manifests are generated
func TestGenerator(t *testing.T) {
	manifests, err := Generate(rand.New(rand.NewSource(randomSeed)))
	require.NoError(t, err)
	require.True(t, len(manifests) >= 24, "insufficient combinations %d", len(manifests))

	for idx, m := range manifests {
		t.Run(fmt.Sprintf("Case%04d", idx), func(t *testing.T) {
			_, err := e2e.NewTestnetFromManifest(m, filepath.Join(t.TempDir(), fmt.Sprintf("Case%04d", idx)), e2e.InfrastructureData{})
			require.NoError(t, err)
		})
	}
}
