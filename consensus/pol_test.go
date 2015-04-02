package consensus

import (
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"

	"bytes"
	"testing"
)

// NOTE: see consensus/test.go for common test methods.

// Convenience method.
// Signs the vote and sets the POL's vote at the desired index
// Returns the POLVoteSignature pointer, so you can modify it afterwards.
func signAddPOLVoteSignature(val *sm.PrivValidator, valSet *sm.ValidatorSet, vote *types.Vote, pol *POL) *POLVoteSignature {
	vote = vote.Copy()
	err := val.SignVote(vote)
	if err != nil {
		panic(err)
	}
	idx, _ := valSet.GetByAddress(val.Address) // now we have the index
	pol.Votes[idx] = POLVoteSignature{vote.Round, vote.Signature}
	return &pol.Votes[idx]
}

func TestVerifyVotes(t *testing.T) {
	height, round := uint(1), uint(0)
	_, valSet, privValidators := randVoteSet(height, round, types.VoteTypePrevote, 10, 1)

	// Make a POL with -2/3 votes.
	blockHash := RandBytes(32)
	pol := &POL{
		Height: height, Round: round, BlockHash: blockHash,
		Votes: make([]POLVoteSignature, valSet.Size()),
	}
	voteProto := &types.Vote{
		Height: height, Round: round, Type: types.VoteTypePrevote, BlockHash: blockHash,
	}
	for i := 0; i < 6; i++ {
		signAddPOLVoteSignature(privValidators[i], valSet, voteProto, pol)
	}

	// Check that validation fails.
	if err := pol.Verify(valSet); err == nil {
		t.Errorf("Expected POL.Verify() to fail, not enough votes.")
	}

	// Insert another vote to make +2/3
	signAddPOLVoteSignature(privValidators[7], valSet, voteProto, pol)

	// Check that validation succeeds.
	if err := pol.Verify(valSet); err != nil {
		t.Errorf("POL.Verify() failed: %v", err)
	}
}

func TestVerifyInvalidVote(t *testing.T) {
	height, round := uint(1), uint(0)
	_, valSet, privValidators := randVoteSet(height, round, types.VoteTypePrevote, 10, 1)

	// Make a POL with +2/3 votes with the wrong signature.
	blockHash := RandBytes(32)
	pol := &POL{
		Height: height, Round: round, BlockHash: blockHash,
		Votes: make([]POLVoteSignature, valSet.Size()),
	}
	voteProto := &types.Vote{
		Height: height, Round: round, Type: types.VoteTypePrevote, BlockHash: blockHash,
	}
	for i := 0; i < 7; i++ {
		polVoteSig := signAddPOLVoteSignature(privValidators[i], valSet, voteProto, pol)
		polVoteSig.Signature[0] += byte(0x01) // mutated!
	}

	// Check that validation fails.
	if err := pol.Verify(valSet); err == nil {
		t.Errorf("Expected POL.Verify() to fail, wrong signatures.")
	}
}

func TestVerifyCommits(t *testing.T) {
	height, round := uint(1), uint(2)
	_, valSet, privValidators := randVoteSet(height, round, types.VoteTypePrevote, 10, 1)

	// Make a POL with +2/3 votes.
	blockHash := RandBytes(32)
	pol := &POL{
		Height: height, Round: round, BlockHash: blockHash,
		Votes: make([]POLVoteSignature, valSet.Size()),
	}
	voteProto := &types.Vote{
		Height: height, Round: round - 1, Type: types.VoteTypeCommit, BlockHash: blockHash,
	}
	for i := 0; i < 7; i++ {
		signAddPOLVoteSignature(privValidators[i], valSet, voteProto, pol)
	}

	// Check that validation succeeds.
	if err := pol.Verify(valSet); err != nil {
		t.Errorf("POL.Verify() failed: %v", err)
	}
}

func TestVerifyInvalidCommits(t *testing.T) {
	height, round := uint(1), uint(2)
	_, valSet, privValidators := randVoteSet(height, round, types.VoteTypePrevote, 10, 1)

	// Make a POL with +2/3 votes with the wrong signature.
	blockHash := RandBytes(32)
	pol := &POL{
		Height: height, Round: round, BlockHash: blockHash,
		Votes: make([]POLVoteSignature, valSet.Size()),
	}
	voteProto := &types.Vote{
		Height: height, Round: round - 1, Type: types.VoteTypeCommit, BlockHash: blockHash,
	}
	for i := 0; i < 7; i++ {
		polVoteSig := signAddPOLVoteSignature(privValidators[i], valSet, voteProto, pol)
		polVoteSig.Signature[0] += byte(0x01)
	}

	// Check that validation fails.
	if err := pol.Verify(valSet); err == nil {
		t.Errorf("Expected POL.Verify() to fail, wrong signatures.")
	}
}

func TestVerifyInvalidCommitRounds(t *testing.T) {
	height, round := uint(1), uint(2)
	_, valSet, privValidators := randVoteSet(height, round, types.VoteTypePrevote, 10, 1)

	// Make a POL with +2/3 commits for the current round.
	blockHash := RandBytes(32)
	pol := &POL{
		Height: height, Round: round, BlockHash: blockHash,
		Votes: make([]POLVoteSignature, valSet.Size()),
	}
	voteProto := &types.Vote{
		Height: height, Round: round, Type: types.VoteTypeCommit, BlockHash: blockHash,
	}
	for i := 0; i < 7; i++ {
		signAddPOLVoteSignature(privValidators[i], valSet, voteProto, pol)
	}

	// Check that validation fails.
	if err := pol.Verify(valSet); err == nil {
		t.Errorf("Expected POL.Verify() to fail, same round.")
	}
}

func TestVerifyInvalidCommitRounds2(t *testing.T) {
	height, round := uint(1), uint(2)
	_, valSet, privValidators := randVoteSet(height, round, types.VoteTypePrevote, 10, 1)

	// Make a POL with +2/3 commits for future round.
	blockHash := RandBytes(32)
	pol := &POL{
		Height: height, Round: round, BlockHash: blockHash,
		Votes: make([]POLVoteSignature, valSet.Size()),
	}
	voteProto := &types.Vote{
		Height: height, Round: round + 1, Type: types.VoteTypeCommit, BlockHash: blockHash,
	}
	for i := 0; i < 7; i++ {
		polVoteSig := signAddPOLVoteSignature(privValidators[i], valSet, voteProto, pol)
		polVoteSig.Round += 1 // mutate round
	}

	// Check that validation fails.
	if err := pol.Verify(valSet); err == nil {
		t.Errorf("Expected POL.Verify() to fail, future round.")
	}
}

func TestReadWrite(t *testing.T) {
	height, round := uint(1), uint(2)
	_, valSet, privValidators := randVoteSet(height, round, types.VoteTypePrevote, 10, 1)

	// Make a POL with +2/3 votes.
	blockHash := RandBytes(32)
	pol := &POL{
		Height: height, Round: round, BlockHash: blockHash,
		Votes: make([]POLVoteSignature, valSet.Size()),
	}
	voteProto := &types.Vote{
		Height: height, Round: round, Type: types.VoteTypePrevote, BlockHash: blockHash,
	}
	for i := 0; i < 7; i++ {
		signAddPOLVoteSignature(privValidators[i], valSet, voteProto, pol)
	}

	// Write it to a buffer.
	buf, n, err := new(bytes.Buffer), new(int64), new(error)
	binary.WriteBinary(pol, buf, n, err)
	if *err != nil {
		t.Fatalf("Failed to write POL: %v", *err)
	}

	// Read from buffer.
	pol2 := binary.ReadBinary(&POL{}, buf, n, err).(*POL)
	if *err != nil {
		t.Fatalf("Failed to read POL")
	}

	// Check that validation succeeds.
	if err := pol2.Verify(valSet); err != nil {
		t.Errorf("POL.Verify() failed: %v", err)
	}
}
