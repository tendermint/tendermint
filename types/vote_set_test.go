package types

import (
	"bytes"

	. "github.com/tendermint/go-common"
	. "github.com/tendermint/go-common/test"
	"github.com/tendermint/go-crypto"

	"testing"
)

// NOTE: privValidators are in order
// TODO: Move it out?
func randVoteSet(height int, round int, type_ byte, numValidators int, votingPower int64) (*VoteSet, *ValidatorSet, []*PrivValidator) {
	valSet, privValidators := RandValidatorSet(numValidators, votingPower)
	return NewVoteSet("test_chain_id", height, round, type_, valSet), valSet, privValidators
}

// Convenience: Return new vote with different validator address/index
func withValidator(vote *Vote, addr []byte, idx int) *Vote {
	vote = vote.Copy()
	vote.ValidatorAddress = addr
	vote.ValidatorIndex = idx
	return vote
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
	vote.BlockID.Hash = blockHash
	return vote
}

// Convenience: Return new vote with different blockParts
func withBlockPartsHeader(vote *Vote, blockPartsHeader PartSetHeader) *Vote {
	vote = vote.Copy()
	vote.BlockID.PartsHeader = blockPartsHeader
	return vote
}

func signAddVote(privVal *PrivValidator, vote *Vote, voteSet *VoteSet) (bool, error) {
	vote.Signature = privVal.Sign(SignBytes(voteSet.ChainID(), vote))
	added, err := voteSet.AddVote(vote)
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
	blockID, ok := voteSet.TwoThirdsMajority()
	if ok || !blockID.IsZero() {
		t.Errorf("There should be no 2/3 majority")
	}

	vote := &Vote{
		ValidatorAddress: val0.Address,
		ValidatorIndex:   0, // since privValidators are in order
		Height:           height,
		Round:            round,
		Type:             VoteTypePrevote,
		BlockID:          BlockID{nil, PartSetHeader{}},
	}
	_, err := signAddVote(val0, vote, voteSet)
	if err != nil {
		t.Error(err)
	}

	if voteSet.GetByAddress(val0.Address) == nil {
		t.Errorf("Expected GetByAddress(val0.Address) to be present")
	}
	if !voteSet.BitArray().GetIndex(0) {
		t.Errorf("Expected BitArray.GetIndex(0) to be true")
	}
	blockID, ok = voteSet.TwoThirdsMajority()
	if ok || !blockID.IsZero() {
		t.Errorf("There should be no 2/3 majority")
	}
}

func Test2_3Majority(t *testing.T) {
	height, round := 1, 0
	voteSet, _, privValidators := randVoteSet(height, round, VoteTypePrevote, 10, 1)

	voteProto := &Vote{
		ValidatorAddress: nil, // NOTE: must fill in
		ValidatorIndex:   -1,  // NOTE: must fill in
		Height:           height,
		Round:            round,
		Type:             VoteTypePrevote,
		BlockID:          BlockID{nil, PartSetHeader{}},
	}
	// 6 out of 10 voted for nil.
	for i := 0; i < 6; i++ {
		vote := withValidator(voteProto, privValidators[i].Address, i)
		signAddVote(privValidators[i], vote, voteSet)
	}
	blockID, ok := voteSet.TwoThirdsMajority()
	if ok || !blockID.IsZero() {
		t.Errorf("There should be no 2/3 majority")
	}

	// 7th validator voted for some blockhash
	{
		vote := withValidator(voteProto, privValidators[6].Address, 6)
		signAddVote(privValidators[6], withBlockHash(vote, RandBytes(32)), voteSet)
		blockID, ok = voteSet.TwoThirdsMajority()
		if ok || !blockID.IsZero() {
			t.Errorf("There should be no 2/3 majority")
		}
	}

	// 8th validator voted for nil.
	{
		vote := withValidator(voteProto, privValidators[7].Address, 7)
		signAddVote(privValidators[7], vote, voteSet)
		blockID, ok = voteSet.TwoThirdsMajority()
		if !ok || !blockID.IsZero() {
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

	voteProto := &Vote{
		ValidatorAddress: nil, // NOTE: must fill in
		ValidatorIndex:   -1,  // NOTE: must fill in
		Height:           height,
		Round:            round,
		Type:             VoteTypePrevote,
		BlockID:          BlockID{blockHash, blockPartsHeader},
	}

	// 66 out of 100 voted for nil.
	for i := 0; i < 66; i++ {
		vote := withValidator(voteProto, privValidators[i].Address, i)
		signAddVote(privValidators[i], vote, voteSet)
	}
	blockID, ok := voteSet.TwoThirdsMajority()
	if ok || !blockID.IsZero() {
		t.Errorf("There should be no 2/3 majority")
	}

	// 67th validator voted for nil
	{
		vote := withValidator(voteProto, privValidators[66].Address, 66)
		signAddVote(privValidators[66], withBlockHash(vote, nil), voteSet)
		blockID, ok = voteSet.TwoThirdsMajority()
		if ok || !blockID.IsZero() {
			t.Errorf("There should be no 2/3 majority: last vote added was nil")
		}
	}

	// 68th validator voted for a different BlockParts PartSetHeader
	{
		vote := withValidator(voteProto, privValidators[67].Address, 67)
		blockPartsHeader := PartSetHeader{blockPartsTotal, crypto.CRandBytes(32)}
		signAddVote(privValidators[67], withBlockPartsHeader(vote, blockPartsHeader), voteSet)
		blockID, ok = voteSet.TwoThirdsMajority()
		if ok || !blockID.IsZero() {
			t.Errorf("There should be no 2/3 majority: last vote added had different PartSetHeader Hash")
		}
	}

	// 69th validator voted for different BlockParts Total
	{
		vote := withValidator(voteProto, privValidators[68].Address, 68)
		blockPartsHeader := PartSetHeader{blockPartsTotal + 1, blockPartsHeader.Hash}
		signAddVote(privValidators[68], withBlockPartsHeader(vote, blockPartsHeader), voteSet)
		blockID, ok = voteSet.TwoThirdsMajority()
		if ok || !blockID.IsZero() {
			t.Errorf("There should be no 2/3 majority: last vote added had different PartSetHeader Total")
		}
	}

	// 70th validator voted for different BlockHash
	{
		vote := withValidator(voteProto, privValidators[69].Address, 69)
		signAddVote(privValidators[69], withBlockHash(vote, RandBytes(32)), voteSet)
		blockID, ok = voteSet.TwoThirdsMajority()
		if ok || !blockID.IsZero() {
			t.Errorf("There should be no 2/3 majority: last vote added had different BlockHash")
		}
	}

	// 71st validator voted for the right BlockHash & BlockPartsHeader
	{
		vote := withValidator(voteProto, privValidators[70].Address, 70)
		signAddVote(privValidators[70], vote, voteSet)
		blockID, ok = voteSet.TwoThirdsMajority()
		if !ok || !blockID.Equals(BlockID{blockHash, blockPartsHeader}) {
			t.Errorf("There should be 2/3 majority")
		}
	}
}

func TestBadVotes(t *testing.T) {
	height, round := 1, 0
	voteSet, _, privValidators := randVoteSet(height, round, VoteTypePrevote, 10, 1)

	voteProto := &Vote{
		ValidatorAddress: nil,
		ValidatorIndex:   -1,
		Height:           height,
		Round:            round,
		Type:             VoteTypePrevote,
		BlockID:          BlockID{nil, PartSetHeader{}},
	}

	// val0 votes for nil.
	{
		vote := withValidator(voteProto, privValidators[0].Address, 0)
		added, err := signAddVote(privValidators[0], vote, voteSet)
		if !added || err != nil {
			t.Errorf("Expected VoteSet.Add to succeed")
		}
	}

	// val0 votes again for some block.
	{
		vote := withValidator(voteProto, privValidators[0].Address, 0)
		added, err := signAddVote(privValidators[0], withBlockHash(vote, RandBytes(32)), voteSet)
		if added || err == nil {
			t.Errorf("Expected VoteSet.Add to fail, conflicting vote.")
		}
	}

	// val1 votes on another height
	{
		vote := withValidator(voteProto, privValidators[1].Address, 1)
		added, err := signAddVote(privValidators[1], withHeight(vote, height+1), voteSet)
		if added || err == nil {
			t.Errorf("Expected VoteSet.Add to fail, wrong height")
		}
	}

	// val2 votes on another round
	{
		vote := withValidator(voteProto, privValidators[2].Address, 2)
		added, err := signAddVote(privValidators[2], withRound(vote, round+1), voteSet)
		if added || err == nil {
			t.Errorf("Expected VoteSet.Add to fail, wrong round")
		}
	}

	// val3 votes of another type.
	{
		vote := withValidator(voteProto, privValidators[3].Address, 3)
		added, err := signAddVote(privValidators[3], withType(vote, VoteTypePrecommit), voteSet)
		if added || err == nil {
			t.Errorf("Expected VoteSet.Add to fail, wrong type")
		}
	}
}

func TestConflicts(t *testing.T) {
	height, round := 1, 0
	voteSet, _, privValidators := randVoteSet(height, round, VoteTypePrevote, 4, 1)
	blockHash1 := RandBytes(32)
	blockHash2 := RandBytes(32)

	voteProto := &Vote{
		ValidatorAddress: nil,
		ValidatorIndex:   -1,
		Height:           height,
		Round:            round,
		Type:             VoteTypePrevote,
		BlockID:          BlockID{nil, PartSetHeader{}},
	}

	// val0 votes for nil.
	{
		vote := withValidator(voteProto, privValidators[0].Address, 0)
		added, err := signAddVote(privValidators[0], vote, voteSet)
		if !added || err != nil {
			t.Errorf("Expected VoteSet.Add to succeed")
		}
	}

	// val0 votes again for blockHash1.
	{
		vote := withValidator(voteProto, privValidators[0].Address, 0)
		added, err := signAddVote(privValidators[0], withBlockHash(vote, blockHash1), voteSet)
		if added {
			t.Errorf("Expected VoteSet.Add to fail, conflicting vote.")
		}
		if err == nil {
			t.Errorf("Expected VoteSet.Add to return error, conflicting vote.")
		}
	}

	// start tracking blockHash1
	voteSet.SetPeerMaj23("peerA", BlockID{blockHash1, PartSetHeader{}})

	// val0 votes again for blockHash1.
	{
		vote := withValidator(voteProto, privValidators[0].Address, 0)
		added, err := signAddVote(privValidators[0], withBlockHash(vote, blockHash1), voteSet)
		if !added {
			t.Errorf("Expected VoteSet.Add to succeed, called SetPeerMaj23().")
		}
		if err == nil {
			t.Errorf("Expected VoteSet.Add to return error, conflicting vote.")
		}
	}

	// attempt tracking blockHash2, should fail because already set for peerA.
	voteSet.SetPeerMaj23("peerA", BlockID{blockHash2, PartSetHeader{}})

	// val0 votes again for blockHash1.
	{
		vote := withValidator(voteProto, privValidators[0].Address, 0)
		added, err := signAddVote(privValidators[0], withBlockHash(vote, blockHash2), voteSet)
		if added {
			t.Errorf("Expected VoteSet.Add to fail, duplicate SetPeerMaj23() from peerA")
		}
		if err == nil {
			t.Errorf("Expected VoteSet.Add to return error, conflicting vote.")
		}
	}

	// val1 votes for blockHash1.
	{
		vote := withValidator(voteProto, privValidators[1].Address, 1)
		added, err := signAddVote(privValidators[1], withBlockHash(vote, blockHash1), voteSet)
		if !added || err != nil {
			t.Errorf("Expected VoteSet.Add to succeed")
		}
	}

	// check
	if voteSet.HasTwoThirdsMajority() {
		t.Errorf("We shouldn't have 2/3 majority yet")
	}
	if voteSet.HasTwoThirdsAny() {
		t.Errorf("We shouldn't have 2/3 if any votes yet")
	}

	// val2 votes for blockHash2.
	{
		vote := withValidator(voteProto, privValidators[2].Address, 2)
		added, err := signAddVote(privValidators[2], withBlockHash(vote, blockHash2), voteSet)
		if !added || err != nil {
			t.Errorf("Expected VoteSet.Add to succeed")
		}
	}

	// check
	if voteSet.HasTwoThirdsMajority() {
		t.Errorf("We shouldn't have 2/3 majority yet")
	}
	if !voteSet.HasTwoThirdsAny() {
		t.Errorf("We should have 2/3 if any votes")
	}

	// now attempt tracking blockHash1
	voteSet.SetPeerMaj23("peerB", BlockID{blockHash1, PartSetHeader{}})

	// val2 votes for blockHash1.
	{
		vote := withValidator(voteProto, privValidators[2].Address, 2)
		added, err := signAddVote(privValidators[2], withBlockHash(vote, blockHash1), voteSet)
		if !added {
			t.Errorf("Expected VoteSet.Add to succeed")
		}
		if err == nil {
			t.Errorf("Expected VoteSet.Add to return error, conflicting vote")
		}
	}

	// check
	if !voteSet.HasTwoThirdsMajority() {
		t.Errorf("We should have 2/3 majority for blockHash1")
	}
	blockIDMaj23, _ := voteSet.TwoThirdsMajority()
	if !bytes.Equal(blockIDMaj23.Hash, blockHash1) {
		t.Errorf("Got the wrong 2/3 majority blockhash")
	}
	if !voteSet.HasTwoThirdsAny() {
		t.Errorf("We should have 2/3 if any votes")
	}

}

func TestMakeCommit(t *testing.T) {
	height, round := 1, 0
	voteSet, _, privValidators := randVoteSet(height, round, VoteTypePrecommit, 10, 1)
	blockHash, blockPartsHeader := crypto.CRandBytes(32), PartSetHeader{123, crypto.CRandBytes(32)}

	voteProto := &Vote{
		ValidatorAddress: nil,
		ValidatorIndex:   -1,
		Height:           height,
		Round:            round,
		Type:             VoteTypePrecommit,
		BlockID:          BlockID{blockHash, blockPartsHeader},
	}

	// 6 out of 10 voted for some block.
	for i := 0; i < 6; i++ {
		vote := withValidator(voteProto, privValidators[i].Address, i)
		signAddVote(privValidators[i], vote, voteSet)
	}

	// MakeCommit should fail.
	AssertPanics(t, "Doesn't have +2/3 majority", func() { voteSet.MakeCommit() })

	// 7th voted for some other block.
	{
		vote := withValidator(voteProto, privValidators[6].Address, 6)
		vote = withBlockHash(vote, RandBytes(32))
		vote = withBlockPartsHeader(vote, PartSetHeader{123, RandBytes(32)})
		signAddVote(privValidators[6], vote, voteSet)
	}

	// The 8th voted like everyone else.
	{
		vote := withValidator(voteProto, privValidators[7].Address, 7)
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
