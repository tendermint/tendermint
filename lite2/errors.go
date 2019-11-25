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
