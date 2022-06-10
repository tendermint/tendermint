package pubsub

import (
	"context"
	"errors"

	"github.com/google/uuid"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/libs/queue"
	"github.com/tendermint/tendermint/types"
)

var (
	// ErrUnsubscribed is returned by Next when the client has unsubscribed.
	ErrUnsubscribed = errors.New("subscription removed by client")

	// ErrTerminated is returned by Next when the subscription was terminated by
	// the publisher.
	ErrTerminated = errors.New("subscription terminated by publisher")
)

// A Subscription represents a client subscription for a particular query.
type Subscription struct {
	id      string
	queue   *queue.Queue // open until the subscription ends
	stopErr error        // after queue is closed, the reason why
}

// newSubscription returns a new subscription with the given queue capacity.
func newSubscription(quota, limit int) (*Subscription, error) {
	queue, err := queue.New(queue.Options{
		SoftQuota: quota,
		HardLimit: limit,
	})
	if err != nil {
		return nil, err
	}
	return &Subscription{
		id:    uuid.NewString(),
		queue: queue,
	}, nil
}

// Next blocks until a message is available, ctx ends, or the subscription
// ends.  Next returns ErrUnsubscribed if s was unsubscribed, ErrTerminated if
// s was terminated by the publisher, or a context error if ctx ended without a
// message being available.
func (s *Subscription) Next(ctx context.Context) (Message, error) {
	next, err := s.queue.Wait(ctx)
	if errors.Is(err, queue.ErrQueueClosed) {
		return Message{}, s.stopErr
	} else if err != nil {
		return Message{}, err
	}
	return next.(Message), nil
}

// ID returns the unique subscription identifier for s.
func (s *Subscription) ID() string { return s.id }

// publish transmits msg to the subscriber. It reports a queue error if the
// queue cannot accept any further messages.
func (s *Subscription) publish(msg Message) error { return s.queue.Add(msg) }

// stop terminates the subscription with the given error reason.
func (s *Subscription) stop(err error) {
	if err == nil {
		panic("nil stop error")
	}
	s.stopErr = err
	s.queue.Close()
}

// Message glues data and events together.
type Message struct {
	subID  string
	data   types.EventData
	events []abci.Event
}

// SubscriptionID returns the unique identifier for the subscription
// that produced this message.
func (msg Message) SubscriptionID() string { return msg.subID }

// Data returns an original data published.
func (msg Message) Data() types.EventData { return msg.data }

// Events returns events, which matched the client's query.
func (msg Message) Events() []abci.Event { return msg.events }
