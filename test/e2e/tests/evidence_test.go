package e2e_test

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cs "github.com/tendermint/tendermint/test/e2e/maverick/consensus"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/types"
)

// assert that all nodes that have blocks during the height (or height + 1) of a misbehavior has evidence
// for that misbehavior
func TestEvidence_Misbehavior(t *testing.T) {
	networkMisbehaviors := fetchMisbehaviors(t)
	testNode(t, func(t *testing.T, node e2e.Node) {
		client, err := node.Client()
		require.NoError(t, err)
		status, err := client.Status(ctx)
		require.NoError(t, err)

		for height, misbehaviorsAtThisHeight := range networkMisbehaviors {
			if height < status.SyncInfo.EarliestBlockHeight || height+1 > status.SyncInfo.LatestBlockHeight {
				// node has either pruned this block or hasn't received it yet (this could be because the misbehavior
				// hasn't happened yet)
				continue
			}
			for _, misbehavior := range misbehaviorsAtThisHeight {
				if misbehavior.String == "double-prevote" && height < status.SyncInfo.LatestBlockHeight {
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
		}

	})
}

func fetchMisbehaviors(t *testing.T) map[int64][]cs.Misbehavior {
	t.Helper()

	networkMisbehaviors := make(map[int64][]cs.Misbehavior)

	testnet := loadTestnet(t)
	for _, node := range testnet.Nodes {
		if len(node.Misbehaviors) != 0 {
			for heightString, misbehaviorString := range node.Misbehaviors {
				// should have already been validated
				misbehavior, ok := cs.MisbehaviorList[misbehaviorString]
				if !ok {
					t.Fatalf("an unknown misbehavior was used %s", misbehaviorString)
				}
				height, err := strconv.ParseInt(heightString, 10, 64)
				if err != nil {
					t.Fatalf("unable to parse height (%s) of misbehavior, err: %v", heightString, err)
				}

				// if we already have an attack at this height we append to the behaviors else create a new one
				if misbehaviors, ok := networkMisbehaviors[height]; ok {
					networkMisbehaviors[height] = append(misbehaviors, misbehavior)
				} else {
					networkMisbehaviors[height] = []cs.Misbehavior{misbehavior}
				}
			}
		}
	}
	return networkMisbehaviors
}
