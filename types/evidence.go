package types

import (
	"bytes"
	"fmt"
)

// Evidence represents any provable malicious activity by a validator
type Evidence interface {
	Verify() error
	Address() []byte
}

//-------------------------------------------

type DuplicateVoteEvidence struct {
	VoteA *Vote
	VoteB *Vote
}

// Address returns the address of the validator
func (dve *DuplicateVoteEvidence) Address() []byte {
	return dve.VoteA.ValidatorAddress
}

// Verify returns an error if the two votes aren't from the same validator, for the same H/R/S, but for different blocks
func (dve *DuplicateVoteEvidence) Verify() error {
	// H/R/S must be the same
	if dve.VoteA.Height != dve.VoteB.Height ||
		dve.VoteA.Round != dve.VoteB.Round ||
		dve.VoteA.Type != dve.VoteB.Type {
		return fmt.Errorf("DuplicateVoteEvidence Error: H/R/S does not match. Got %v and %v", dve.VoteA, dve.VoteB)
	}

	// Address and Index must be the same
	if !bytes.Equal(dve.VoteA.ValidatorAddress, dve.VoteB.ValidatorAddress) {
		return fmt.Errorf("DuplicateVoteEvidence Error: Validator addresses do not match. Got %X and %X", dve.VoteA.ValidatorAddress, dve.VoteB.ValidatorAddress)
	}
	if dve.VoteA.ValidatorIndex != dve.VoteB.ValidatorIndex {
		return fmt.Errorf("DuplicateVoteEvidence Error: Validator indices do not match. Got %d and %d", dve.VoteA.ValidatorIndex, dve.VoteB.ValidatorIndex)
	}

	// BlockIDs must be different
	if dve.VoteA.BlockID.Equals(dve.VoteB.BlockID) {
		return fmt.Errorf("DuplicateVoteEvidence Error: BlockIDs are the same (%v) - not a real duplicate vote!", dve.VoteA.BlockID)
	}

	// Signatures must be valid
	// TODO

	return nil
}
