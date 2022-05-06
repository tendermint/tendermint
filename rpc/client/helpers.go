package client

import (
	"context"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/internal/jsontypes"
	"github.com/tendermint/tendermint/rpc/coretypes"
	"github.com/tendermint/tendermint/types"
)

// Waiter is informed of current height, decided whether to quit early
type Waiter func(delta int64) (abort error)

// DefaultWaitStrategy is the standard backoff algorithm,
// but you can plug in another one
func DefaultWaitStrategy(delta int64) (abort error) {
	if delta > 10 {
		return fmt.Errorf("waiting for %d blocks... aborting", delta)
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
func WaitForHeight(ctx context.Context, c StatusClient, h int64, waiter Waiter) error {
	if waiter == nil {
		waiter = DefaultWaitStrategy
	}
	delta := int64(1)
	for delta > 0 {
		s, err := c.Status(ctx)
		if err != nil {
			return err
		}
		delta = h - s.SyncInfo.LatestBlockHeight
		// wait for the time, or abort early
		if err := waiter(delta); err != nil {
			return err
		}
	}

	return nil
}

// WaitForOneEvent waits for the first event matching the given query on c, or
// until ctx ends. It reports an error if ctx ends before a matching event is
// received.
func WaitForOneEvent(ctx context.Context, c EventsClient, query string) (types.EventData, error) {
	for {
		rsp, err := c.Events(ctx, &coretypes.RequestEvents{
			Filter:   &coretypes.EventFilter{Query: query},
			MaxItems: 1,
			WaitTime: 10 * time.Second, // duration doesn't matter, limited by ctx timeout
		})
		if err != nil {
			return nil, err
		} else if len(rsp.Items) == 0 {
			continue // continue polling until ctx expires
		}
		var result types.EventData
		if err := jsontypes.Unmarshal(rsp.Items[0].Data, &result); err != nil {
			return nil, err
		}
		return result, nil
	}
}
