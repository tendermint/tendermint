// Package pubsub implements an event dispatching server with a single publisher
// and multiple subscriber clients. Multiple goroutines can safely publish to a
// single Server instance.
//
// Clients register subscriptions with a query to select which messages they
// wish to receive. When messages are published, they are broadcast to all
// clients whose subscription query matches that message. Queries are
// constructed using the github.com/tendermint/tendermint/internal/pubsub/query
// package.
//
// Example:
//
//     q, err := query.New(`account.name='John'`)
//     if err != nil {
//         return err
//     }
//     sub, err := pubsub.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
//         ClientID: "johns-transactions",
//         Query:    q,
//     })
//     if err != nil {
//         return err
//     }
//
//     for {
//         next, err := sub.Next(ctx)
//         if err == pubsub.ErrTerminated {
//            return err // terminated by publisher
//         } else if err != nil {
//            return err // timed out, client unsubscribed, etc.
//         }
//         process(next)
//     }
//
package pubsub

import (
	"context"
	"errors"
	"fmt"
	"sync"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/pubsub/query"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/types"
)

var (
	// ErrSubscriptionNotFound is returned when a client tries to unsubscribe
	// from not existing subscription.
	ErrSubscriptionNotFound = errors.New("subscription not found")

	// ErrAlreadySubscribed is returned when a client tries to subscribe twice or
	// more using the same query.
	ErrAlreadySubscribed = errors.New("already subscribed")

	// ErrServerStopped is returned when attempting to publish or subscribe to a
	// server that has been stopped.
	ErrServerStopped = errors.New("pubsub server is stopped")
)

// SubscribeArgs are the parameters to create a new subscription.
type SubscribeArgs struct {
	ClientID string       // Client ID
	Query    *query.Query // filter query for events (required)
	Limit    int          // subscription queue capacity limit (0 means 1)
	Quota    int          // subscription queue soft quota (0 uses Limit)
}

// UnsubscribeArgs are the parameters to remove a subscription.
// The subscriber ID must be populated, and at least one of the client ID or
// the registered query.
type UnsubscribeArgs struct {
	Subscriber string       // subscriber ID chosen by the client (required)
	ID         string       // subscription ID (assigned by the server)
	Query      *query.Query // the query registered with the subscription
}

// Validate returns nil if args are valid to identify a subscription to remove.
// Otherwise, it reports an error.
func (args UnsubscribeArgs) Validate() error {
	if args.Subscriber == "" {
		return errors.New("must specify a subscriber")
	}
	return nil
}

// Server allows clients to subscribe/unsubscribe for messages, publishing
// messages with or without events, and manages internal state.
type Server struct {
	service.BaseService
	logger log.Logger

	queue  chan item
	done   <-chan struct{} // closed when server should exit
	pubs   sync.RWMutex    // excl: shutdown; shared: active publisher
	exited chan struct{}   // server exited

	// All subscriptions currently known.
	// Lock exclusive to add, remove, or cancel subscriptions.
	// Lock shared to look up or publish to subscriptions.
	subs struct {
		sync.RWMutex
		index *subIndex

		// This function is called synchronously with each message published
		// before it is delivered to any other subscriber. This allows an index
		// to be persisted before any subscribers see the messages.
		observe func(Message) error
	}

	// TODO(creachadair): Rework the options so that this does not need to live
	// as a field. It is not otherwise needed.
	queueCap int
}

// Option sets a parameter for the server.
type Option func(*Server)

// NewServer returns a new server. See the commentary on the Option functions
// for a detailed description of how to configure buffering. If no options are
// provided, the resulting server's queue is unbuffered.
func NewServer(logger log.Logger, options ...Option) *Server {
	s := &Server{logger: logger}

	s.BaseService = *service.NewBaseService(logger, "PubSub", s)
	for _, opt := range options {
		opt(s)
	}

	// The queue receives items to be published.
	s.queue = make(chan item, s.queueCap)

	// The index tracks subscriptions by ID and query terms.
	s.subs.index = newSubIndex()

	return s
}

// BufferCapacity allows you to specify capacity for publisher's queue.  This
// is the number of messages that can be published without blocking.  If no
// buffer is specified, publishing is synchronous with delivery.  This function
// will panic if cap < 0.
func BufferCapacity(cap int) Option {
	if cap < 0 {
		panic("negative buffer capacity")
	}
	return func(s *Server) { s.queueCap = cap }
}

// BufferCapacity returns capacity of the publication queue.
func (s *Server) BufferCapacity() int { return cap(s.queue) }

// Observe registers an observer function that will be called synchronously
// with each published message matching any of the given queries, prior to it
// being forwarded to any subscriber.  If no queries are specified, all
// messages will be observed. An error is reported if an observer is already
// registered.
func (s *Server) Observe(ctx context.Context, observe func(Message) error, queries ...*query.Query) error {
	s.subs.Lock()
	defer s.subs.Unlock()
	if observe == nil {
		return errors.New("observe callback is nil")
	} else if s.subs.observe != nil {
		return errors.New("an observer is already registered")
	}

	// Compile the message filter.
	var matches func(Message) bool
	if len(queries) == 0 {
		matches = func(Message) bool { return true }
	} else {
		matches = func(msg Message) bool {
			for _, q := range queries {
				if q.Matches(msg.events) {
					return true
				}
			}
			return false
		}
	}

	s.subs.observe = func(msg Message) error {
		if matches(msg) {
			return observe(msg)
		}
		return nil // nothing to do for this message
	}
	return nil
}

// SubscribeWithArgs creates a subscription for the given arguments.  It is an
// error if the query is nil, a subscription already exists for the specified
// client ID and query, or if the capacity arguments are invalid.
func (s *Server) SubscribeWithArgs(ctx context.Context, args SubscribeArgs) (*Subscription, error) {
	s.subs.Lock()
	defer s.subs.Unlock()

	if s.subs.index == nil {
		return nil, ErrServerStopped
	} else if s.subs.index.contains(args.ClientID, args.Query.String()) {
		return nil, ErrAlreadySubscribed
	}

	if args.Limit == 0 {
		args.Limit = 1
	}
	sub, err := newSubscription(args.Quota, args.Limit)
	if err != nil {
		return nil, err
	}
	s.subs.index.add(&subInfo{
		clientID: args.ClientID,
		query:    args.Query,
		subID:    sub.id,
		sub:      sub,
	})
	return sub, nil
}

// Unsubscribe removes the subscription for the given client and/or query.  It
// returns ErrSubscriptionNotFound if no such subscription exists.
func (s *Server) Unsubscribe(ctx context.Context, args UnsubscribeArgs) error {
	if err := args.Validate(); err != nil {
		return err
	}
	s.subs.Lock()
	defer s.subs.Unlock()
	if s.subs.index == nil {
		return ErrServerStopped
	}

	// TODO(creachadair): Do we need to support unsubscription for an "empty"
	// query?  I believe that case is not possible by the Query grammar, but we
	// should make sure.
	//
	// Revisit this logic once we are able to remove indexing by query.

	var evict subInfoSet
	if args.Subscriber != "" {
		evict = s.subs.index.findClientID(args.Subscriber)
		if args.Query != nil {
			evict = evict.withQuery(args.Query.String())
		}
	} else {
		evict = s.subs.index.findQuery(args.Query.String())
	}

	if len(evict) == 0 {
		return ErrSubscriptionNotFound
	}
	s.removeSubs(evict, ErrUnsubscribed)
	return nil
}

// UnsubscribeAll removes all subscriptions for the given client ID.
// It returns ErrSubscriptionNotFound if no subscriptions exist for that client.
func (s *Server) UnsubscribeAll(ctx context.Context, clientID string) error {
	s.subs.Lock()
	defer s.subs.Unlock()

	if s.subs.index == nil {
		return ErrServerStopped
	}
	evict := s.subs.index.findClientID(clientID)
	if len(evict) == 0 {
		return ErrSubscriptionNotFound
	}
	s.removeSubs(evict, ErrUnsubscribed)
	return nil
}

// NumClients returns the number of clients.
func (s *Server) NumClients() int {
	s.subs.RLock()
	defer s.subs.RUnlock()
	return len(s.subs.index.byClient)
}

// NumClientSubscriptions returns the number of subscriptions the client has.
func (s *Server) NumClientSubscriptions(clientID string) int {
	s.subs.RLock()
	defer s.subs.RUnlock()
	return len(s.subs.index.findClientID(clientID))
}

// Publish publishes the given message. An error will be returned to the caller
// if the pubsub server has shut down.
func (s *Server) Publish(msg types.EventData) error {
	return s.publish(msg, []abci.Event{})
}

// PublishWithEvents publishes the given message with the set of events. The set
// is matched with clients queries. If there is a match, the message is sent to
// the client.
func (s *Server) PublishWithEvents(msg types.EventData, events []abci.Event) error {
	return s.publish(msg, events)
}

// OnStop implements part of the Service interface. It is a no-op.
func (s *Server) OnStop() {}

// Wait implements Service.Wait by blocking until the server has exited, then
// yielding to the base service wait.
func (s *Server) Wait() { <-s.exited; s.BaseService.Wait() }

// OnStart implements Service.OnStart by starting the server.
func (s *Server) OnStart(ctx context.Context) error { s.run(ctx); return nil }

func (s *Server) publish(data types.EventData, events []abci.Event) error {
	s.pubs.RLock()
	defer s.pubs.RUnlock()

	select {
	case <-s.done:
		return ErrServerStopped
	case s.queue <- item{
		Data:   data,
		Events: events,
	}:
		return nil
	}
}

func (s *Server) run(ctx context.Context) {
	// The server runs until ctx is canceled.
	s.done = ctx.Done()
	queue := s.queue

	// Shutdown monitor: When the context ends, wait for any active publish
	// calls to exit, then close the queue to signal the sender to exit.
	go func() {
		<-ctx.Done()
		s.pubs.Lock()
		defer s.pubs.Unlock()
		close(s.queue)
		s.queue = nil
	}()

	s.exited = make(chan struct{})
	go func() {
		defer close(s.exited)

		// Sender: Service the queue and forward messages to subscribers.
		for it := range queue {
			if err := s.send(it.Data, it.Events); err != nil {
				s.logger.Error("error sending event", "err", err)
			}
		}
		// Terminate all subscribers before exit.
		s.subs.Lock()
		defer s.subs.Unlock()
		for si := range s.subs.index.all {
			si.sub.stop(ErrTerminated)
		}
		s.subs.index = nil
	}()
}

// removeSubs cancels and removes all the subscriptions in evict with the given
// error. The caller must hold the s.subs lock.
func (s *Server) removeSubs(evict subInfoSet, reason error) {
	for si := range evict {
		si.sub.stop(reason)
	}
	s.subs.index.removeAll(evict)
}

// send delivers the given message to all matching subscribers.  An error in
// query matching stops transmission and is returned.
func (s *Server) send(data types.EventData, events []abci.Event) error {
	// At exit, evict any subscriptions that were too slow.
	evict := make(subInfoSet)
	defer func() {
		if len(evict) != 0 {
			s.subs.Lock()
			defer s.subs.Unlock()
			s.removeSubs(evict, ErrTerminated)
		}
	}()

	// N.B. Order is important here. We must acquire and defer the lock release
	// AFTER deferring the eviction cleanup: The cleanup must happen after the
	// reader lock has released, or it will deadlock.
	s.subs.RLock()
	defer s.subs.RUnlock()

	// If an observer is defined, give it control of the message before
	// attempting to deliver it to any matching subscribers. If the observer
	// fails, the message will not be forwarded.
	if s.subs.observe != nil {
		err := s.subs.observe(Message{
			data:   data,
			events: events,
		})
		if err != nil {
			return fmt.Errorf("observer failed on message: %w", err)
		}
	}

	for si := range s.subs.index.all {
		if !si.query.Matches(events) {
			continue
		}

		// Publish the events to the subscriber's queue. If this fails, e.g.,
		// because the queue is over capacity or out of quota, evict the
		// subscription from the index.
		if err := si.sub.publish(Message{
			subID:  si.sub.id,
			data:   data,
			events: events,
		}); err != nil {
			evict.add(si)
		}
	}

	return nil
}
