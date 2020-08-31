package light

import (
	"fmt"
	"time"

	"github.com/tendermint/tendermint/light/provider"
	"github.com/tendermint/tendermint/types"
)

// ErrOldHeaderExpired means the old (trusted) header has expired according to
// the given trustingPeriod and current time. If so, the light client must be
// reset subjectively.
type ErrOldHeaderExpired struct {
	At  time.Time
	Now time.Time
}

func (e ErrOldHeaderExpired) Error() string {
	return fmt.Sprintf("old header has expired at %v (now: %v)", e.At, e.Now)
}

// ErrNewValSetCantBeTrusted means the new validator set cannot be trusted
// because < 1/3rd (+trustLevel+) of the old validator set has signed.
type ErrNewValSetCantBeTrusted struct {
	Reason types.ErrNotEnoughVotingPowerSigned
}

func (e ErrNewValSetCantBeTrusted) Error() string {
	return fmt.Sprintf("cant trust new val set: %v", e.Reason)
}

// ErrInvalidHeader means the header either failed the basic validation or
// commit is not signed by 2/3+.
type ErrInvalidHeader struct {
	Reason error
}

func (e ErrInvalidHeader) Error() string {
	return fmt.Sprintf("invalid header: %v", e.Reason)
}

// ErrConflictingHeaders is thrown when two conflicting headers are discovered.
type ErrConflictingHeaders struct {
	H1      *types.SignedHeader
	Primary provider.Provider

	H2      *types.SignedHeader
	Witness provider.Provider
}

func (e ErrConflictingHeaders) Error() string {
	return fmt.Sprintf(
		"header hash %X from primary %v does not match one %X from witness %v",
		e.H1.Hash(), e.Primary,
		e.H2.Hash(), e.Witness)
}

// ErrVerificationFailed means either sequential or skipping verification has
// failed to verify from header #1 to header #2 due to some reason.
type ErrVerificationFailed struct {
	From   int64
	To     int64
	Reason error
}

// Unwrap returns underlying reason.
func (e ErrVerificationFailed) Unwrap() error {
	return e.Reason
}

func (e ErrVerificationFailed) Error() string {
	return fmt.Sprintf(
		"verify from #%d to #%d failed: %v",
		e.From, e.To, e.Reason)
}

// errNoWitnesses means that there are not enough witnesses connected to
// continue running the light client.
type errNoWitnesses struct{}

func (e errNoWitnesses) Error() string {
	return "no witnesses connected. please reset light client"
}

type badWitnessCode int

const (
	noResponse badWitnessCode = iota + 1
	invalidLightBlock
)

// errBadWitness is returned when the witness either does not respond or
// responds with an invalid header.
type errBadWitness struct {
	Reason       error
	Code         badWitnessCode
	WitnessIndex int
}

func (e errBadWitness) Error() string {
	switch e.Code {
	case noResponse:
		return fmt.Sprintf("failed to get a header/vals from witness: %v", e.Reason)
	case invalidLightBlock:
		return fmt.Sprintf("witness sent us an invalid light block: %v", e.Reason)
	default:
		return fmt.Sprintf("unknown code: %d", e.Code)
	}
}
