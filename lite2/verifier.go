package lite

import (
	"bytes"
	"time"

	"github.com/pkg/errors"

	tmmath "github.com/tendermint/tendermint/libs/math"
	"github.com/tendermint/tendermint/types"
)

var (
	// DefaultTrustLevel - new header can be trusted if at least one correct old
	// validator signed it.
	DefaultTrustLevel = tmmath.Fraction{Numerator: 1, Denominator: 3}
)

func Verify(
	chainID string,
	h1 *types.SignedHeader,
	h1NextVals *types.ValidatorSet,
	h2 *types.SignedHeader,
	h2Vals *types.ValidatorSet,
	trustingPeriod time.Duration,
	now time.Time,
	trustLevel tmmath.Fraction) error {

	if err := ValidateTrustLevel(trustLevel); err != nil {
		return err
	}

	// Ensure last header can still be trusted.
	expirationTime := h1.Time.Add(trustingPeriod)
	if !expirationTime.After(now) {
		return ErrOldHeaderExpired{expirationTime, now}
	}

	if err := verifyNewHeaderAndVals(chainID, h2, h2Vals, h1, now); err != nil {
		return err
	}

	if h2.Height == h1.Height+1 {
		if !bytes.Equal(h2.ValidatorsHash, h1NextVals.Hash()) {
			return errors.Errorf("expected old header validators (%X) to match those from new header (%X)",
				h1NextVals.Hash(),
				h2.ValidatorsHash,
			)
		}
	} else {
		// Ensure that +`trustLevel` (default 1/3) or more of last trusted validators signed correctly.
		err := h1NextVals.VerifyCommitTrusting(chainID, h2.Commit.BlockID, h2.Height, h2.Commit, trustLevel)
		if err != nil {
			return err
		}
	}

	// Ensure that +2/3 of new validators signed correctly.
	err := h2Vals.VerifyCommit(chainID, h2.Commit.BlockID, h2.Height, h2.Commit)
	if err != nil {
		return err
	}

	return nil
}

func verifyNewHeaderAndVals(
	chainID string,
	h2 *types.SignedHeader,
	h2Vals *types.ValidatorSet,
	h1 *types.SignedHeader,
	now time.Time) error {

	if err := h2.ValidateBasic(chainID); err != nil {
		return errors.Wrap(err, "h2.ValidateBasic failed")
	}

	if h2.Height <= h1.Height {
		return errors.Errorf("expected new header height %d to be greater than one of old header %d",
			h2.Height,
			h1.Height)
	}

	if !h2.Time.After(h1.Time) {
		return errors.Errorf("expected new header time %v to be after old header time %v",
			h2.Time,
			h1.Time)
	}

	if !h2.Time.Before(now) {
		return errors.Errorf("new header has a time from the future %v (now: %v)",
			h2.Time,
			now)
	}

	if !bytes.Equal(h2.ValidatorsHash, h2Vals.Hash()) {
		return errors.Errorf("expected new header validators (%X) to match those that were supplied (%X)",
			h2Vals.Hash(),
			h2.NextValidatorsHash,
		)
	}

	return nil
}

// ValidateTrustLevel checks that trustLevel is within the allowed range [1/3,
// 1]. If not, it returns an error. 1/3 is the minimum amount of trust needed
// which does not break the security model.
func ValidateTrustLevel(lvl tmmath.Fraction) error {
	if lvl.Numerator*3 < lvl.Denominator || // < 1/3
		lvl.Numerator > lvl.Denominator || // > 1
		lvl.Denominator == 0 {
		return errors.Errorf("trustLevel must be within [1/3, 1], given %v", lvl)
	}
	return nil
}
