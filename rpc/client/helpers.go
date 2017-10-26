package client

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/types"
)

// Waiter is informed of current height, decided whether to quit early
type Waiter func(delta int) (abort error)

// DefaultWaitStrategy is the standard backoff algorithm,
// but you can plug in another one
func DefaultWaitStrategy(delta int) (abort error) {
	if delta > 10 {
		return errors.Errorf("Waiting for %d blocks... aborting", delta)
	} else if delta > 0 {
		// estimate of wait time....
		// wait half a second for the next block (in progress)
		// plus one second for every full block
		delay := time.Duration(delta-1)*time.Second + 500*time.Millisecond
		time.Sleep(delay)
	}
	return nil
}

// Wait for height will poll status at reasonable intervals until
// the block at the given height is available.
//
// If waiter is nil, we use DefaultWaitStrategy, but you can also
// provide your own implementation
func WaitForHeight(c StatusClient, h int, waiter Waiter) error {
	if waiter == nil {
		waiter = DefaultWaitStrategy
	}
	delta := 1
	for delta > 0 {
		s, err := c.Status()
		if err != nil {
			return err
		}
		delta = h - s.LatestBlockHeight
		// wait for the time, or abort early
		if err := waiter(delta); err != nil {
			return err
		}
	}
	return nil
}

// WaitForOneEvent subscribes to a websocket event for the given
// event time and returns upon receiving it one time, or
// when the timeout duration has expired.
//
// This handles subscribing and unsubscribing under the hood
func WaitForOneEvent(c EventsClient, evtTyp string, timeout time.Duration) (types.TMEventData, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	evts := make(chan interface{}, 1)

	// register for the next event of this type
	query := fmt.Sprintf("%s='%s'", types.EventTypeKey, evtTyp)
	err := c.Subscribe(ctx, query, evts)
	if err != nil {
		return types.TMEventData{}, errors.Wrap(err, "failed to subscribe")
	}

	// make sure to unregister after the test is over
	defer c.Unsubscribe(ctx, query)

	select {
	case evt := <-evts:
		return evt.(types.TMEventData), nil
	case <-ctx.Done():
		return types.TMEventData{}, errors.New("timed out waiting for event")
	}
}
