package main

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

func TestGenerator(t *testing.T) {
	manifests, err := Generate(rand.New(rand.NewSource(randomSeed)), Options{P2P: MixedP2PMode})
	require.NoError(t, err)
	require.True(t, len(manifests) > 16, "insufficient combinations")

	// this just means that the numbers reported by the test
	// failures map to the test cases that you'd see locally.
	e2e.SortManifests(manifests, false)

	for idx, m := range manifests {
		t.Run(fmt.Sprintf("Case%04d", idx), func(t *testing.T) {
			numStateSyncs := 0
			for name, node := range m.Nodes {
				if node.StateSync != e2e.StateSyncDisabled {
					numStateSyncs++
				}
				t.Run(name, func(t *testing.T) {
					if node.StartAt > m.InitialHeight+5 && !node.Stateless() {
						require.NotEqual(t, node.StateSync, e2e.StateSyncDisabled)
					}
					if node.StateSync != e2e.StateSyncDisabled {
						require.Zero(t, node.Seeds, node.StateSync)
						require.True(t, len(node.PersistentPeers) >= 2)
						require.Equal(t, "v0", node.BlockSync)
					}

				})
			}
			require.True(t, numStateSyncs <= 2)
		})
	}
}
