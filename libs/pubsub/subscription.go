package pubsub

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/libs/queue"
	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
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
// 1) channel onto which messages and events are published
// 2) channel which is closed if a client is too slow or choose to unsubscribe
// 3) err indicating the reason for (2)
type Subscription struct {
	id    string
	out   chan Message
	queue *queue.Queue

	canceled chan struct{}
	stop     func()
	mtx      tmsync.RWMutex
	err      error
}

// newSubscription returns a new subscription with the given outCapacity.
func newSubscription(outCapacity int) (*Subscription, error) {
	sub := &Subscription{
		id:       uuid.NewString(),
		out:      make(chan Message),
		canceled: make(chan struct{}),

		// N.B. The output channel is always unbuffered. For an unbuffered
		// subscription that was already the case, and for a buffered one the
		// queue now serves as the buffer.
	}

	if outCapacity == 0 {
		sub.stop = func() { close(sub.canceled) }
		return sub, nil
	}
	q, err := queue.New(queue.Options{
		SoftQuota: outCapacity,
		HardLimit: outCapacity,
	})
	if err != nil {
		return nil, fmt.Errorf("creating queue: %w", err)
	}
	sub.queue = q
	sub.stop = func() { q.Close(); close(sub.canceled) }

	// Start a goroutine to bridge messages from the queue to the channel.
	// TODO(creachadair): This is a temporary hack until we can change the
	// interface not to expose the channel directly.
	go func() {
		for {
			next, err := q.Wait(context.Background())
			if err != nil {
				return // the subscription was terminated
			}
			sub.out <- next.(Message)
		}
	}()
	return sub, nil
}

// putMessage transmits msg to the subscriber. If s is unbuffered, this blocks
// until msg is delivered and returns nil; otherwise it reports an error if the
// queue cannot accept any further messages.
func (s *Subscription) putMessage(msg Message) error {
	if s.queue != nil {
		return s.queue.Add(msg)
	}
	s.out <- msg
	return nil
}

// Out returns a channel onto which messages and events are published.
// Unsubscribe/UnsubscribeAll does not close the channel to avoid clients from
// receiving a nil message.
func (s *Subscription) Out() <-chan Message { return s.out }

func (s *Subscription) ID() string { return s.id }

// Canceled returns a channel that's closed when the subscription is
// terminated and supposed to be used in a select statement.
func (s *Subscription) Canceled() <-chan struct{} {
	return s.canceled
}

// Err returns nil if the channel returned by Canceled is not yet closed.
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

func (s *Subscription) cancel(err error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	defer func() {
		perr := recover()
		if err == nil && perr != nil {
			err = fmt.Errorf("problem closing subscription: %v", perr)
		}
	}()

	if s.err == nil && err != nil {
		s.err = err
	}

	s.stop()
}

// Message glues data and events together.
type Message struct {
	subID  string
	data   interface{}
	events []types.Event
}

func NewMessage(subID string, data interface{}, events []types.Event) Message {
	return Message{
		subID:  subID,
		data:   data,
		events: events,
	}
}

// SubscriptionID returns the unique identifier for the subscription
// that produced this message.
func (msg Message) SubscriptionID() string { return msg.subID }

// Data returns an original data published.
func (msg Message) Data() interface{} { return msg.data }

// Events returns events, which matched the client's query.
func (msg Message) Events() []types.Event { return msg.events }
