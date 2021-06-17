package e2e_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/types"
)

// assert that all nodes that have blocks at the height of a misbehavior has evidence
// for that misbehavior
func TestEvidence_Misbehavior(t *testing.T) {
	blocks := fetchBlockChain(t)
	testNode(t, func(t *testing.T, node e2e.Node) {
		seenEvidence := make(map[int64]struct{})
		for _, block := range blocks {
			// Find any evidence blaming this node in this block
			var nodeEvidence types.Evidence
			for _, evidence := range block.Evidence.Evidence {
				switch evidence := evidence.(type) {
				case *types.DuplicateVoteEvidence:
					if bytes.Equal(evidence.VoteA.ValidatorProTxHash, node.ProTxHash) {
						nodeEvidence = evidence
					}
				default:
					t.Fatalf("unexpected evidence type %T", evidence)
				}
			}
			if nodeEvidence == nil {
				continue // no evidence for the node at this height
			}

			// Check that evidence was as expected
			misbehavior, ok := node.Misbehaviors[nodeEvidence.Height()]
			require.True(t, ok, "found unexpected evidence %v in height %v",
				nodeEvidence, block.Height)

			switch misbehavior {
			case "double-prevote":
				require.IsType(t, &types.DuplicateVoteEvidence{}, nodeEvidence, "unexpected evidence type")
			default:
				t.Fatalf("unknown misbehavior %v", misbehavior)
			}

			seenEvidence[nodeEvidence.Height()] = struct{}{}
		}
		// see if there is any evidence that we were expecting but didn't see
		for height, misbehavior := range node.Misbehaviors {
			_, ok := seenEvidence[height]
			require.True(t, ok, "expected evidence for %v misbehavior at height %v by node but was never found",
				misbehavior, height)
		}
	})
}
