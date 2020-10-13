package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

  nd "github.com/tendermint/tendermint/test/e2e/maverick/node"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

func TestEvidence_DoubleVote(t *testing.T) {
	testNode(t, func(t *testing.T, node e2e.Node) {
		// We only use the maverick nodes to check misbehavior as
		// it's the only way to ensure that the network at least
		// has a maverick node
		if node.Behaviors == "" {
			return
		}

		client, err := node.Client()
		require.NoError(t, err)
		status, err := client.Status(ctx)
		require.NoError(t, err)

		behaviors, err := nd.ParseBehaviors(node.Behaviors)
		require.NoError(t, err)

		for height, behavior := range behaviors {
			if behavior.String == "double-prevote" && int64(height) < status.SyncInfo.LatestBlockHeight {
				// we expect evidence to be formed in the height directly after hence height + 1
				var reportHeight int64 = height + 1
				resp, err := client.Block(ctx, &reportHeight)
				require.NoError(t, err)
				assert.NotEmpty(t, resp.Block.Evidence, "no evidence seen of node %v equivocating at height %d",
					node.Name, height)
			}
		}

	})
}
