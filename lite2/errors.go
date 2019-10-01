package lite

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
)

type errNewHeaderTooFarIntoFuture struct {
	headerTime           time.Time
	trustingPeriodEndsAt time.Time
}

func (e errNewHeaderTooFarIntoFuture) Error() string {
	return fmt.Sprintf("expected new header %v to be within the trusting period, which ends at %v",
		e.headerTime, e.trustingPeriodEndsAt)
}

// IsErrNewHeaderTooFarIntoFuture returns true if err is related to
// new header having timestamp after trusting period end.
func IsErrNewHeaderTooFarIntoFuture(err error) bool {
	_, ok := errors.Cause(err).(errNewHeaderTooFarIntoFuture)
	return ok
}
