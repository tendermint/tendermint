package consensus

import (
	"github.com/tendermint/tendermint/config/tendermint_test"
	"github.com/tendermint/tendermint/types"

	"testing"
)

func init() {
	tendermint_test.ResetConfig("consensus_height_vote_set_test")
}

func TestPeerCatchupRounds(t *testing.T) {
	valSet, privVals := types.RandValidatorSet(10, 1)

	hvs := NewHeightVoteSet(1, valSet)

	vote999_0 := makeVoteHR(t, 1, 999, privVals[0])
	added, _, err := hvs.AddByIndex(0, vote999_0, "peer1")
	if !added || err != nil {
		t.Error("Expected to successfully add vote from peer", added, err)
	}

	vote1000_0 := makeVoteHR(t, 1, 1000, privVals[0])
	added, _, err = hvs.AddByIndex(0, vote1000_0, "peer1")
	if added {
		t.Error("Expected to *not* add vote from peer, too many catchup rounds.")
	}

	added, _, err = hvs.AddByIndex(0, vote1000_0, "peer2")
	if !added || err != nil {
		t.Error("Expected to successfully add vote from another peer")
	}

}

func makeVoteHR(t *testing.T, height, round int, privVal *types.PrivValidator) *types.Vote {
	vote := &types.Vote{
		Height:    height,
		Round:     round,
		Type:      types.VoteTypePrecommit,
		BlockHash: []byte("fakehash"),
	}
	chainID := config.GetString("chain_id")
	err := privVal.SignVote(chainID, vote)
	if err != nil {
		t.Fatalf("Error signing vote: %v", err)
		return nil
	}
	return vote
}
