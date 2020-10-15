package e2e_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	nd "github.com/tendermint/tendermint/test/e2e/maverick/node"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/types"
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
			if behavior.String == "double-prevote" && height < status.SyncInfo.LatestBlockHeight {
				// we expect evidence to be formed in the height directly after hence height + 1
				var reportHeight int64 = height + 1
				resp, err := client.Block(ctx, &reportHeight)
				require.NoError(t, err)
				assert.NotEmpty(t, resp.Block.Evidence.Evidence, "no evidence seen of node %v equivocating at height %d",
					node.Name, height)
				containsMaverick := false
				// we expect that at least one of the evidence committed at this height will be duplicate vote evidence
				// and contain the maverick nodes address
				for _, ev := range resp.Block.Evidence.Evidence {
					if dev, ok := ev.(*types.DuplicateVoteEvidence); ok {
						if bytes.Equal(dev.VoteA.ValidatorAddress, node.Key.PubKey().Address()) {
							containsMaverick = true
						}
					}
				}
				assert.True(t, containsMaverick)
			}
		}

	})
}
