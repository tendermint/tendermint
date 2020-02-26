package lite

import (
	"bytes"
	"time"

	"github.com/pkg/errors"

	tmmath "github.com/tendermint/tendermint/libs/math"
	"github.com/tendermint/tendermint/types"
)

var (
	// DefaultTrustLevel - new header can be trusted if at least one correct
	// validator signed it.
	DefaultTrustLevel = tmmath.Fraction{Numerator: 1, Denominator: 3}
)

// VerifyNonAdjacent verifies non-adjacent untrustedHeader against
// trustedHeader. It ensures that:
//
//	a) trustedHeader can still be trusted (if not, ErrOldHeaderExpired is returned);
//	b) untrustedHeader is valid;
//	c) trustLevel ([1/3, 1]) of trustedHeaderVals (or trustedHeaderNextVals)
//  signed correctly (if not, ErrNewValSetCantBeTrusted is returned);
//	d) more than 2/3 of untrustedVals have signed h2 (if not,
//	   ErrNotEnoughVotingPowerSigned is returned);
//  e) headers are non-adjacent.
func VerifyNonAdjacent(
	chainID string,
	trustedHeader *types.SignedHeader, // height=X
	trustedVals *types.ValidatorSet, // height=X or height=X+1
	untrustedHeader *types.SignedHeader, // height=Y
	untrustedVals *types.ValidatorSet, // height=Y
	trustingPeriod time.Duration,
	now time.Time,
	trustLevel tmmath.Fraction) error {

	if untrustedHeader.Height == trustedHeader.Height+1 {
		return errors.New("headers must be non adjacent in height")
	}

	if HeaderExpired(trustedHeader, trustingPeriod, now) {
		return ErrOldHeaderExpired{trustedHeader.Time.Add(trustingPeriod), now}
	}

	if err := verifyNewHeaderAndVals(chainID, untrustedHeader, untrustedVals, trustedHeader, now); err != nil {
		return err
	}

	// Ensure that +`trustLevel` (default 1/3) or more of last trusted validators signed correctly.
	err := trustedVals.VerifyCommitTrusting(chainID, untrustedHeader.Commit.BlockID, untrustedHeader.Height,
		untrustedHeader.Commit, trustLevel)
	if err != nil {
		switch e := err.(type) {
		case types.ErrNotEnoughVotingPowerSigned:
			return ErrNewValSetCantBeTrusted{e}
		default:
			return e
		}
	}

	// Ensure that +2/3 of new validators signed correctly.
	//
	// NOTE: this should always be the last check because untrustedVals can be
	// intentionaly made very large to DOS the light client. not the case for
	// VerifyAdjacent, where validator set is known in advance.
	if err := untrustedVals.VerifyCommit(chainID, untrustedHeader.Commit.BlockID, untrustedHeader.Height,
		untrustedHeader.Commit); err != nil {
		return err
	}

	return nil
}

// VerifyAdjacent verifies directly adjacent untrustedHeader against
// trustedHeader. It ensures that:
//
//	a) trustedHeader can still be trusted (if not, ErrOldHeaderExpired is returned);
//	b) untrustedHeader is valid;
//	c) untrustedHeader.ValidatorsHash equals trustedHeader.NextValidatorsHash;
//	d) more than 2/3 of new validators (untrustedVals) have signed h2 (if not,
//	   ErrNotEnoughVotingPowerSigned is returned);
//  e) headers are adjacent.
func VerifyAdjacent(
	chainID string,
	trustedHeader *types.SignedHeader, // height=X
	untrustedHeader *types.SignedHeader, // height=X+1
	untrustedVals *types.ValidatorSet, // height=X+1
	trustingPeriod time.Duration,
	now time.Time) error {

	if untrustedHeader.Height != trustedHeader.Height+1 {
		return errors.New("headers must be adjacent in height")
	}

	if HeaderExpired(trustedHeader, trustingPeriod, now) {
		return ErrOldHeaderExpired{trustedHeader.Time.Add(trustingPeriod), now}
	}

	if err := verifyNewHeaderAndVals(chainID, untrustedHeader, untrustedVals, trustedHeader, now); err != nil {
		return err
	}

	// Check the validator hashes are the same
	if !bytes.Equal(untrustedHeader.ValidatorsHash, trustedHeader.NextValidatorsHash) {
		err := errors.Errorf("expected old header next validators (%X) to match those from new header (%X)",
			trustedHeader.NextValidatorsHash,
			untrustedHeader.ValidatorsHash,
		)
		return err
	}

	// Ensure that +2/3 of new validators signed correctly.
	if err := untrustedVals.VerifyCommit(chainID, untrustedHeader.Commit.BlockID, untrustedHeader.Height,
		untrustedHeader.Commit); err != nil {
		return err
	}

	return nil
}

// Verify combines both VerifyAdjacent and VerifyNonAdjacent functions.
func Verify(
	chainID string,
	trustedHeader *types.SignedHeader, // height=X
	trustedVals *types.ValidatorSet, // height=X or height=X+1
	untrustedHeader *types.SignedHeader, // height=Y
	untrustedVals *types.ValidatorSet, // height=Y
	trustingPeriod time.Duration,
	now time.Time,
	trustLevel tmmath.Fraction) error {

	if untrustedHeader.Height != trustedHeader.Height+1 {
		return VerifyNonAdjacent(chainID, trustedHeader, trustedVals, untrustedHeader, untrustedVals,
			trustingPeriod, now, trustLevel)
	}

	return VerifyAdjacent(chainID, trustedHeader, untrustedHeader, untrustedVals, trustingPeriod, now)
}

func verifyNewHeaderAndVals(
	chainID string,
	untrustedHeader *types.SignedHeader,
	untrustedVals *types.ValidatorSet,
	trustedHeader *types.SignedHeader,
	now time.Time) error {

	if err := untrustedHeader.ValidateBasic(chainID); err != nil {
		return errors.Wrap(err, "untrustedHeader.ValidateBasic failed")
	}

	if untrustedHeader.Height <= trustedHeader.Height {
		return errors.Errorf("expected new header height %d to be greater than one of old header %d",
			untrustedHeader.Height,
			trustedHeader.Height)
	}

	if !untrustedHeader.Time.After(trustedHeader.Time) {
		return errors.Errorf("expected new header time %v to be after old header time %v",
			untrustedHeader.Time,
			trustedHeader.Time)
	}

	if !untrustedHeader.Time.Before(now) {
		return errors.Errorf("new header has a time from the future %v (now: %v)",
			untrustedHeader.Time,
			now)
	}

	if !bytes.Equal(untrustedHeader.ValidatorsHash, untrustedVals.Hash()) {
		return errors.Errorf("expected new header validators (%X) to match those that were supplied (%X)",
			untrustedHeader.ValidatorsHash,
			untrustedVals.Hash(),
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
