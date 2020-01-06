package lite

import (
	"fmt"
	"time"
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

// ErrInvalidNewHeader means the new (untrusted) header is mailformed.
type ErrInvalidNewHeader struct {
	Reason error
}

func (e ErrInvalidNewHeader) Error() string {
	return fmt.Sprintf("invalid new header: %v", e.Reason)
}

// ErrNewValSetCantBeTrusted means the new validator set cannot be trusted
// either because:
//		a) Reason=ErrNotEnoughVotingPowerSigned: < 1/3rd (+trustLevel+) of the
//		old validator set has signed (for non-adjacent headers);
//		b) Reason=error new validator set hash != old (trusted) header's next
//		validator set hash (for adjacent headers).
type ErrNewValSetCantBeTrusted struct {
	Reason error
}

func (e ErrNewValSetCantBeTrusted) Error() string {
	return fmt.Sprintf("cant trust new val set: %v", e.Reason)
}

// ErrNewHeaderCantBeTrusted means the new validator set is trusted, but the
// header cannot be trusted. E.g. because < 2/3rds has signed
// (Reason=ErrNotEnoughVotingPowerSigned).
type ErrNewHeaderCantBeTrusted struct {
	Reason error
}

func (e ErrNewHeaderCantBeTrusted) Error() string {
	return fmt.Sprintf("cant trust new header: %v", e.Reason)
}
