package consensus

import (
	"bytes"

	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/common/test"

	"testing"
)

// NOTE: see consensus/test.go for common test methods.

func TestAddVote(t *testing.T) {
	height, round := uint32(1), uint16(0)
	voteSet, _, privAccounts := makeVoteSet(height, round, VoteTypePrevote, 10, 1)

	// t.Logf(">> %v", voteSet)

	if voteSet.GetById(0) != nil {
		t.Errorf("Expected GetById(0) to be nil")
	}
	if voteSet.BitArray().GetIndex(0) {
		t.Errorf("Expected BitArray.GetIndex(0) to be false")
	}
	hash, header, ok := voteSet.TwoThirdsMajority()
	if hash != nil || !header.IsZero() || ok {
		t.Errorf("There should be no 2/3 majority")
	}

	vote := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: nil}
	privAccounts[0].Sign(vote)
	voteSet.Add(vote)

	if voteSet.GetById(0) == nil {
		t.Errorf("Expected GetById(0) to be present")
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
	height, round := uint32(1), uint16(0)
	voteSet, _, privAccounts := makeVoteSet(height, round, VoteTypePrevote, 10, 1)

	// 6 out of 10 voted for nil.
	voteProto := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: nil}
	for i := 0; i < 6; i++ {
		vote := voteProto.Copy()
		privAccounts[i].Sign(vote)
		voteSet.Add(vote)
	}
	hash, header, ok := voteSet.TwoThirdsMajority()
	if hash != nil || !header.IsZero() || ok {
		t.Errorf("There should be no 2/3 majority")
	}

	// 7th validator voted for some blockhash
	{
		vote := voteProto.Copy()
		vote.BlockHash = CRandBytes(32)
		privAccounts[6].Sign(vote)
		voteSet.Add(vote)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if hash != nil || !header.IsZero() || ok {
			t.Errorf("There should be no 2/3 majority")
		}
	}

	// 8th validator voted for nil.
	{
		vote := voteProto.Copy()
		vote.BlockHash = nil
		privAccounts[7].Sign(vote)
		voteSet.Add(vote)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if hash != nil || !header.IsZero() || !ok {
			t.Errorf("There should be 2/3 majority for nil")
		}
	}
}

func Test2_3MajorityRedux(t *testing.T) {
	height, round := uint32(1), uint16(0)
	voteSet, _, privAccounts := makeVoteSet(height, round, VoteTypePrevote, 100, 1)

	blockHash := CRandBytes(32)
	blockPartsTotal := uint16(123)
	blockParts := PartSetHeader{blockPartsTotal, CRandBytes(32)}

	// 66 out of 100 voted for nil.
	voteProto := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: blockHash, BlockParts: blockParts}
	for i := 0; i < 66; i++ {
		vote := voteProto.Copy()
		privAccounts[i].Sign(vote)
		voteSet.Add(vote)
	}
	hash, header, ok := voteSet.TwoThirdsMajority()
	if hash != nil || !header.IsZero() || ok {
		t.Errorf("There should be no 2/3 majority")
	}

	// 67th validator voted for nil
	{
		vote := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: nil, BlockParts: PartSetHeader{}}
		privAccounts[66].Sign(vote)
		voteSet.Add(vote)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if hash != nil || !header.IsZero() || ok {
			t.Errorf("There should be no 2/3 majority: last vote added was nil")
		}
	}

	// 68th validator voted for a different BlockParts PartSetHeader
	{
		vote := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: blockHash, BlockParts: PartSetHeader{blockPartsTotal, CRandBytes(32)}}
		privAccounts[67].Sign(vote)
		voteSet.Add(vote)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if hash != nil || !header.IsZero() || ok {
			t.Errorf("There should be no 2/3 majority: last vote added had different PartSetHeader Hash")
		}
	}

	// 69th validator voted for different BlockParts Total
	{
		vote := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: blockHash, BlockParts: PartSetHeader{blockPartsTotal + 1, blockParts.Hash}}
		privAccounts[68].Sign(vote)
		voteSet.Add(vote)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if hash != nil || !header.IsZero() || ok {
			t.Errorf("There should be no 2/3 majority: last vote added had different PartSetHeader Total")
		}
	}

	// 70th validator voted for different BlockHash
	{
		vote := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: CRandBytes(32), BlockParts: blockParts}
		privAccounts[69].Sign(vote)
		voteSet.Add(vote)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if hash != nil || !header.IsZero() || ok {
			t.Errorf("There should be no 2/3 majority: last vote added had different BlockHash")
		}
	}

	// 71st validator voted for the right BlockHash & BlockParts
	{
		vote := voteProto.Copy()
		privAccounts[70].Sign(vote)
		voteSet.Add(vote)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if !bytes.Equal(hash, blockHash) || !header.Equals(blockParts) || !ok {
			t.Errorf("There should be 2/3 majority")
		}
	}
}

func TestBadVotes(t *testing.T) {
	height, round := uint32(1), uint16(0)
	voteSet, _, privAccounts := makeVoteSet(height, round, VoteTypePrevote, 10, 1)

	// val0 votes for nil.
	vote := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: nil}
	privAccounts[0].Sign(vote)
	added, err := voteSet.Add(vote)
	if !added || err != nil {
		t.Errorf("Expected Add(vote) to succeed")
	}

	// val0 votes again for some block.
	vote = &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: CRandBytes(32)}
	privAccounts[0].Sign(vote)
	added, err = voteSet.Add(vote)
	if added || err == nil {
		t.Errorf("Expected Add(vote) to fail, dupeout.")
	}

	// val1 votes on another height
	vote = &Vote{Height: height + 1, Round: round, Type: VoteTypePrevote, BlockHash: nil}
	privAccounts[1].Sign(vote)
	added, err = voteSet.Add(vote)
	if added {
		t.Errorf("Expected Add(vote) to fail, wrong height")
	}

	// val2 votes on another round
	vote = &Vote{Height: height, Round: round + 1, Type: VoteTypePrevote, BlockHash: nil}
	privAccounts[2].Sign(vote)
	added, err = voteSet.Add(vote)
	if added {
		t.Errorf("Expected Add(vote) to fail, wrong round")
	}

	// val3 votes of another type.
	vote = &Vote{Height: height, Round: round, Type: VoteTypePrecommit, BlockHash: nil}
	privAccounts[3].Sign(vote)
	added, err = voteSet.Add(vote)
	if added {
		t.Errorf("Expected Add(vote) to fail, wrong type")
	}
}

func TestAddCommitsToPrevoteVotes(t *testing.T) {
	height, round := uint32(2), uint16(5)
	voteSet, _, privAccounts := makeVoteSet(height, round, VoteTypePrevote, 10, 1)

	// val0, val1, val2, val3, val4, val5 vote for nil.
	voteProto := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: nil}
	for i := 0; i < 6; i++ {
		vote := voteProto.Copy()
		privAccounts[i].Sign(vote)
		voteSet.Add(vote)
	}
	hash, header, ok := voteSet.TwoThirdsMajority()
	if hash != nil || !header.IsZero() || ok {
		t.Errorf("There should be no 2/3 majority")
	}

	// Attempt to add a commit from val6 at a previous height
	vote := &Vote{Height: height - 1, Round: round, Type: VoteTypeCommit, BlockHash: nil}
	privAccounts[6].Sign(vote)
	added, _ := voteSet.Add(vote)
	if added {
		t.Errorf("Expected Add(vote) to fail, wrong height.")
	}

	// Attempt to add a commit from val6 at a later round
	vote = &Vote{Height: height, Round: round + 1, Type: VoteTypeCommit, BlockHash: nil}
	privAccounts[6].Sign(vote)
	added, _ = voteSet.Add(vote)
	if added {
		t.Errorf("Expected Add(vote) to fail, cannot add future round vote.")
	}

	// Attempt to add a commit from val6 for currrent height/round.
	vote = &Vote{Height: height, Round: round, Type: VoteTypeCommit, BlockHash: nil}
	privAccounts[6].Sign(vote)
	added, err := voteSet.Add(vote)
	if added || err == nil {
		t.Errorf("Expected Add(vote) to fail, only prior round commits can be added.")
	}

	// Add commit from val6 at a previous round
	vote = &Vote{Height: height, Round: round - 1, Type: VoteTypeCommit, BlockHash: nil}
	privAccounts[6].Sign(vote)
	added, err = voteSet.Add(vote)
	if !added || err != nil {
		t.Errorf("Expected Add(vote) to succeed, commit for prior rounds are relevant.")
	}

	// Also add commit from val7 for previous round.
	vote = &Vote{Height: height, Round: round - 2, Type: VoteTypeCommit, BlockHash: nil}
	privAccounts[7].Sign(vote)
	added, err = voteSet.Add(vote)
	if !added || err != nil {
		t.Errorf("Expected Add(vote) to succeed. err: %v", err)
	}

	// We should have 2/3 majority
	hash, header, ok = voteSet.TwoThirdsMajority()
	if hash != nil || !header.IsZero() || !ok {
		t.Errorf("There should be 2/3 majority for nil")
	}

}

func TestMakeValidation(t *testing.T) {
	height, round := uint32(1), uint16(0)
	voteSet, _, privAccounts := makeVoteSet(height, round, VoteTypeCommit, 10, 1)
	blockHash, blockParts := CRandBytes(32), PartSetHeader{123, CRandBytes(32)}

	// 6 out of 10 voted for some block.
	voteProto := &Vote{Height: height, Round: round, Type: VoteTypeCommit,
		BlockHash: blockHash, BlockParts: blockParts}
	for i := 0; i < 6; i++ {
		vote := voteProto.Copy()
		privAccounts[i].Sign(vote)
		voteSet.Add(vote)
	}

	// MakeValidation should fail.
	AssertPanics(t, "Doesn't have +2/3 majority", func() { voteSet.MakeValidation() })

	// 7th voted for some other block.
	{
		vote := &Vote{Height: height, Round: round, Type: VoteTypeCommit,
			BlockHash:  CRandBytes(32),
			BlockParts: PartSetHeader{123, CRandBytes(32)}}
		privAccounts[6].Sign(vote)
		voteSet.Add(vote)
	}

	// The 8th voted like everyone else.
	{
		vote := voteProto.Copy()
		privAccounts[7].Sign(vote)
		voteSet.Add(vote)
	}

	validation := voteSet.MakeValidation()

	// Validation should have 10 elements
	if len(validation.Commits) != 10 {
		t.Errorf("Validation Commits should have the same number of commits as validators")
	}

	// Ensure that Validation commits are ordered.
	for i, rsig := range validation.Commits {
		if i < 6 || i == 7 {
			if rsig.Round != round {
				t.Errorf("Expected round %v but got %v", round, rsig.Round)
			}
			if rsig.SignerId != uint64(i) {
				t.Errorf("Validation commit signer out of order. Expected %v, got %v", i, rsig.Signature)
			}
			vote := &Vote{Height: height, Round: rsig.Round, Type: VoteTypeCommit,
				BlockHash: blockHash, BlockParts: blockParts,
				Signature: rsig.Signature}
			if !privAccounts[i].Verify(vote) {
				t.Errorf("Validation commit did not verify")
			}
		} else {
			if !rsig.IsZero() {
				t.Errorf("Expected zero RoundSignature for the rest")
			}
		}
	}
}
