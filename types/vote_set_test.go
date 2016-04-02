package types

import (
	"bytes"

	. "github.com/tendermint/go-common"
	. "github.com/tendermint/go-common/test"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/config/tendermint_test"

	"testing"
)

func init() {
	tendermint_test.ResetConfig("types_vote_set_test")
}

// Move it out?
func randVoteSet(height int, round int, type_ byte, numValidators int, votingPower int64) (*VoteSet, *ValidatorSet, []*PrivValidator) {
	valSet, privValidators := RandValidatorSet(numValidators, votingPower)
	return NewVoteSet(height, round, type_, valSet), valSet, privValidators
}

// Convenience: Return new vote with different height
func withHeight(vote *Vote, height int) *Vote {
	vote = vote.Copy()
	vote.Height = height
	return vote
}

// Convenience: Return new vote with different round
func withRound(vote *Vote, round int) *Vote {
	vote = vote.Copy()
	vote.Round = round
	return vote
}

// Convenience: Return new vote with different type
func withType(vote *Vote, type_ byte) *Vote {
	vote = vote.Copy()
	vote.Type = type_
	return vote
}

// Convenience: Return new vote with different blockHash
func withBlockHash(vote *Vote, blockHash []byte) *Vote {
	vote = vote.Copy()
	vote.BlockHash = blockHash
	return vote
}

// Convenience: Return new vote with different blockParts
func withBlockPartsHeader(vote *Vote, blockPartsHeader PartSetHeader) *Vote {
	vote = vote.Copy()
	vote.BlockPartsHeader = blockPartsHeader
	return vote
}

func signAddVote(privVal *PrivValidator, vote *Vote, voteSet *VoteSet) (bool, error) {
	vote.Signature = privVal.Sign(SignBytes(config.GetString("chain_id"), vote)).(crypto.SignatureEd25519)
	added, _, err := voteSet.AddByAddress(privVal.Address, vote)
	return added, err
}

func TestAddVote(t *testing.T) {
	height, round := 1, 0
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
	height, round := 1, 0
	voteSet, _, privValidators := randVoteSet(height, round, VoteTypePrevote, 10, 1)

	vote := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: nil}

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
	height, round := 1, 0
	voteSet, _, privValidators := randVoteSet(height, round, VoteTypePrevote, 100, 1)

	blockHash := crypto.CRandBytes(32)
	blockPartsTotal := 123
	blockPartsHeader := PartSetHeader{blockPartsTotal, crypto.CRandBytes(32)}

	vote := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: blockHash, BlockPartsHeader: blockPartsHeader}

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
		blockPartsHeader := PartSetHeader{blockPartsTotal, crypto.CRandBytes(32)}
		signAddVote(privValidators[67], withBlockPartsHeader(vote, blockPartsHeader), voteSet)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if hash != nil || !header.IsZero() || ok {
			t.Errorf("There should be no 2/3 majority: last vote added had different PartSetHeader Hash")
		}
	}

	// 69th validator voted for different BlockParts Total
	{
		blockPartsHeader := PartSetHeader{blockPartsTotal + 1, blockPartsHeader.Hash}
		signAddVote(privValidators[68], withBlockPartsHeader(vote, blockPartsHeader), voteSet)
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

	// 71st validator voted for the right BlockHash & BlockPartsHeader
	{
		signAddVote(privValidators[70], vote, voteSet)
		hash, header, ok = voteSet.TwoThirdsMajority()
		if !bytes.Equal(hash, blockHash) || !header.Equals(blockPartsHeader) || !ok {
			t.Errorf("There should be 2/3 majority")
		}
	}
}

func TestBadVotes(t *testing.T) {
	height, round := 1, 0
	voteSet, _, privValidators := randVoteSet(height, round, VoteTypePrevote, 10, 1)

	// val0 votes for nil.
	vote := &Vote{Height: height, Round: round, Type: VoteTypePrevote, BlockHash: nil}
	added, err := signAddVote(privValidators[0], vote, voteSet)
	if !added || err != nil {
		t.Errorf("Expected VoteSet.Add to succeed")
	}

	// val0 votes again for some block.
	added, err = signAddVote(privValidators[0], withBlockHash(vote, RandBytes(32)), voteSet)
	if added || err == nil {
		t.Errorf("Expected VoteSet.Add to fail, dupeout.")
	}

	// val1 votes on another height
	added, err = signAddVote(privValidators[1], withHeight(vote, height+1), voteSet)
	if added {
		t.Errorf("Expected VoteSet.Add to fail, wrong height")
	}

	// val2 votes on another round
	added, err = signAddVote(privValidators[2], withRound(vote, round+1), voteSet)
	if added {
		t.Errorf("Expected VoteSet.Add to fail, wrong round")
	}

	// val3 votes of another type.
	added, err = signAddVote(privValidators[3], withType(vote, VoteTypePrecommit), voteSet)
	if added {
		t.Errorf("Expected VoteSet.Add to fail, wrong type")
	}
}

func TestMakeCommit(t *testing.T) {
	height, round := 1, 0
	voteSet, _, privValidators := randVoteSet(height, round, VoteTypePrecommit, 10, 1)
	blockHash, blockPartsHeader := crypto.CRandBytes(32), PartSetHeader{123, crypto.CRandBytes(32)}

	vote := &Vote{Height: height, Round: round, Type: VoteTypePrecommit,
		BlockHash: blockHash, BlockPartsHeader: blockPartsHeader}

	// 6 out of 10 voted for some block.
	for i := 0; i < 6; i++ {
		signAddVote(privValidators[i], vote, voteSet)
	}

	// MakeCommit should fail.
	AssertPanics(t, "Doesn't have +2/3 majority", func() { voteSet.MakeCommit() })

	// 7th voted for some other block.
	{
		vote := withBlockHash(vote, RandBytes(32))
		vote = withBlockPartsHeader(vote, PartSetHeader{123, RandBytes(32)})
		signAddVote(privValidators[6], vote, voteSet)
	}

	// The 8th voted like everyone else.
	{
		signAddVote(privValidators[7], vote, voteSet)
	}

	commit := voteSet.MakeCommit()

	// Commit should have 10 elements
	if len(commit.Precommits) != 10 {
		t.Errorf("Commit Precommits should have the same number of precommits as validators")
	}

	// Ensure that Commit precommits are ordered.
	if err := commit.ValidateBasic(); err != nil {
		t.Errorf("Error in Commit.ValidateBasic(): %v", err)
	}

}
