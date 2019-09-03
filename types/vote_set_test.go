package types

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
	tmtime "github.com/tendermint/tendermint/types/time"
)

// NOTE: privValidators are in order
func randVoteSet(height int64, round int, type_ SignedMsgType, numValidators int, votingPower int64) (*VoteSet, *ValidatorSet, []PrivValidator) {
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
func withHeight(vote *Vote, height int64) *Vote {
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
	vote.Type = SignedMsgType(type_)
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

func TestAddVote(t *testing.T) {
	height, round := int64(1), 0
	voteSet, _, privValidators := randVoteSet(height, round, PrevoteType, 10, 1)
	val0 := privValidators[0]

	// t.Logf(">> %v", voteSet)

	val0Addr := val0.GetPubKey().Address()
	if voteSet.GetByAddress(val0Addr) != nil {
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
		ValidatorAddress: val0Addr,
		ValidatorIndex:   0, // since privValidators are in order
		Height:           height,
		Round:            round,
		Type:             PrevoteType,
		Timestamp:        tmtime.Now(),
		BlockID:          BlockID{nil, PartSetHeader{}},
	}
	_, err := signAddVote(val0, vote, voteSet)
	if err != nil {
		t.Error(err)
	}

	if voteSet.GetByAddress(val0Addr) == nil {
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
	height, round := int64(1), 0
	voteSet, _, privValidators := randVoteSet(height, round, PrevoteType, 10, 1)

	voteProto := &Vote{
		ValidatorAddress: nil, // NOTE: must fill in
		ValidatorIndex:   -1,  // NOTE: must fill in
		Height:           height,
		Round:            round,
		Type:             PrevoteType,
		Timestamp:        tmtime.Now(),
		BlockID:          BlockID{nil, PartSetHeader{}},
	}
	// 6 out of 10 voted for nil.
	for i := 0; i < 6; i++ {
		addr := privValidators[i].GetPubKey().Address()
		vote := withValidator(voteProto, addr, i)
		_, err := signAddVote(privValidators[i], vote, voteSet)
		if err != nil {
			t.Error(err)
		}
	}
	blockID, ok := voteSet.TwoThirdsMajority()
	if ok || !blockID.IsZero() {
		t.Errorf("There should be no 2/3 majority")
	}

	// 7th validator voted for some blockhash
	{
		addr := privValidators[6].GetPubKey().Address()
		vote := withValidator(voteProto, addr, 6)
		_, err := signAddVote(privValidators[6], withBlockHash(vote, cmn.RandBytes(32)), voteSet)
		if err != nil {
			t.Error(err)
		}
		blockID, ok = voteSet.TwoThirdsMajority()
		if ok || !blockID.IsZero() {
			t.Errorf("There should be no 2/3 majority")
		}
	}

	// 8th validator voted for nil.
	{
		addr := privValidators[7].GetPubKey().Address()
		vote := withValidator(voteProto, addr, 7)
		_, err := signAddVote(privValidators[7], vote, voteSet)
		if err != nil {
			t.Error(err)
		}
		blockID, ok = voteSet.TwoThirdsMajority()
		if !ok || !blockID.IsZero() {
			t.Errorf("There should be 2/3 majority for nil")
		}
	}
}

func Test2_3MajorityRedux(t *testing.T) {
	height, round := int64(1), 0
	voteSet, _, privValidators := randVoteSet(height, round, PrevoteType, 100, 1)

	blockHash := crypto.CRandBytes(32)
	blockPartsTotal := 123
	blockPartsHeader := PartSetHeader{blockPartsTotal, crypto.CRandBytes(32)}

	voteProto := &Vote{
		ValidatorAddress: nil, // NOTE: must fill in
		ValidatorIndex:   -1,  // NOTE: must fill in
		Height:           height,
		Round:            round,
		Timestamp:        tmtime.Now(),
		Type:             PrevoteType,
		BlockID:          BlockID{blockHash, blockPartsHeader},
	}

	// 66 out of 100 voted for nil.
	for i := 0; i < 66; i++ {
		addr := privValidators[i].GetPubKey().Address()
		vote := withValidator(voteProto, addr, i)
		_, err := signAddVote(privValidators[i], vote, voteSet)
		if err != nil {
			t.Error(err)
		}
	}
	blockID, ok := voteSet.TwoThirdsMajority()
	if ok || !blockID.IsZero() {
		t.Errorf("There should be no 2/3 majority")
	}

	// 67th validator voted for nil
	{
		adrr := privValidators[66].GetPubKey().Address()
		vote := withValidator(voteProto, adrr, 66)
		_, err := signAddVote(privValidators[66], withBlockHash(vote, nil), voteSet)
		if err != nil {
			t.Error(err)
		}
		blockID, ok = voteSet.TwoThirdsMajority()
		if ok || !blockID.IsZero() {
			t.Errorf("There should be no 2/3 majority: last vote added was nil")
		}
	}

	// 68th validator voted for a different BlockParts PartSetHeader
	{
		addr := privValidators[67].GetPubKey().Address()
		vote := withValidator(voteProto, addr, 67)
		blockPartsHeader := PartSetHeader{blockPartsTotal, crypto.CRandBytes(32)}
		_, err := signAddVote(privValidators[67], withBlockPartsHeader(vote, blockPartsHeader), voteSet)
		if err != nil {
			t.Error(err)
		}
		blockID, ok = voteSet.TwoThirdsMajority()
		if ok || !blockID.IsZero() {
			t.Errorf("There should be no 2/3 majority: last vote added had different PartSetHeader Hash")
		}
	}

	// 69th validator voted for different BlockParts Total
	{
		addr := privValidators[68].GetPubKey().Address()
		vote := withValidator(voteProto, addr, 68)
		blockPartsHeader := PartSetHeader{blockPartsTotal + 1, blockPartsHeader.Hash}
		_, err := signAddVote(privValidators[68], withBlockPartsHeader(vote, blockPartsHeader), voteSet)
		if err != nil {
			t.Error(err)
		}
		blockID, ok = voteSet.TwoThirdsMajority()
		if ok || !blockID.IsZero() {
			t.Errorf("There should be no 2/3 majority: last vote added had different PartSetHeader Total")
		}
	}

	// 70th validator voted for different BlockHash
	{
		addr := privValidators[69].GetPubKey().Address()
		vote := withValidator(voteProto, addr, 69)
		_, err := signAddVote(privValidators[69], withBlockHash(vote, cmn.RandBytes(32)), voteSet)
		if err != nil {
			t.Error(err)
		}
		blockID, ok = voteSet.TwoThirdsMajority()
		if ok || !blockID.IsZero() {
			t.Errorf("There should be no 2/3 majority: last vote added had different BlockHash")
		}
	}

	// 71st validator voted for the right BlockHash & BlockPartsHeader
	{
		addr := privValidators[70].GetPubKey().Address()
		vote := withValidator(voteProto, addr, 70)
		_, err := signAddVote(privValidators[70], vote, voteSet)
		if err != nil {
			t.Error(err)
		}
		blockID, ok = voteSet.TwoThirdsMajority()
		if !ok || !blockID.Equals(BlockID{blockHash, blockPartsHeader}) {
			t.Errorf("There should be 2/3 majority")
		}
	}
}

func TestBadVotes(t *testing.T) {
	height, round := int64(1), 0
	voteSet, _, privValidators := randVoteSet(height, round, PrevoteType, 10, 1)

	voteProto := &Vote{
		ValidatorAddress: nil,
		ValidatorIndex:   -1,
		Height:           height,
		Round:            round,
		Timestamp:        tmtime.Now(),
		Type:             PrevoteType,
		BlockID:          BlockID{nil, PartSetHeader{}},
	}

	// val0 votes for nil.
	{
		addr := privValidators[0].GetPubKey().Address()
		vote := withValidator(voteProto, addr, 0)
		added, err := signAddVote(privValidators[0], vote, voteSet)
		if !added || err != nil {
			t.Errorf("Expected VoteSet.Add to succeed")
		}
	}

	// val0 votes again for some block.
	{
		addr := privValidators[0].GetPubKey().Address()
		vote := withValidator(voteProto, addr, 0)
		added, err := signAddVote(privValidators[0], withBlockHash(vote, cmn.RandBytes(32)), voteSet)
		if added || err == nil {
			t.Errorf("Expected VoteSet.Add to fail, conflicting vote.")
		}
	}

	// val1 votes on another height
	{
		addr := privValidators[1].GetPubKey().Address()
		vote := withValidator(voteProto, addr, 1)
		added, err := signAddVote(privValidators[1], withHeight(vote, height+1), voteSet)
		if added || err == nil {
			t.Errorf("Expected VoteSet.Add to fail, wrong height")
		}
	}

	// val2 votes on another round
	{
		addr := privValidators[2].GetPubKey().Address()
		vote := withValidator(voteProto, addr, 2)
		added, err := signAddVote(privValidators[2], withRound(vote, round+1), voteSet)
		if added || err == nil {
			t.Errorf("Expected VoteSet.Add to fail, wrong round")
		}
	}

	// val3 votes of another type.
	{
		addr := privValidators[3].GetPubKey().Address()
		vote := withValidator(voteProto, addr, 3)
		added, err := signAddVote(privValidators[3], withType(vote, byte(PrecommitType)), voteSet)
		if added || err == nil {
			t.Errorf("Expected VoteSet.Add to fail, wrong type")
		}
	}
}

func TestConflicts(t *testing.T) {
	height, round := int64(1), 0
	voteSet, _, privValidators := randVoteSet(height, round, PrevoteType, 4, 1)
	blockHash1 := cmn.RandBytes(32)
	blockHash2 := cmn.RandBytes(32)

	voteProto := &Vote{
		ValidatorAddress: nil,
		ValidatorIndex:   -1,
		Height:           height,
		Round:            round,
		Timestamp:        tmtime.Now(),
		Type:             PrevoteType,
		BlockID:          BlockID{nil, PartSetHeader{}},
	}

	val0Addr := privValidators[0].GetPubKey().Address()
	// val0 votes for nil.
	{
		vote := withValidator(voteProto, val0Addr, 0)
		added, err := signAddVote(privValidators[0], vote, voteSet)
		if !added || err != nil {
			t.Errorf("Expected VoteSet.Add to succeed")
		}
	}

	// val0 votes again for blockHash1.
	{
		vote := withValidator(voteProto, val0Addr, 0)
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
		vote := withValidator(voteProto, val0Addr, 0)
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
		vote := withValidator(voteProto, val0Addr, 0)
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
		addr := privValidators[1].GetPubKey().Address()
		vote := withValidator(voteProto, addr, 1)
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
		addr := privValidators[2].GetPubKey().Address()
		vote := withValidator(voteProto, addr, 2)
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
		addr := privValidators[2].GetPubKey().Address()
		vote := withValidator(voteProto, addr, 2)
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
	height, round := int64(1), 0
	voteSet, _, privValidators := randVoteSet(height, round, PrecommitType, 10, 1)
	blockHash, blockPartsHeader := crypto.CRandBytes(32), PartSetHeader{123, crypto.CRandBytes(32)}

	voteProto := &Vote{
		ValidatorAddress: nil,
		ValidatorIndex:   -1,
		Height:           height,
		Round:            round,
		Timestamp:        tmtime.Now(),
		Type:             PrecommitType,
		BlockID:          BlockID{blockHash, blockPartsHeader},
	}

	// 6 out of 10 voted for some block.
	for i := 0; i < 6; i++ {
		addr := privValidators[i].GetPubKey().Address()
		vote := withValidator(voteProto, addr, i)
		_, err := signAddVote(privValidators[i], vote, voteSet)
		if err != nil {
			t.Error(err)
		}
	}

	// MakeCommit should fail.
	assert.Panics(t, func() { voteSet.MakeCommit() }, "Doesn't have +2/3 majority")

	// 7th voted for some other block.
	{
		addr := privValidators[6].GetPubKey().Address()
		vote := withValidator(voteProto, addr, 6)
		vote = withBlockHash(vote, cmn.RandBytes(32))
		vote = withBlockPartsHeader(vote, PartSetHeader{123, cmn.RandBytes(32)})

		_, err := signAddVote(privValidators[6], vote, voteSet)
		if err != nil {
			t.Error(err)
		}
	}

	// The 8th voted like everyone else.
	{
		addr := privValidators[7].GetPubKey().Address()
		vote := withValidator(voteProto, addr, 7)
		_, err := signAddVote(privValidators[7], vote, voteSet)
		if err != nil {
			t.Error(err)
		}
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
