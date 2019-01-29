package lite

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/types"
)

// FullCommit contains a SignedHeader (the block header and a commit that signs it),
// the validator set which signed the commit, and the next validator set. The
// next validator set (which is proven from the block header) allows us to
// revert to block-by-block updating of lite Verifier's latest validator set,
// even in the face of arbitrarily large power changes.
type FullCommit struct {
	SignedHeader   types.SignedHeader  `json:"signed_header"`
	Validators     *types.ValidatorSet `json:"validator_set"`
	NextValidators *types.ValidatorSet `json:"next_validator_set"`
}

// NewFullCommit returns a new FullCommit.
func NewFullCommit(signedHeader types.SignedHeader, valset, nextValset *types.ValidatorSet) FullCommit {
	return FullCommit{
		SignedHeader:   signedHeader,
		Validators:     valset,
		NextValidators: nextValset,
	}
}

// Validate the components and check for consistency.
// This also checks to make sure that Validators actually
// signed the SignedHeader.Commit.
// If > 2/3 did not sign the Commit from fc.Validators, it
// is not a valid commit!
func (fc FullCommit) ValidateFull(chainID string) error {
	// Ensure that Validators exists and matches the header.
	if fc.Validators.Size() == 0 {
		return errors.New("need FullCommit.Validators")
	}
	if !bytes.Equal(
		fc.SignedHeader.ValidatorsHash,
		fc.Validators.Hash()) {
		return fmt.Errorf("header has vhash %X but valset hash is %X",
			fc.SignedHeader.ValidatorsHash,
			fc.Validators.Hash(),
		)
	}
	// Ensure that NextValidators exists and matches the header.
	if fc.NextValidators.Size() == 0 {
		return errors.New("need FullCommit.NextValidators")
	}
	if !bytes.Equal(
		fc.SignedHeader.NextValidatorsHash,
		fc.NextValidators.Hash()) {
		return fmt.Errorf("header has next vhash %X but next valset hash is %X",
			fc.SignedHeader.NextValidatorsHash,
			fc.NextValidators.Hash(),
		)
	}
	// Validate the header.
	err := fc.SignedHeader.ValidateBasic(chainID)
	if err != nil {
		return err
	}
	// Validate the signatures on the commit.
	hdr, cmt := fc.SignedHeader.Header, fc.SignedHeader.Commit
	return fc.Validators.VerifyCommit(
		hdr.ChainID, cmt.BlockID,
		hdr.Height, cmt)
}

// Height returns the height of the header.
func (fc FullCommit) Height() int64 {
	if fc.SignedHeader.Header == nil {
		panic("should not happen")
	}
	return fc.SignedHeader.Height
}

// ChainID returns the chainID of the header.
func (fc FullCommit) ChainID() string {
	if fc.SignedHeader.Header == nil {
		panic("should not happen")
	}
	return fc.SignedHeader.ChainID
}
