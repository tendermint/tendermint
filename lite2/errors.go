package lite

import (
	"fmt"
	"time"
)

// ErrNewHeaderTooFarIntoFuture means the new (untrusted) header has a
// timestamp beyound the trustingPeriod. If so, the light client must be
// pointed to another full node.
type ErrNewHeaderTooFarIntoFuture struct {
	HeaderTime           time.Time
	TrustingPeriodEndsAt time.Time
}

func (e ErrNewHeaderTooFarIntoFuture) Error() string {
	return fmt.Sprintf("expected new header %v to be within the trusting period, which ends at %v",
		e.HeaderTime, e.TrustingPeriodEndsAt)
}

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
