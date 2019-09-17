package lite

import (
	"bytes"
	"time"

	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/types"
)

func Verify(chainID string,
	lastHeader *types.SignedHeader,
	lastVals *types.ValidatorSet,
	newHeader *types.SignedHeader,
	newVals *types.ValidatorSet,
	now time.Time) error {

	if err := verifyNewHeaderAndVals(chainID, newHeader, newVals, lastHeader, now); err != nil {
		return err
	}

	if newHeader.Height == c.state.Header.Height+1 {
		if !bytes.Equal(newHeader.ValidatorsHash, lastVals.Hash()) {
			return errors.Errorf("expected our validators (%X) to match those from new header (%X)",
				lastVals.Hash(),
				newHeader.ValidatorsHash,
			)
		}
	} else {
		// Ensure that +1/3 of last trusted validators signed correctly.
		err := lastVals.VerifyCommitTrusting(chainID, newHeader.Commit.BlockID, newHeader.Height, newHeader.Commit, v.trustLevel)
		if err != nil {
			return err
		}
	}

	// Ensure that +2/3 of current validators signed correctly.
	err = newVals.VerifyCommit(v.chainID, newHeader.Commit.BlockID, newHeader.Height, newHeader.Commit)
	if err != nil {
		return err
	}

	return nil
}

func verifyNewHeaderAndVals(
	chainID string,
	newHeader *types.SignedHeader,
	newVals *types.ValidatorSet,
	lastHeader *types.SignedHeader,
	now time.Time) error {

	if err := newHeader.ValidateBasic(chainID); err != nil {
		return errors.Wrap(err, "newHeader.ValidateBasic failed")
	}

	if newHeader.Height <= lastHeader.Height {
		return errors.Errorf("expected new header height %d to be greater than one of last header %d",
			newHeader.Height,
			lastHeader.Height)
	}

	if !newHeader.Time.After(lastHeader.Time) {
		return errors.Errorf("expected new header time %v to be after last header time %v",
			newHeader.Time,
			lastHeader.Time)
	}

	if !newHeader.Time.Before(now) {
		return errors.Errorf("new header has a time from the future %v (now: %v)",
			newHeader.Time,
			now)
	}

	if !bytes.Equal(newHeader.ValidatorsHash, newVals.Hash()) {
		return errors.Errorf("expected new header validators (%X) to match those that were supplied (%X)",
			newVals.Hash(),
			newHeader.NextValidatorsHash,
		)
	}

	return nil
}
