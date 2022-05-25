package light

import (
	"bytes"
	"errors"
	"fmt"
	"time"

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
//	a) trustedHeader can still be trusted (if not, ErrOldHeaderExpired is returned)
//	b) untrustedHeader is valid (if not, ErrInvalidHeader is returned)
//	c) trustLevel ([1/3, 1]) of trustedHeaderVals (or trustedHeaderNextVals)
//  signed correctly (if not, ErrNewValSetCantBeTrusted is returned)
//	d) more than 2/3 of untrustedVals have signed h2
//    (otherwise, ErrInvalidHeader is returned)
//  e) headers are non-adjacent.
//
// maxClockDrift defines how much untrustedHeader.Time can drift into the
// future.
// trustedHeader must have a ChainID, Height and Time
func VerifyNonAdjacent(
	trustedHeader *types.SignedHeader, // height=X
	trustedVals *types.ValidatorSet, // height=X or height=X+1
	untrustedHeader *types.SignedHeader, // height=Y
	untrustedVals *types.ValidatorSet, // height=Y
	trustingPeriod time.Duration,
	now time.Time,
	maxClockDrift time.Duration,
	trustLevel tmmath.Fraction,
) error {

	if err := checkRequiredHeaderFields(trustedHeader); err != nil {
		return err
	}

	if untrustedHeader.Height == trustedHeader.Height+1 {
		return errors.New("headers must be non adjacent in height")
	}

	if err := ValidateTrustLevel(trustLevel); err != nil {
		return err
	}

	// check if the untrusted header is within the trust period
	if HeaderExpired(untrustedHeader, trustingPeriod, now) {
		return ErrOldHeaderExpired{untrustedHeader.Time.Add(trustingPeriod), now}
	}

	if err := verifyNewHeaderAndVals(
		untrustedHeader, untrustedVals,
		trustedHeader,
		now, maxClockDrift); err != nil {
		return ErrInvalidHeader{err}
	}

	// Ensure that +`trustLevel` (default 1/3) or more in voting power of the last trusted validator
	// set signed correctly.
	err := trustedVals.VerifyCommitLightTrusting(trustedHeader.ChainID, untrustedHeader.Commit, trustLevel)
	if err != nil {
		switch e := err.(type) {
		case types.ErrNotEnoughVotingPowerSigned:
			return ErrNewValSetCantBeTrusted{e}
		default:
			return ErrInvalidHeader{e}
		}
	}

	// Ensure that +2/3 of new validators signed correctly.
	//
	// NOTE: this should always be the last check because untrustedVals can be
	// intentionally made very large to DOS the light client. not the case for
	// VerifyAdjacent, where validator set is known in advance.
	if err := untrustedVals.VerifyCommitLight(trustedHeader.ChainID, untrustedHeader.Commit.BlockID,
		untrustedHeader.Height, untrustedHeader.Commit); err != nil {
		return ErrInvalidHeader{err}
	}

	return nil
}

// VerifyAdjacent verifies directly adjacent untrustedHeader against
// trustedHeader. It ensures that:
//
//  a) trustedHeader can still be trusted (if not, ErrOldHeaderExpired is returned)
//  b) untrustedHeader is valid (if not, ErrInvalidHeader is returned)
//  c) untrustedHeader.ValidatorsHash equals trustedHeader.NextValidatorsHash
//  d) more than 2/3 of new validators (untrustedVals) have signed h2
//    (otherwise, ErrInvalidHeader is returned)
//  e) headers are adjacent.
//
// maxClockDrift defines how much untrustedHeader.Time can drift into the
// future.
// trustedHeader must have a ChainID, Height, Time and NextValidatorsHash
func VerifyAdjacent(
	trustedHeader *types.SignedHeader, // height=X
	untrustedHeader *types.SignedHeader, // height=X+1
	untrustedVals *types.ValidatorSet, // height=X+1
	trustingPeriod time.Duration,
	now time.Time,
	maxClockDrift time.Duration,
) error {

	if err := checkRequiredHeaderFields(trustedHeader); err != nil {
		return err
	}

	if len(trustedHeader.NextValidatorsHash) == 0 {
		return errors.New("next validators hash in trusted header is empty")
	}

	if untrustedHeader.Height != trustedHeader.Height+1 {
		return errors.New("headers must be adjacent in height")
	}

	// check if the untrusted header is within the trust period
	if HeaderExpired(untrustedHeader, trustingPeriod, now) {
		return ErrOldHeaderExpired{untrustedHeader.Time.Add(trustingPeriod), now}
	}

	if err := verifyNewHeaderAndVals(
		untrustedHeader, untrustedVals,
		trustedHeader,
		now, maxClockDrift); err != nil {
		return ErrInvalidHeader{err}
	}

	// Check the validator hashes are the same
	if !bytes.Equal(untrustedHeader.ValidatorsHash, trustedHeader.NextValidatorsHash) {
		err := fmt.Errorf("expected old header's next validators (%X) to match those from new header (%X)",
			trustedHeader.NextValidatorsHash,
			untrustedHeader.ValidatorsHash,
		)
		return ErrInvalidHeader{err}
	}

	// Ensure that +2/3 of new validators signed correctly.
	if err := untrustedVals.VerifyCommitLight(trustedHeader.ChainID, untrustedHeader.Commit.BlockID,
		untrustedHeader.Height, untrustedHeader.Commit); err != nil {
		return ErrInvalidHeader{err}
	}

	return nil
}

// Verify combines both VerifyAdjacent and VerifyNonAdjacent functions.
func Verify(
	trustedHeader *types.SignedHeader, // height=X
	trustedVals *types.ValidatorSet, // height=X or height=X+1
	untrustedHeader *types.SignedHeader, // height=Y
	untrustedVals *types.ValidatorSet, // height=Y
	trustingPeriod time.Duration,
	now time.Time,
	maxClockDrift time.Duration,
	trustLevel tmmath.Fraction) error {

	if untrustedHeader.Height != trustedHeader.Height+1 {
		return VerifyNonAdjacent(trustedHeader, trustedVals, untrustedHeader, untrustedVals,
			trustingPeriod, now, maxClockDrift, trustLevel)
	}

	return VerifyAdjacent(trustedHeader, untrustedHeader, untrustedVals, trustingPeriod, now, maxClockDrift)
}

// ValidateTrustLevel checks that trustLevel is within the allowed range [1/3,
// 1]. If not, it returns an error. 1/3 is the minimum amount of trust needed
// which does not break the security model. Must be strictly less than 1.
func ValidateTrustLevel(lvl tmmath.Fraction) error {
	if lvl.Numerator*3 < lvl.Denominator || // < 1/3
		lvl.Numerator >= lvl.Denominator || // >= 1
		lvl.Denominator == 0 {
		return fmt.Errorf("trustLevel must be within [1/3, 1], given %v", lvl)
	}
	return nil
}

// HeaderExpired return true if the given header expired.
func HeaderExpired(h *types.SignedHeader, trustingPeriod time.Duration, now time.Time) bool {
	expirationTime := h.Time.Add(trustingPeriod)
	return !expirationTime.After(now)
}

// VerifyBackwards verifies an untrusted header with a height one less than
// that of an adjacent trusted header. It ensures that:
//
// 	a) untrusted header is valid
//  b) untrusted header has a time before the trusted header
//  c) that the LastBlockID hash of the trusted header is the same as the hash
//  of the trusted header
//
// For any of these cases ErrInvalidHeader is returned.
// NOTE: This does not check whether the trusted or untrusted header has expired
// or not. These checks are not necessary because the detector never runs during
// backwards verification and thus evidence that needs to be within a certain
// time bound is never sent.
func VerifyBackwards(untrustedHeader, trustedHeader *types.Header) error {
	if err := untrustedHeader.ValidateBasic(); err != nil {
		return ErrInvalidHeader{err}
	}

	if untrustedHeader.ChainID != trustedHeader.ChainID {
		return ErrInvalidHeader{fmt.Errorf("new header belongs to a different chain (%s != %s)",
			untrustedHeader.ChainID, trustedHeader.ChainID)}
	}

	if !untrustedHeader.Time.Before(trustedHeader.Time) {
		return ErrInvalidHeader{
			fmt.Errorf("expected older header time %v to be before new header time %v",
				untrustedHeader.Time,
				trustedHeader.Time)}
	}

	if !bytes.Equal(untrustedHeader.Hash(), trustedHeader.LastBlockID.Hash) {
		return ErrInvalidHeader{
			fmt.Errorf("older header hash %X does not match trusted header's last block %X",
				untrustedHeader.Hash(),
				trustedHeader.LastBlockID.Hash)}
	}

	return nil
}

// NOTE: This function assumes that untrustedHeader is after trustedHeader.
// Do not use for backwards verification.
func verifyNewHeaderAndVals(
	untrustedHeader *types.SignedHeader,
	untrustedVals *types.ValidatorSet,
	trustedHeader *types.SignedHeader,
	now time.Time,
	maxClockDrift time.Duration) error {

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

	if !untrustedHeader.Time.Before(now.Add(maxClockDrift)) {
		return fmt.Errorf("new header has a time from the future %v (now: %v; max clock drift: %v)",
			untrustedHeader.Time,
			now,
			maxClockDrift)
	}

	if !bytes.Equal(untrustedHeader.ValidatorsHash, untrustedVals.Hash()) {
		return fmt.Errorf("expected new header validators (%X) to match those that were supplied (%X) at height %d",
			untrustedHeader.ValidatorsHash,
			untrustedVals.Hash(),
			untrustedHeader.Height,
		)
	}

	return nil
}

func checkRequiredHeaderFields(h *types.SignedHeader) error {
	if h.Height == 0 {
		return errors.New("height in trusted header must be set (non zero")
	}

	if h.Time.IsZero() {
		return errors.New("time in trusted header must be set")
	}

	if h.ChainID == "" {
		return errors.New("chain ID in trusted header must be set")
	}
	return nil
}
