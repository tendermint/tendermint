package consensus

import (
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"

	"testing"
)

// NOTE: see consensus/test.go for common test methods.

func TestAddVote(t *testing.T) {
	voteSet, _, privAccounts := makeVoteSet(0, 0, 10, 1)

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

	vote := &Vote{Height: 0, Round: 0, Type: VoteTypePrevote, BlockHash: nil}
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
	voteSet, _, privAccounts := makeVoteSet(0, 0, 10, 1)

	// 6 out of 10 voted for nil.
	vote := &Vote{Height: 0, Round: 0, Type: VoteTypePrevote, BlockHash: nil}
	for i := 0; i < 6; i++ {
		privAccounts[i].Sign(vote)
		voteSet.Add(vote)
	}
	hash, header, ok := voteSet.TwoThirdsMajority()
	if hash != nil || !header.IsZero() || ok {
		t.Errorf("There should be no 2/3 majority")
	}

	// 7th validator voted for some blockhash
	vote.BlockHash = CRandBytes(32)
	privAccounts[6].Sign(vote)
	voteSet.Add(vote)
	hash, header, ok = voteSet.TwoThirdsMajority()
	if hash != nil || !header.IsZero() || ok {
		t.Errorf("There should be no 2/3 majority")
	}

	// 8th validator voted for nil.
	vote.BlockHash = nil
	privAccounts[7].Sign(vote)
	voteSet.Add(vote)
	hash, header, ok = voteSet.TwoThirdsMajority()
	if hash != nil || !header.IsZero() || !ok {
		t.Errorf("There should be 2/3 majority for nil")
	}

}

func TestBadVotes(t *testing.T) {
	voteSet, _, privAccounts := makeVoteSet(1, 0, 10, 1)

	// val0 votes for nil.
	vote := &Vote{Height: 1, Round: 0, Type: VoteTypePrevote, BlockHash: nil}
	privAccounts[0].Sign(vote)
	added, err := voteSet.Add(vote)
	if !added || err != nil {
		t.Errorf("Expected Add(vote) to succeed")
	}

	// val0 votes again for some block.
	vote = &Vote{Height: 1, Round: 0, Type: VoteTypePrevote, BlockHash: CRandBytes(32)}
	privAccounts[0].Sign(vote)
	added, err = voteSet.Add(vote)
	if added || err == nil {
		t.Errorf("Expected Add(vote) to fail, dupeout.")
	}

	// val1 votes on another height
	vote = &Vote{Height: 0, Round: 0, Type: VoteTypePrevote, BlockHash: nil}
	privAccounts[1].Sign(vote)
	added, err = voteSet.Add(vote)
	if added {
		t.Errorf("Expected Add(vote) to fail, wrong height")
	}

	// val2 votes on another round
	vote = &Vote{Height: 1, Round: 1, Type: VoteTypePrevote, BlockHash: nil}
	privAccounts[2].Sign(vote)
	added, err = voteSet.Add(vote)
	if added {
		t.Errorf("Expected Add(vote) to fail, wrong round")
	}

	// val3 votes of another type.
	vote = &Vote{Height: 1, Round: 0, Type: VoteTypePrecommit, BlockHash: nil}
	privAccounts[3].Sign(vote)
	added, err = voteSet.Add(vote)
	if added {
		t.Errorf("Expected Add(vote) to fail, wrong type")
	}
}

func TestAddCommitsToPrevoteVotes(t *testing.T) {
	voteSet, _, privAccounts := makeVoteSet(1, 5, 10, 1)

	// val0, val1, val2, val3, val4, val5 vote for nil.
	vote := &Vote{Height: 1, Round: 5, Type: VoteTypePrevote, BlockHash: nil}
	for i := 0; i < 6; i++ {
		privAccounts[i].Sign(vote)
		voteSet.Add(vote)
	}
	hash, header, ok := voteSet.TwoThirdsMajority()
	if hash != nil || !header.IsZero() || ok {
		t.Errorf("There should be no 2/3 majority")
	}

	// Attempt to add a commit from val6 at a previous height
	vote = &Vote{Height: 0, Round: 5, Type: VoteTypeCommit, BlockHash: nil}
	privAccounts[6].Sign(vote)
	added, _ := voteSet.Add(vote)
	if added {
		t.Errorf("Expected Add(vote) to fail, wrong height.")
	}

	// Attempt to add a commit from val6 at a later round
	vote = &Vote{Height: 1, Round: 6, Type: VoteTypeCommit, BlockHash: nil}
	privAccounts[6].Sign(vote)
	added, _ = voteSet.Add(vote)
	if added {
		t.Errorf("Expected Add(vote) to fail, cannot add future round vote.")
	}

	// Attempt to add a commit from val6 for currrent height/round.
	vote = &Vote{Height: 1, Round: 5, Type: VoteTypeCommit, BlockHash: nil}
	privAccounts[6].Sign(vote)
	added, err := voteSet.Add(vote)
	if added || err == nil {
		t.Errorf("Expected Add(vote) to fail, only prior round commits can be added.")
	}

	// Add commit from val6 at a previous round
	vote = &Vote{Height: 1, Round: 4, Type: VoteTypeCommit, BlockHash: nil}
	privAccounts[6].Sign(vote)
	added, err = voteSet.Add(vote)
	if !added || err != nil {
		t.Errorf("Expected Add(vote) to succeed, commit for prior rounds are relevant.")
	}

	// Also add commit from val7 for previous round.
	vote = &Vote{Height: 1, Round: 3, Type: VoteTypeCommit, BlockHash: nil}
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
