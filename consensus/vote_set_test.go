package consensus

import (
	"bytes"

	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/common/test"
	_ "github.com/tendermint/tendermint/config/tendermint_test"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"

	"testing"
)

// NOTE: see consensus/test.go for common test methods.

// Convenience: Return new vote with different height
func withHeight(vote *types.Vote, height uint) *types.Vote {
	vote = vote.Copy()
	vote.Height = height
	return vote
}

// Convenience: Return new vote with different round
func withRound(vote *types.Vote, round uint) *types.Vote {
	vote = vote.Copy()
	vote.Round = round
	return vote
}

// Convenience: Return new vote with different type
func withType(vote *types.Vote, type_ byte) *types.Vote {
	vote = vote.Copy()
	vote.Type = type_
	return vote
}

// Convenience: Return new vote with different blockHash
func withBlockHash(vote *types.Vote, blockHash []byte) *types.Vote {
	vote = vote.Copy()
	vote.BlockHash = blockHash
	return vote
}

// Convenience: Return new vote with different blockParts
func withBlockParts(vote *types.Vote, blockParts types.PartSetHeader) *types.Vote {
	vote = vote.Copy()
	vote.BlockParts = blockParts
	return vote
}

func signAddVote(privVal *sm.PrivValidator, vote *types.Vote, voteSet *VoteSet) (bool, error) {
	privVal.SignVoteUnsafe(config.GetString("chain_id"), vote)
	added, _, err := voteSet.Add(privVal.Address, vote)
	return added, err
}

func TestAddVote(t *testing.T) {
	height, round := uint(1), uint(0)
	voteSet, _, privValidators := randVoteSet(height, round, types.VoteTypePrevote, 10, 1)
	val0 := privValidators[0]

	// t.Logf(">> %v", voteSet)

	if voteSet.GetByAddress(val0.Address) != nil {
		t.Errorf("Expected GetByAddress(val0.Address) to be nil")
	}
	if voteSet.BitArray().GetIndex(0) {
		t.Errorf("Expected BitArray.GetIndex(0) to be false")
	}
	hash, header, ok := voteSet.TwoThirdsMajority()
	if hash != nil || !header.IsZero() || ok {
		t.Errorf("There should be no 2/3 majority")
	}

	vote := &types.Vote{Height: height, Round: round, Type: types.VoteTypePrevote, BlockHash: nil}
	signAddVote(val0, vote, voteSet)

	if voteSet.GetByAddress(val0.Address) == nil {
		t.Errorf("Expected GetByAddress(val0.Address) to be present")
	}
	if !voteSet.BitArray().GetIndex(0) {
		t.Errorf("Expected BitArray.GetIndex(0) to be true")
	}
	hash, header, ok = voteSet.TwoThirdsMajority()
	if hash != nil || !header.IsZero() || ok {
		t.Errorf("There should be no 2/3 majority")
	}
}

func Test2_3Majority(t *testing.T) {
	height, round := uint(1), uint(0)
	voteSet, _, privValidators := randVoteSet(height, round, types.VoteTypePrevote, 10, 1)

	vote := &types.Vote{Height: height, Round: round, Type: types.VoteTypePrevote, BlockHash: nil}

	// 6 out of 10 voted for nil.
	for i := 0; i < 6; i++ {
		signAddVote(privValidators[i], vote, voteSet)
	}
	hash, header, ok := voteSet.TwoThirdsMajority()
	if hash != nil || !header.IsZero() || ok {
		t.Errorf("There should be no 2/3 majority")
	}

	// 7th validator voted for some blockhash
	{
		signAddVote(privValidators[6], withBlockHash(vote, RandBytes(32)), voteSet)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if hash != nil || !header.IsZero() || ok {
			t.Errorf("There should be no 2/3 majority")
		}
	}

	// 8th validator voted for nil.
	{
		signAddVote(privValidators[7], vote, voteSet)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if hash != nil || !header.IsZero() || !ok {
			t.Errorf("There should be 2/3 majority for nil")
		}
	}
}

func Test2_3MajorityRedux(t *testing.T) {
	height, round := uint(1), uint(0)
	voteSet, _, privValidators := randVoteSet(height, round, types.VoteTypePrevote, 100, 1)

	blockHash := CRandBytes(32)
	blockPartsTotal := uint(123)
	blockParts := types.PartSetHeader{blockPartsTotal, CRandBytes(32)}

	vote := &types.Vote{Height: height, Round: round, Type: types.VoteTypePrevote, BlockHash: blockHash, BlockParts: blockParts}

	// 66 out of 100 voted for nil.
	for i := 0; i < 66; i++ {
		signAddVote(privValidators[i], vote, voteSet)
	}
	hash, header, ok := voteSet.TwoThirdsMajority()
	if hash != nil || !header.IsZero() || ok {
		t.Errorf("There should be no 2/3 majority")
	}

	// 67th validator voted for nil
	{
		signAddVote(privValidators[66], withBlockHash(vote, nil), voteSet)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if hash != nil || !header.IsZero() || ok {
			t.Errorf("There should be no 2/3 majority: last vote added was nil")
		}
	}

	// 68th validator voted for a different BlockParts PartSetHeader
	{
		blockParts := types.PartSetHeader{blockPartsTotal, CRandBytes(32)}
		signAddVote(privValidators[67], withBlockParts(vote, blockParts), voteSet)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if hash != nil || !header.IsZero() || ok {
			t.Errorf("There should be no 2/3 majority: last vote added had different PartSetHeader Hash")
		}
	}

	// 69th validator voted for different BlockParts Total
	{
		blockParts := types.PartSetHeader{blockPartsTotal + 1, blockParts.Hash}
		signAddVote(privValidators[68], withBlockParts(vote, blockParts), voteSet)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if hash != nil || !header.IsZero() || ok {
			t.Errorf("There should be no 2/3 majority: last vote added had different PartSetHeader Total")
		}
	}

	// 70th validator voted for different BlockHash
	{
		signAddVote(privValidators[69], withBlockHash(vote, RandBytes(32)), voteSet)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if hash != nil || !header.IsZero() || ok {
			t.Errorf("There should be no 2/3 majority: last vote added had different BlockHash")
		}
	}

	// 71st validator voted for the right BlockHash & BlockParts
	{
		signAddVote(privValidators[70], vote, voteSet)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if !bytes.Equal(hash, blockHash) || !header.Equals(blockParts) || !ok {
			t.Errorf("There should be 2/3 majority")
		}
	}
}

func TestBadVotes(t *testing.T) {
	height, round := uint(1), uint(0)
	voteSet, _, privValidators := randVoteSet(height, round, types.VoteTypePrevote, 10, 1)

	// val0 votes for nil.
	vote := &types.Vote{Height: height, Round: round, Type: types.VoteTypePrevote, BlockHash: nil}
	added, err := signAddVote(privValidators[0], vote, voteSet)
	if !added || err != nil {
		t.Errorf("Expected Add() to succeed")
	}

	// val0 votes again for some block.
	added, err = signAddVote(privValidators[0], withBlockHash(vote, RandBytes(32)), voteSet)
	if added || err == nil {
		t.Errorf("Expected Add() to fail, dupeout.")
	}

	// val1 votes on another height
	added, err = signAddVote(privValidators[1], withHeight(vote, height+1), voteSet)
	if added {
		t.Errorf("Expected Add() to fail, wrong height")
	}

	// val2 votes on another round
	added, err = signAddVote(privValidators[2], withRound(vote, round+1), voteSet)
	if added {
		t.Errorf("Expected Add() to fail, wrong round")
	}

	// val3 votes of another type.
	added, err = signAddVote(privValidators[3], withType(vote, types.VoteTypePrecommit), voteSet)
	if added {
		t.Errorf("Expected Add() to fail, wrong type")
	}
}

func TestAddCommitsToPrevoteVotes(t *testing.T) {
	height, round := uint(2), uint(5)
	voteSet, _, privValidators := randVoteSet(height, round, types.VoteTypePrevote, 10, 1)

	// val0, val1, val2, val3, val4, val5 vote for nil.
	vote := &types.Vote{Height: height, Round: round, Type: types.VoteTypePrevote, BlockHash: nil}
	for i := 0; i < 6; i++ {
		signAddVote(privValidators[i], vote, voteSet)
	}
	hash, header, ok := voteSet.TwoThirdsMajority()
	if hash != nil || !header.IsZero() || ok {
		t.Errorf("There should be no 2/3 majority")
	}

	// Attempt to add a commit from val6 at a previous height
	vote = &types.Vote{Height: height - 1, Round: round, Type: types.VoteTypeCommit, BlockHash: nil}
	added, _ := signAddVote(privValidators[6], vote, voteSet)
	if added {
		t.Errorf("Expected Add() to fail, wrong height.")
	}

	// Attempt to add a commit from val6 at a later round
	vote = &types.Vote{Height: height, Round: round + 1, Type: types.VoteTypeCommit, BlockHash: nil}
	added, _ = signAddVote(privValidators[6], vote, voteSet)
	if added {
		t.Errorf("Expected Add() to fail, cannot add future round vote.")
	}

	// Attempt to add a commit from val6 for currrent height/round.
	vote = &types.Vote{Height: height, Round: round, Type: types.VoteTypeCommit, BlockHash: nil}
	added, err := signAddVote(privValidators[6], vote, voteSet)
	if added || err == nil {
		t.Errorf("Expected Add() to fail, only prior round commits can be added.")
	}

	// Add commit from val6 at a previous round
	vote = &types.Vote{Height: height, Round: round - 1, Type: types.VoteTypeCommit, BlockHash: nil}
	added, err = signAddVote(privValidators[6], vote, voteSet)
	if !added || err != nil {
		t.Errorf("Expected Add() to succeed, commit for prior rounds are relevant.")
	}

	// Also add commit from val7 for previous round.
	vote = &types.Vote{Height: height, Round: round - 2, Type: types.VoteTypeCommit, BlockHash: nil}
	added, err = signAddVote(privValidators[7], vote, voteSet)
	if !added || err != nil {
		t.Errorf("Expected Add() to succeed. err: %v", err)
	}

	// We should have 2/3 majority
	hash, header, ok = voteSet.TwoThirdsMajority()
	if hash != nil || !header.IsZero() || !ok {
		t.Errorf("There should be 2/3 majority for nil")
	}

}

func TestMakeValidation(t *testing.T) {
	height, round := uint(1), uint(0)
	voteSet, _, privValidators := randVoteSet(height, round, types.VoteTypeCommit, 10, 1)
	blockHash, blockParts := CRandBytes(32), types.PartSetHeader{123, CRandBytes(32)}

	vote := &types.Vote{Height: height, Round: round, Type: types.VoteTypeCommit,
		BlockHash: blockHash, BlockParts: blockParts}

	// 6 out of 10 voted for some block.
	for i := 0; i < 6; i++ {
		signAddVote(privValidators[i], vote, voteSet)
	}

	// MakeValidation should fail.
	AssertPanics(t, "Doesn't have +2/3 majority", func() { voteSet.MakeValidation() })

	// 7th voted for some other block.
	{
		vote := withBlockHash(vote, RandBytes(32))
		vote = withBlockParts(vote, types.PartSetHeader{123, RandBytes(32)})
		signAddVote(privValidators[6], vote, voteSet)
	}

	// The 8th voted like everyone else.
	{
		signAddVote(privValidators[7], vote, voteSet)
	}

	validation := voteSet.MakeValidation()

	// Validation should have 10 elements
	if len(validation.Commits) != 10 {
		t.Errorf("Validation Commits should have the same number of commits as validators")
	}

	// Ensure that Validation commits are ordered.
	if err := validation.ValidateBasic(); err != nil {
		t.Errorf("Error in Validation.ValidateBasic(): %v", err)
	}

}
