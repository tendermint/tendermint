package blocksync

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/light"
	"github.com/tendermint/tendermint/types"
)

func VerifyAdjacent(
	trustedHeader *types.SignedHeader, // height=X
	untrustedHeader *types.SignedHeader, // height=X+1
	untrustedVals *types.ValidatorSet, // height=X+1)
) error {

	if len(trustedHeader.NextValidatorsHash) == 0 {
		return errors.New("next validators hash in trusted header is empty")
	}

	if untrustedHeader.Height != trustedHeader.Height+1 {
		return errors.New("headers must be adjacent in height")
	}

	if err := untrustedHeader.ValidateBasic(trustedHeader.ChainID); err != nil {
		return fmt.Errorf("untrustedHeader.ValidateBasic failed: %w", err)
	}

	if untrustedHeader.Height <= trustedHeader.Height {
		return fmt.Errorf("expected new header height %d to be greater than one of old header %d",
			untrustedHeader.Height,
			trustedHeader.Height)
	}

	if !untrustedHeader.Time.After(trustedHeader.Time) {
		return fmt.Errorf("expected new header time %v to be after old header time %v",
			untrustedHeader.Time,
			trustedHeader.Time)
	}

	if !bytes.Equal(untrustedHeader.ValidatorsHash, untrustedVals.Hash()) {
		return fmt.Errorf("expected new header validators (%X) to match those that were supplied (%X) at height %d",
			untrustedHeader.ValidatorsHash,
			untrustedVals.Hash(),
			untrustedHeader.Height,
		)
	}

	// Check the validator hashes are the same
	if !bytes.Equal(untrustedHeader.ValidatorsHash, trustedHeader.NextValidatorsHash) {
		err := fmt.Errorf("expected old header's next validators (%X) to match those from new header (%X)",
			trustedHeader.NextValidatorsHash,
			untrustedHeader.ValidatorsHash,
		)
		return light.ErrInvalidHeader{Reason: err}
	}
	return nil
}

type ErrBlockIDDiff struct {
	Reason error
}

func (e ErrBlockIDDiff) Error() string {
	return "block ID in lastCommit of new block is not matching trusted block ID"
}

type ErrInvalidVerifyBlock struct {
	Reason error
}

func (e ErrInvalidVerifyBlock) Error() string {
	return "last commit of invalid block used to verify new block"
}

type ErrValidationFailed struct {
	Reason error
}

func (e ErrValidationFailed) Error() string {
	return "failed to verify next block"
}

func VerifyNextBlock(newBlock *types.Block, newBlockID types.BlockID, verifyBlock *types.Block, trustedBlock *types.Block, trustedCommit *types.Commit, validators *types.ValidatorSet) error {

	// If the blockID in LastCommit of NewBlock does not match the trusted block
	// we can assume NewBlock is not correct
	if !(newBlock.LastCommit.BlockID.Equals(trustedCommit.BlockID)) {
		return ErrBlockIDDiff{}
	}

	// Todo: Verify verifyBlock.LastCommit validators against state.NextValidators
	// If they do not match, need a new verifyBlock
	if err := validators.VerifyCommitLight(trustedBlock.ChainID, newBlockID, newBlock.Height, verifyBlock.LastCommit); err != nil {
		return ErrInvalidVerifyBlock{Reason: err}
	}

	// Verify NewBlock usign the validator set obtained after applying the last block
	// Note: VerifyAdjacent in the LightClient relies on a trusting period which is not applicable here
	// ToDo: We need witness verification here as well and backwards verification from a state where we can trust validators
	if err := VerifyAdjacent(&types.SignedHeader{Header: &trustedBlock.Header, Commit: trustedCommit}, &types.SignedHeader{Header: &newBlock.Header, Commit: verifyBlock.LastCommit}, validators); err != nil {
		return ErrValidationFailed{Reason: err}
	}

	return nil

}
