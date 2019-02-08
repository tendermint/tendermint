package types

import (
	"fmt"
	"time"

	"github.com/tendermint/tendermint/crypto/tmhash"
	tmtime "github.com/tendermint/tendermint/types/time"
)

// ValidateTime does a basic time validation ensuring time does not drift too
// much: +/- one year.
// TODO: reduce this to eg 1 day
// NOTE: DO NOT USE in ValidateBasic methods in this package. This function
// can only be used for real time validation, like on proposals and votes
// in the consensus. If consensus is stuck, and rounds increase for more than a day,
// having only a 1-day band here could break things...
// Can't use for validating blocks because we may be syncing years worth of history.
func ValidateTime(t time.Time) error {
	var (
		now     = tmtime.Now()
		oneYear = 8766 * time.Hour
	)
	if t.Before(now.Add(-oneYear)) || t.After(now.Add(oneYear)) {
		return fmt.Errorf("Time drifted too much. Expected: -1 < %v < 1 year", now)
	}
	return nil
}

// ValidateHash returns an error if the hash is not empty, but its
// size != tmhash.Size.
func ValidateHash(h []byte) error {
	if len(h) > 0 && len(h) != tmhash.Size {
		return fmt.Errorf("Expected size to be %d bytes, got %d bytes",
			tmhash.Size,
			len(h),
		)
	}
	return nil
}
