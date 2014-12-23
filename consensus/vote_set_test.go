package consensus

import (
	"bytes"

	. "github.com/tendermint/tendermint/block"
	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/common/test"

	"testing"
)

// NOTE: see consensus/test.go for common test methods.

func TestAddVote(t *testing.T) {
	height, round := uint(1), uint(0)
	voteSet, _, privValidators := randVoteSet(height, round, VoteTypePrevote, 10, 1)
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

	vote := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: nil}
	vote.Signature = val0.SignVoteUnsafe(vote)
	voteSet.Add(val0.Address, vote)

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
	voteSet, _, privValidators := randVoteSet(height, round, VoteTypePrevote, 10, 1)

	// 6 out of 10 voted for nil.
	voteProto := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: nil}
	for i := 0; i < 6; i++ {
		vote := voteProto.Copy()
		vote.Signature = privValidators[i].SignVoteUnsafe(vote)
		voteSet.Add(privValidators[i].Address, vote)
	}
	hash, header, ok := voteSet.TwoThirdsMajority()
	if hash != nil || !header.IsZero() || ok {
		t.Errorf("There should be no 2/3 majority")
	}

	// 7th validator voted for some blockhash
	{
		vote := voteProto.Copy()
		vote.BlockHash = CRandBytes(32)
		vote.Signature = privValidators[6].SignVoteUnsafe(vote)
		voteSet.Add(privValidators[6].Address, vote)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if hash != nil || !header.IsZero() || ok {
			t.Errorf("There should be no 2/3 majority")
		}
	}

	// 8th validator voted for nil.
	{
		vote := voteProto.Copy()
		vote.BlockHash = nil
		vote.Signature = privValidators[7].SignVoteUnsafe(vote)
		voteSet.Add(privValidators[7].Address, vote)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if hash != nil || !header.IsZero() || !ok {
			t.Errorf("There should be 2/3 majority for nil")
		}
	}
}

func Test2_3MajorityRedux(t *testing.T) {
	height, round := uint(1), uint(0)
	voteSet, _, privValidators := randVoteSet(height, round, VoteTypePrevote, 100, 1)

	blockHash := CRandBytes(32)
	blockPartsTotal := uint(123)
	blockParts := PartSetHeader{blockPartsTotal, CRandBytes(32)}

	// 66 out of 100 voted for nil.
	voteProto := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: blockHash, BlockParts: blockParts}
	for i := 0; i < 66; i++ {
		vote := voteProto.Copy()
		vote.Signature = privValidators[i].SignVoteUnsafe(vote)
		voteSet.Add(privValidators[i].Address, vote)
	}
	hash, header, ok := voteSet.TwoThirdsMajority()
	if hash != nil || !header.IsZero() || ok {
		t.Errorf("There should be no 2/3 majority")
	}

	// 67th validator voted for nil
	{
		vote := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: nil, BlockParts: PartSetHeader{}}
		vote.Signature = privValidators[66].SignVoteUnsafe(vote)
		voteSet.Add(privValidators[66].Address, vote)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if hash != nil || !header.IsZero() || ok {
			t.Errorf("There should be no 2/3 majority: last vote added was nil")
		}
	}

	// 68th validator voted for a different BlockParts PartSetHeader
	{
		vote := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: blockHash, BlockParts: PartSetHeader{blockPartsTotal, CRandBytes(32)}}
		vote.Signature = privValidators[67].SignVoteUnsafe(vote)
		voteSet.Add(privValidators[67].Address, vote)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if hash != nil || !header.IsZero() || ok {
			t.Errorf("There should be no 2/3 majority: last vote added had different PartSetHeader Hash")
		}
	}

	// 69th validator voted for different BlockParts Total
	{
		vote := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: blockHash, BlockParts: PartSetHeader{blockPartsTotal + 1, blockParts.Hash}}
		vote.Signature = privValidators[68].SignVoteUnsafe(vote)
		voteSet.Add(privValidators[68].Address, vote)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if hash != nil || !header.IsZero() || ok {
			t.Errorf("There should be no 2/3 majority: last vote added had different PartSetHeader Total")
		}
	}

	// 70th validator voted for different BlockHash
	{
		vote := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: CRandBytes(32), BlockParts: blockParts}
		vote.Signature = privValidators[69].SignVoteUnsafe(vote)
		voteSet.Add(privValidators[69].Address, vote)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if hash != nil || !header.IsZero() || ok {
			t.Errorf("There should be no 2/3 majority: last vote added had different BlockHash")
		}
	}

	// 71st validator voted for the right BlockHash & BlockParts
	{
		vote := voteProto.Copy()
		vote.Signature = privValidators[70].SignVoteUnsafe(vote)
		voteSet.Add(privValidators[70].Address, vote)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if !bytes.Equal(hash, blockHash) || !header.Equals(blockParts) || !ok {
			t.Errorf("There should be 2/3 majority")
		}
	}
}

func TestBadVotes(t *testing.T) {
	height, round := uint(1), uint(0)
	voteSet, _, privValidators := randVoteSet(height, round, VoteTypePrevote, 10, 1)

	// val0 votes for nil.
	vote := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: nil}
	vote.Signature = privValidators[0].SignVoteUnsafe(vote)
	added, _, err := voteSet.Add(privValidators[0].Address, vote)
	if !added || err != nil {
		t.Errorf("Expected Add() to succeed")
	}

	// val0 votes again for some block.
	vote = &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: CRandBytes(32)}
	vote.Signature = privValidators[0].SignVoteUnsafe(vote)
	added, _, err = voteSet.Add(privValidators[0].Address, vote)
	if added || err == nil {
		t.Errorf("Expected Add() to fail, dupeout.")
	}

	// val1 votes on another height
	vote = &Vote{Height: height + 1, Round: round, Type: VoteTypePrevote, BlockHash: nil}
	vote.Signature = privValidators[1].SignVoteUnsafe(vote)
	added, _, err = voteSet.Add(privValidators[1].Address, vote)
	if added {
		t.Errorf("Expected Add() to fail, wrong height")
	}

	// val2 votes on another round
	vote = &Vote{Height: height, Round: round + 1, Type: VoteTypePrevote, BlockHash: nil}
	vote.Signature = privValidators[2].SignVoteUnsafe(vote)
	added, _, err = voteSet.Add(privValidators[2].Address, vote)
	if added {
		t.Errorf("Expected Add() to fail, wrong round")
	}

	// val3 votes of another type.
	vote = &Vote{Height: height, Round: round, Type: VoteTypePrecommit, BlockHash: nil}
	vote.Signature = privValidators[3].SignVoteUnsafe(vote)
	added, _, err = voteSet.Add(privValidators[3].Address, vote)
	if added {
		t.Errorf("Expected Add() to fail, wrong type")
	}
}

func TestAddCommitsToPrevoteVotes(t *testing.T) {
	height, round := uint(2), uint(5)
	voteSet, _, privValidators := randVoteSet(height, round, VoteTypePrevote, 10, 1)

	// val0, val1, val2, val3, val4, val5 vote for nil.
	voteProto := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: nil}
	for i := 0; i < 6; i++ {
		vote := voteProto.Copy()
		vote.Signature = privValidators[i].SignVoteUnsafe(vote)
		voteSet.Add(privValidators[i].Address, vote)
	}
	hash, header, ok := voteSet.TwoThirdsMajority()
	if hash != nil || !header.IsZero() || ok {
		t.Errorf("There should be no 2/3 majority")
	}

	// Attempt to add a commit from val6 at a previous height
	vote := &Vote{Height: height - 1, Round: round, Type: VoteTypeCommit, BlockHash: nil}
	vote.Signature = privValidators[6].SignVoteUnsafe(vote)
	added, _, _ := voteSet.Add(privValidators[6].Address, vote)
	if added {
		t.Errorf("Expected Add() to fail, wrong height.")
	}

	// Attempt to add a commit from val6 at a later round
	vote = &Vote{Height: height, Round: round + 1, Type: VoteTypeCommit, BlockHash: nil}
	vote.Signature = privValidators[6].SignVoteUnsafe(vote)
	added, _, _ = voteSet.Add(privValidators[6].Address, vote)
	if added {
		t.Errorf("Expected Add() to fail, cannot add future round vote.")
	}

	// Attempt to add a commit from val6 for currrent height/round.
	vote = &Vote{Height: height, Round: round, Type: VoteTypeCommit, BlockHash: nil}
	vote.Signature = privValidators[6].SignVoteUnsafe(vote)
	added, _, err := voteSet.Add(privValidators[6].Address, vote)
	if added || err == nil {
		t.Errorf("Expected Add() to fail, only prior round commits can be added.")
	}

	// Add commit from val6 at a previous round
	vote = &Vote{Height: height, Round: round - 1, Type: VoteTypeCommit, BlockHash: nil}
	vote.Signature = privValidators[6].SignVoteUnsafe(vote)
	added, _, err = voteSet.Add(privValidators[6].Address, vote)
	if !added || err != nil {
		t.Errorf("Expected Add() to succeed, commit for prior rounds are relevant.")
	}

	// Also add commit from val7 for previous round.
	vote = &Vote{Height: height, Round: round - 2, Type: VoteTypeCommit, BlockHash: nil}
	vote.Signature = privValidators[7].SignVoteUnsafe(vote)
	added, _, err = voteSet.Add(privValidators[7].Address, vote)
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
	voteSet, _, privValidators := randVoteSet(height, round, VoteTypeCommit, 10, 1)
	blockHash, blockParts := CRandBytes(32), PartSetHeader{123, CRandBytes(32)}

	// 6 out of 10 voted for some block.
	voteProto := &Vote{Height: height, Round: round, Type: VoteTypeCommit,
		BlockHash: blockHash, BlockParts: blockParts}
	for i := 0; i < 6; i++ {
		vote := voteProto.Copy()
		vote.Signature = privValidators[i].SignVoteUnsafe(vote)
		voteSet.Add(privValidators[i].Address, vote)
	}

	// MakeValidation should fail.
	AssertPanics(t, "Doesn't have +2/3 majority", func() { voteSet.MakeValidation() })

	// 7th voted for some other block.
	{
		vote := &Vote{Height: height, Round: round, Type: VoteTypeCommit,
			BlockHash:  CRandBytes(32),
			BlockParts: PartSetHeader{123, CRandBytes(32)}}
		vote.Signature = privValidators[6].SignVoteUnsafe(vote)
		voteSet.Add(privValidators[6].Address, vote)
	}

	// The 8th voted like everyone else.
	{
		vote := voteProto.Copy()
		vote.Signature = privValidators[7].SignVoteUnsafe(vote)
		voteSet.Add(privValidators[7].Address, vote)
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
