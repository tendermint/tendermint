package lite

import (
	"fmt"
	"time"
)

type ErrNewHeaderTooFarIntoFuture struct {
	HeaderTime           time.Time
	TrustingPeriodEndsAt time.Time
}

func (e ErrNewHeaderTooFarIntoFuture) Error() string {
	return fmt.Sprintf("expected new header %v to be within the trusting period, which ends at %v",
		e.HeaderTime, e.TrustingPeriodEndsAt)
}
