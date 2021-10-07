package client

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tendermint/tendermint/libs/log"
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
func WaitForHeight(c StatusClient, h int64, waiter Waiter) error {
	if waiter == nil {
		waiter = DefaultWaitStrategy
	}
	delta := int64(1)
	for delta > 0 {
		s, err := c.Status(context.Background())
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

// WaitForOneEvent subscribes to a websocket event for the given
// event time and returns upon receiving it one time, or
// when the timeout duration has expired.
//
// This handles subscribing and unsubscribing under the hood
func WaitForOneEvent(c EventsClient, eventValue string, timeout time.Duration) (types.TMEventData, error) {
	const subscriber = "helpers"
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// register for the next event of this type
	eventCh, err := c.Subscribe(ctx, subscriber, types.QueryForEvent(eventValue).String())
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	// make sure to un-register after the test is over
	defer func() {
		if deferErr := c.UnsubscribeAll(ctx, subscriber); deferErr != nil {
			panic(err)
		}
	}()

	select {
	case event := <-eventCh:
		return event.Data, nil
	case <-ctx.Done():
		return nil, errors.New("timed out waiting for event")
	}
}

var (
	// ErrClientRunning is returned by Start when the client is already running.
	ErrClientRunning = errors.New("client already running")

	// ErrClientNotRunning is returned by Stop when the client is not running.
	ErrClientNotRunning = errors.New("client is not running")
)

// RunState is a helper that a client implementation can embed to implement
// common plumbing for keeping track of run state and logging.
//
// TODO(creachadair): This type is a temporary measure, and will be removed.
// See the discussion on #6971.
type RunState struct {
	Logger log.Logger

	mu        sync.Mutex
	name      string
	isRunning bool
	quit      chan struct{}
}

// NewRunState returns a new unstarted run state tracker with the given logging
// label and log sink. If logger == nil, a no-op logger is provided by default.
func NewRunState(name string, logger log.Logger) *RunState {
	if logger == nil {
		logger = log.NewNopLogger()
	}
	return &RunState{
		name:   name,
		Logger: logger,
	}
}

// Start sets the state to running, or reports an error.
func (r *RunState) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.isRunning {
		r.Logger.Error("not starting client, it is already started", "client", r.name)
		return ErrClientRunning
	}
	r.Logger.Info("starting client", "client", r.name)
	r.isRunning = true
	r.quit = make(chan struct{})
	return nil
}

// Stop sets the state to not running, or reports an error.
func (r *RunState) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.isRunning {
		r.Logger.Error("not stopping client; it is already stopped", "client", r.name)
		return ErrClientNotRunning
	}
	r.Logger.Info("stopping client", "client", r.name)
	r.isRunning = false
	close(r.quit)
	return nil
}

// SetLogger updates the log sink.
func (r *RunState) SetLogger(logger log.Logger) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Logger = logger
}

// IsRunning reports whether the state is running.
func (r *RunState) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.isRunning
}

// Quit returns a channel that is closed when a call to Stop succeeds.
func (r *RunState) Quit() <-chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.quit
}
