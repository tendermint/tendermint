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

	for idx, m := range manifests {
		t.Run(fmt.Sprintf("Case%04d", idx), func(t *testing.T) {
			infra, err := e2e.NewDockerInfrastructureData(m)
			require.NoError(t, err)
			_, err = e2e.NewTestnetFromManifest(m, filepath.Join(t.TempDir(), fmt.Sprintf("Case%04d", idx)), infra)
			require.NoError(t, err)
		})
	}
}
