package consensus

import (
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"

	"bytes"
	"testing"
)

// NOTE: see consensus/test.go for common test methods.

func TestVerifyVotes(t *testing.T) {
	height, round := uint32(1), uint16(0)
	_, valSet, privAccounts := makeVoteSet(height, round, VoteTypePrevote, 10, 1)

	// Make a POL with -2/3 votes.
	blockHash := RandBytes(32)
	pol := &POL{
		Height: height, Round: round, BlockHash: blockHash,
	}
	voteProto := &Vote{
		Height: height, Round: round, Type: VoteTypePrevote, BlockHash: blockHash,
	}
	for i := 0; i < 6; i++ {
		vote := voteProto.Copy()
		privAccounts[i].Sign(vote)
		pol.Votes = append(pol.Votes, vote.Signature)
	}

	// Check that validation fails.
	if err := pol.Verify(valSet); err == nil {
		t.Errorf("Expected POL.Verify() to fail, not enough votes.")
	}

	// Make a POL with +2/3 votes.
	vote := voteProto.Copy()
	privAccounts[7].Sign(vote)
	pol.Votes = append(pol.Votes, vote.Signature)

	// Check that validation succeeds.
	if err := pol.Verify(valSet); err != nil {
		t.Errorf("Expected POL.Verify() to succeed")
	}
}

func TestVerifyInvalidVote(t *testing.T) {
	height, round := uint32(1), uint16(0)
	_, valSet, privAccounts := makeVoteSet(height, round, VoteTypePrevote, 10, 1)

	// Make a POL with +2/3 votes with the wrong signature.
	blockHash := RandBytes(32)
	pol := &POL{
		Height: height, Round: round, BlockHash: blockHash,
	}
	voteProto := &Vote{
		Height: height, Round: round, Type: VoteTypePrevote, BlockHash: blockHash,
	}
	for i := 0; i < 7; i++ {
		vote := voteProto.Copy()
		privAccounts[i].Sign(vote)
		// Mutate the signature.
		vote.Signature.Bytes[0] += byte(0x01)
		pol.Votes = append(pol.Votes, vote.Signature)
	}

	// Check that validation fails.
	if err := pol.Verify(valSet); err == nil {
		t.Errorf("Expected POL.Verify() to fail, wrong signatures.")
	}
}

func TestVerifyCommits(t *testing.T) {
	height, round := uint32(1), uint16(2)
	_, valSet, privAccounts := makeVoteSet(height, round, VoteTypePrevote, 10, 1)

	// Make a POL with +2/3 votes.
	blockHash := RandBytes(32)
	pol := &POL{
		Height: height, Round: round, BlockHash: blockHash,
	}
	voteProto := &Vote{
		Height: height, Round: round - 1, Type: VoteTypeCommit, BlockHash: blockHash,
	}
	for i := 0; i < 7; i++ {
		vote := voteProto.Copy()
		privAccounts[i].Sign(vote)
		pol.Commits = append(pol.Commits, RoundSignature{round - 1, vote.Signature})
	}

	// Check that validation succeeds.
	if err := pol.Verify(valSet); err != nil {
		t.Errorf("Expected POL.Verify() to succeed")
	}
}

func TestVerifyInvalidCommits(t *testing.T) {
	height, round := uint32(1), uint16(2)
	_, valSet, privAccounts := makeVoteSet(height, round, VoteTypePrevote, 10, 1)

	// Make a POL with +2/3 votes with the wrong signature.
	blockHash := RandBytes(32)
	pol := &POL{
		Height: height, Round: round, BlockHash: blockHash,
	}
	voteProto := &Vote{
		Height: height, Round: round - 1, Type: VoteTypeCommit, BlockHash: blockHash,
	}
	for i := 0; i < 7; i++ {
		vote := voteProto.Copy()
		privAccounts[i].Sign(vote)
		// Mutate the signature.
		vote.Signature.Bytes[0] += byte(0x01)
		pol.Commits = append(pol.Commits, RoundSignature{round - 1, vote.Signature})
	}

	// Check that validation fails.
	if err := pol.Verify(valSet); err == nil {
		t.Errorf("Expected POL.Verify() to fail, wrong signatures.")
	}
}

func TestVerifyInvalidCommitRounds(t *testing.T) {
	height, round := uint32(1), uint16(2)
	_, valSet, privAccounts := makeVoteSet(height, round, VoteTypePrevote, 10, 1)

	// Make a POL with +2/3 commits for the current round.
	blockHash := RandBytes(32)
	pol := &POL{
		Height: height, Round: round, BlockHash: blockHash,
	}
	voteProto := &Vote{
		Height: height, Round: round, Type: VoteTypeCommit, BlockHash: blockHash,
	}
	for i := 0; i < 7; i++ {
		vote := voteProto.Copy()
		privAccounts[i].Sign(vote)
		pol.Commits = append(pol.Commits, RoundSignature{round, vote.Signature})
	}

	// Check that validation fails.
	if err := pol.Verify(valSet); err == nil {
		t.Errorf("Expected POL.Verify() to fail, same round.")
	}
}

func TestVerifyInvalidCommitRounds2(t *testing.T) {
	height, round := uint32(1), uint16(2)
	_, valSet, privAccounts := makeVoteSet(height, round, VoteTypePrevote, 10, 1)

	// Make a POL with +2/3 commits for future round.
	blockHash := RandBytes(32)
	pol := &POL{
		Height: height, Round: round, BlockHash: blockHash,
	}
	voteProto := &Vote{
		Height: height, Round: round + 1, Type: VoteTypeCommit, BlockHash: blockHash,
	}
	for i := 0; i < 7; i++ {
		vote := voteProto.Copy()
		privAccounts[i].Sign(vote)
		pol.Commits = append(pol.Commits, RoundSignature{round + 1, vote.Signature})
	}

	// Check that validation fails.
	if err := pol.Verify(valSet); err == nil {
		t.Errorf("Expected POL.Verify() to fail, future round.")
	}
}

func TestReadWrite(t *testing.T) {
	height, round := uint32(1), uint16(2)
	_, valSet, privAccounts := makeVoteSet(height, round, VoteTypePrevote, 10, 1)

	// Make a POL with +2/3 votes.
	blockHash := RandBytes(32)
	pol := &POL{
		Height: height, Round: round, BlockHash: blockHash,
	}
	voteProto := &Vote{
		Height: height, Round: round, Type: VoteTypePrevote, BlockHash: blockHash,
	}
	for i := 0; i < 7; i++ {
		vote := voteProto.Copy()
		privAccounts[i].Sign(vote)
		pol.Votes = append(pol.Votes, vote.Signature)
	}

	// Write it to a buffer.
	buf := new(bytes.Buffer)
	_, err := pol.WriteTo(buf)
	if err != nil {
		t.Fatalf("Failed to write POL")
	}

	// Read from buffer.
	var n int64
	pol2 := ReadPOL(buf, &n, &err)
	if err != nil {
		t.Fatalf("Failed to read POL")
	}

	// Check that validation succeeds.
	if err := pol2.Verify(valSet); err != nil {
		t.Errorf("Expected POL.Verify() to succeed")
	}
}
