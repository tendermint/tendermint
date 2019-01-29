package pubsub

import (
	"errors"
	"sync"
)

var (
	// ErrUnsubscribed is returned by Err when a client unsubscribes.
	ErrUnsubscribed = errors.New("client unsubscribed")

	// ErrOutOfCapacity is returned by Err when a client is not pulling messages
	// fast enough. Note the client's subscription will be terminated.
	ErrOutOfCapacity = errors.New("client is not pulling messages fast enough")
)

// A Subscription represents a client subscription for a particular query and
// consists of three things:
// 1) channel onto which messages and tags are published
// 2) channel which is closed if a client is too slow or choose to unsubscribe
// 3) err indicating the reason for (2)
type Subscription struct {
	out chan MsgAndTags

	cancelled chan struct{}
	mtx       sync.RWMutex
	err       error
}

// Out returns a channel onto which messages and tags are published.
// Unsubscribe/UnsubscribeAll does not close the channel to avoid clients from
// receiving a nil message.
func (s *Subscription) Out() <-chan MsgAndTags {
	return s.out
}

// Cancelled returns a channel that's closed when the subscription is
// terminated and supposed to be used in a select statement.
func (s *Subscription) Cancelled() <-chan struct{} {
	return s.cancelled
}

// Err returns nil if the channel returned by Cancelled is not yet closed.
// If the channel is closed, Err returns a non-nil error explaining why:
//   - ErrUnsubscribed if the subscriber choose to unsubscribe,
//   - ErrOutOfCapacity if the subscriber is not pulling messages fast enough
//   and the channel returned by Out became full,
// After Err returns a non-nil error, successive calls to Err return the same
// error.
func (s *Subscription) Err() error {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.err
}

// MsgAndTags glues a message and tags together.
type MsgAndTags struct {
	Msg  interface{}
	Tags TagMap
}
