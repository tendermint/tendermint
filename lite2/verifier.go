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

// VerifyViaVals verifies the new header (unverifiedHeader) against the old header (trustedHeader). It ensures that:
//
//	a) trustedHeader can still be trusted (if not, ErrOldHeaderExpired is returned);
//	b) unverifiedHeader is valid;
//	c) trustLevel ([1/3, 1]) of last trusted validators (trustedHeaderNextVals) signed
//		 correctly  (if not, ErrNewValSetCantBeTrusted is returned);
//	d) more than 2/3 of new validators (unverifiedVals) have signed h2 (if not,
//	   ErrNotEnoughVotingPowerSigned is returned).
func VerifyViaVals(
	chainID string,
	trustedHeader *types.SignedHeader,
	trustedNextVals *types.ValidatorSet,
	unverifiedHeader *types.SignedHeader,
	unverifiedVals *types.ValidatorSet,
	trustingPeriod time.Duration,
	now time.Time,
	trustLevel tmmath.Fraction,
) error {

	err := initialHeaderVerification(unverifiedHeader, trustedHeader, trustingPeriod, now, chainID, unverifiedVals)
	if err != nil {
		return err
	}

	// Ensure that +`trustLevel` (default 1/3) or more of last trusted validators signed correctly.
	err = trustedNextVals.VerifyCommitTrusting(chainID, unverifiedHeader.Commit.BlockID, unverifiedHeader.Height,
		unverifiedHeader.Commit, trustLevel)
	if err != nil {
		switch e := err.(type) {
		case types.ErrNotEnoughVotingPowerSigned:
			return ErrNewValSetCantBeTrusted{e}
		default:
			return e
		}
	}
	return nil
}

// VerifyViaSigs verifies the directly adjacent new header (unverifiedHeader) against the old header (trustedHeader).
// It ensures that:
//
//	a) trustedHeader can still be trusted (if not, ErrOldHeaderExpired is returned);
//	b) unverifiedHeader is valid;
//	c) unverifiedHeader.ValidatorsHash equals trustedHeaderNextVals.Hash()
//	d) more than 2/3 of new validators (unverifiedVals) have signed h2 (if not,
//	   ErrNotEnoughVotingPowerSigned is returned).
func VerifyViaSigs(
	chainID string,
	trustedHeader *types.SignedHeader,
	unverifiedHeader *types.SignedHeader,
	unverifiedVals *types.ValidatorSet,
	trustingPeriod time.Duration,
	now time.Time) error {

	if unverifiedHeader.Height != trustedHeader.Height+1 {
		return errors.New("headers must be adjacent in height")
	}

	err := initialHeaderVerification(unverifiedHeader, trustedHeader, trustingPeriod, now, chainID, unverifiedVals)
	if err != nil {
		return err
	}

	// Check the validator hashes are the same
	if !bytes.Equal(unverifiedHeader.ValidatorsHash, trustedHeader.NextValidatorsHash) {
		err := errors.Errorf("expected old header next validators (%X) to match those from new header (%X)",
			trustedHeader.NextValidatorsHash,
			unverifiedHeader.ValidatorsHash,
		)
		return err
	}
	return nil
}

func initialHeaderVerification(unverifiedHeader *types.SignedHeader, trustedHeader *types.SignedHeader,
	trustingPeriod time.Duration, now time.Time, chainID string, unverifiedVals *types.ValidatorSet) error {
	// Ensure last header can still be trusted.
	if HeaderExpired(trustedHeader, trustingPeriod, now) {
		return ErrOldHeaderExpired{trustedHeader.Time.Add(trustingPeriod), now}
	}

	if err := verifyNewHeaderAndVals(chainID, unverifiedHeader, unverifiedVals, trustedHeader, now); err != nil {
		return err
	}

	// Ensure that +2/3 of new validators signed correctly.
	if err := unverifiedVals.VerifyCommit(chainID, unverifiedHeader.Commit.BlockID, unverifiedHeader.Height,
		unverifiedHeader.Commit); err != nil {
		return err
	}
	return nil
}

func verifyNewHeaderAndVals(
	chainID string,
	unverifiedHeader *types.SignedHeader,
	unverifiedVals *types.ValidatorSet,
	trustedHeader *types.SignedHeader,
	now time.Time) error {

	if err := unverifiedHeader.ValidateBasic(chainID); err != nil {
		return errors.Wrap(err, "unverifiedHeader.ValidateBasic failed")
	}

	if unverifiedHeader.Height <= trustedHeader.Height {
		return errors.Errorf("expected new header height %d to be greater than one of old header %d",
			unverifiedHeader.Height,
			trustedHeader.Height)
	}

	if !unverifiedHeader.Time.After(trustedHeader.Time) {
		return errors.Errorf("expected new header time %v to be after old header time %v",
			unverifiedHeader.Time,
			trustedHeader.Time)
	}

	if !unverifiedHeader.Time.Before(now) {
		return errors.Errorf("new header has a time from the future %v (now: %v)",
			unverifiedHeader.Time,
			now)
	}

	if !bytes.Equal(unverifiedHeader.ValidatorsHash, unverifiedVals.Hash()) {
		return errors.Errorf("expected new header validators (%X) to match those that were supplied (%X)",
			unverifiedHeader.ValidatorsHash,
			unverifiedVals.Hash(),
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

// HeaderExpired return true if the given header expired.
func HeaderExpired(h *types.SignedHeader, trustingPeriod time.Duration, now time.Time) bool {
	expirationTime := h.Time.Add(trustingPeriod)
	return !expirationTime.After(now)
}
