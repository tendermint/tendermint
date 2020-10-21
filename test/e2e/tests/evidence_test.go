package e2e_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/types"
)

// assert that all nodes that have blocks during the height (or height + 1) of a misbehavior has evidence
// for that misbehavior
func TestEvidence_Misbehavior(t *testing.T) {
	blocks := fetchBlockChain(t)
	testNode(t, func(t *testing.T, node e2e.Node) {
		for _, block := range blocks {
			// Find any evidence blaming this node in this block
			var nodeEvidence types.Evidence
			for _, evidence := range block.Evidence.Evidence {
				switch evidence := evidence.(type) {
				case *types.DuplicateVoteEvidence:
					if bytes.Equal(evidence.VoteA.ValidatorAddress, node.Key.PubKey().Address()) {
						nodeEvidence = evidence
					}
				default:
					t.Fatalf("unexpected evidence type %T", evidence)
				}
			}

			// Check that evidence was as expected (evidence is submitted in following height)
			misbehavior, ok := node.Misbehaviors[block.Height-1]
			if !ok {
				require.Nil(t, nodeEvidence, "found unexpected evidence %v in height %v",
					nodeEvidence, block.Height)
				continue
			}
			require.NotNil(t, nodeEvidence, "no evidence found for misbehavior %v in height %v",
				misbehavior, block.Height)

			switch misbehavior {
			case "double-prevote":
				require.IsType(t, &types.DuplicateVoteEvidence{}, nodeEvidence, "unexpected evidence type")
			default:
				t.Fatalf("unknown misbehavior %v", misbehavior)
			}
		}
	})
}
