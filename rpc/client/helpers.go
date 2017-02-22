package client

import (
	"time"

	"github.com/pkg/errors"
)

// Wait for height will poll status at reasonable intervals until
// the block at the given height is available.
func WaitForHeight(c StatusClient, h int) error {
	wait := 1
	for wait > 0 {
		s, err := c.Status()
		if err != nil {
			return err
		}
		wait = h - s.LatestBlockHeight
		if wait > 10 {
			return errors.Errorf("Waiting for %d block... aborting", wait)
		} else if wait > 0 {
			// estimate of wait time....
			// wait half a second for the next block (in progress)
			// plus one second for every full block
			delay := time.Duration(wait-1)*time.Second + 500*time.Millisecond
			time.Sleep(delay)
		}
	}
	// guess we waited long enough
	return nil
}
