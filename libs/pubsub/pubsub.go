// Package pubsub implements a pub-sub model with a single publisher (Server)
// and multiple subscribers (clients).
//
// Though you can have multiple publishers by sharing a pointer to a server or
// by giving the same channel to each publisher and publishing messages from
// that channel (fan-in).
//
// Clients subscribe for messages, which could be of any type, using a query.
// When some message is published, we match it with all queries. If there is a
// match, this message will be pushed to all clients, subscribed to that query.
// See query subpackage for our implementation.
//
// Example:
//
//     q, err := query.New("account.name='John'")
//		 if err != nil {
//		     return err
//		 }
//		 ctx, cancel := context.WithTimeout(context.Background(), 1 * time.Second)
//     defer cancel()
//     subscription, err := pubsub.Subscribe(ctx, "johns-transactions", q, 1)
//     if err != nil {
//         return err
//     }
//
//     for {
//         select {
//     	   case msgAndTags <- subscription.Out():
//     		     // handle msg and tags
//     	   case <-subscription.Cancelled():
//     		     return subscription.Err()
//		  	 }
//		 }
//
package pubsub

import (
	"context"
	"errors"
	"sync"

	cmn "github.com/tendermint/tendermint/libs/common"
)

type operation int

const (
	sub operation = iota
	pub
	unsub
	shutdown
)

var (
	// ErrSubscriptionNotFound is returned when a client tries to unsubscribe
	// from not existing subscription.
	ErrSubscriptionNotFound = errors.New("subscription not found")

	// ErrAlreadySubscribed is returned when a client tries to subscribe twice or
	// more using the same query.
	ErrAlreadySubscribed = errors.New("already subscribed")
)

// Query defines an interface for a query to be used for subscribing.
type Query interface {
	Matches(tags TagMap) bool
	String() string
}

type cmd struct {
	op operation

	// subscribe, unsubscribe
	query        Query
	subscription *Subscription
	clientID     string

	// publish
	msg  interface{}
	tags TagMap
}

// Server allows clients to subscribe/unsubscribe for messages, publishing
// messages with or without tags, and manages internal state.
type Server struct {
	cmn.BaseService

	cmds    chan cmd
	cmdsCap int

	mtx           sync.RWMutex
	subscriptions map[string]map[string]struct{} // subscriber -> query (string) -> empty struct
}

// Option sets a parameter for the server.
type Option func(*Server)

// NewServer returns a new server. See the commentary on the Option functions
// for a detailed description of how to configure buffering. If no options are
// provided, the resulting server's queue is unbuffered.
func NewServer(options ...Option) *Server {
	s := &Server{
		subscriptions: make(map[string]map[string]struct{}),
	}
	s.BaseService = *cmn.NewBaseService(nil, "PubSub", s)

	for _, option := range options {
		option(s)
	}

	// if BufferCapacity option was not set, the channel is unbuffered
	s.cmds = make(chan cmd, s.cmdsCap)

	return s
}

// BufferCapacity allows you to specify capacity for the internal server's
// queue. Since the server, given Y subscribers, could only process X messages,
// this option could be used to survive spikes (e.g. high amount of
// transactions during peak hours).
func BufferCapacity(cap int) Option {
	return func(s *Server) {
		if cap > 0 {
			s.cmdsCap = cap
		}
	}
}

// BufferCapacity returns capacity of the internal server's queue.
func (s *Server) BufferCapacity() int {
	return s.cmdsCap
}

// Subscribe creates a subscription for the given client. An error will be
// returned to the caller if the context is canceled or if subscription already
// exist for pair clientID and query. outCapacity will be used to set a
// capacity for Subscription#Out channel.
func (s *Server) Subscribe(ctx context.Context, clientID string, query Query, outCapacity int) (*Subscription, error) {
	s.mtx.RLock()
	clientSubscriptions, ok := s.subscriptions[clientID]
	if ok {
		_, ok = clientSubscriptions[query.String()]
	}
	s.mtx.RUnlock()
	if ok {
		return nil, ErrAlreadySubscribed
	}

	subscription := &Subscription{
		out:       make(chan MsgAndTags, outCapacity),
		cancelled: make(chan struct{}),
	}
	select {
	case s.cmds <- cmd{op: sub, clientID: clientID, query: query, subscription: subscription}:
		s.mtx.Lock()
		if _, ok = s.subscriptions[clientID]; !ok {
			s.subscriptions[clientID] = make(map[string]struct{})
		}
		s.subscriptions[clientID][query.String()] = struct{}{}
		s.mtx.Unlock()
		return subscription, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.Quit():
		return nil, nil
	}
}

// Unsubscribe removes the subscription on the given query. An error will be
// returned to the caller if the context is canceled or if subscription does
// not exist.
func (s *Server) Unsubscribe(ctx context.Context, clientID string, query Query) error {
	s.mtx.RLock()
	clientSubscriptions, ok := s.subscriptions[clientID]
	if ok {
		_, ok = clientSubscriptions[query.String()]
	}
	s.mtx.RUnlock()
	if !ok {
		return ErrSubscriptionNotFound
	}

	select {
	case s.cmds <- cmd{op: unsub, clientID: clientID, query: query}:
		s.mtx.Lock()
		delete(clientSubscriptions, query.String())
		if len(clientSubscriptions) == 0 {
			delete(s.subscriptions, clientID)
		}
		s.mtx.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-s.Quit():
		return nil
	}
}

// UnsubscribeAll removes all client subscriptions. An error will be returned
// to the caller if the context is canceled or if subscription does not exist.
func (s *Server) UnsubscribeAll(ctx context.Context, clientID string) error {
	s.mtx.RLock()
	_, ok := s.subscriptions[clientID]
	s.mtx.RUnlock()
	if !ok {
		return ErrSubscriptionNotFound
	}

	select {
	case s.cmds <- cmd{op: unsub, clientID: clientID}:
		s.mtx.Lock()
		delete(s.subscriptions, clientID)
		s.mtx.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-s.Quit():
		return nil
	}
}

// Publish publishes the given message. An error will be returned to the caller
// if the context is canceled.
func (s *Server) Publish(ctx context.Context, msg interface{}) error {
	return s.PublishWithTags(ctx, msg, NewTagMap(make(map[string]string)))
}

// PublishWithTags publishes the given message with the set of tags. The set is
// matched with clients queries. If there is a match, the message is sent to
// the client.
func (s *Server) PublishWithTags(ctx context.Context, msg interface{}, tags TagMap) error {
	select {
	case s.cmds <- cmd{op: pub, msg: msg, tags: tags}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-s.Quit():
		return nil
	}
}

// OnStop implements Service.OnStop by shutting down the server.
func (s *Server) OnStop() {
	s.cmds <- cmd{op: shutdown}
}

// NOTE: not goroutine safe
type state struct {
	// query string -> client -> subscription
	queryToSubscriptionMap map[string]map[string]*Subscription
	// client -> query string -> struct{}
	clientToQueryMap map[string]map[string]struct{}
	// query string -> queryPlusRefCount
	queries map[string]*queryPlusRefCount
}

// queryPlusRefCount holds a pointer to a query and reference counter. When
// refCount is zero, query will be removed.
type queryPlusRefCount struct {
	q        Query
	refCount int
}

// OnStart implements Service.OnStart by starting the server.
func (s *Server) OnStart() error {
	go s.loop(state{
		queryToSubscriptionMap: make(map[string]map[string]*Subscription),
		clientToQueryMap:       make(map[string]map[string]struct{}),
		queries:                make(map[string]*queryPlusRefCount),
	})
	return nil
}

// OnReset implements Service.OnReset
func (s *Server) OnReset() error {
	return nil
}

func (s *Server) loop(state state) {
loop:
	for cmd := range s.cmds {
		switch cmd.op {
		case unsub:
			if cmd.query != nil {
				state.remove(cmd.clientID, cmd.query, ErrUnsubscribed)
			} else {
				state.removeAll(cmd.clientID, ErrUnsubscribed)
			}
		case shutdown:
			for clientID := range state.clientToQueryMap {
				state.removeAll(clientID, nil)
			}
			break loop
		case sub:
			state.add(cmd.clientID, cmd.query, cmd.subscription)
		case pub:
			state.send(cmd.msg, cmd.tags)
		}
	}
}

func (state *state) add(clientID string, q Query, subscription *Subscription) {
	qStr := q.String()

	// initialize clientToSubscriptionMap per query if needed
	if _, ok := state.queryToSubscriptionMap[qStr]; !ok {
		state.queryToSubscriptionMap[qStr] = make(map[string]*Subscription)
	}
	// create subscription
	state.queryToSubscriptionMap[qStr][clientID] = subscription

	// initialize queries if needed
	if _, ok := state.queries[qStr]; !ok {
		state.queries[qStr] = &queryPlusRefCount{q: q, refCount: 0}
	}
	// increment reference counter
	state.queries[qStr].refCount++

	// add client if needed
	if _, ok := state.clientToQueryMap[clientID]; !ok {
		state.clientToQueryMap[clientID] = make(map[string]struct{})
	}
	state.clientToQueryMap[clientID][qStr] = struct{}{}
}

func (state *state) remove(clientID string, q Query, reason error) {
	qStr := q.String()

	clientToSubscriptionMap, ok := state.queryToSubscriptionMap[qStr]
	if !ok {
		return
	}

	subscription, ok := clientToSubscriptionMap[clientID]
	if !ok {
		return
	}

	subscription.mtx.Lock()
	subscription.err = reason
	subscription.mtx.Unlock()
	close(subscription.cancelled)

	// remove the query from client map.
	// if client is not subscribed to anything else, remove it.
	delete(state.clientToQueryMap[clientID], qStr)
	if len(state.clientToQueryMap[clientID]) == 0 {
		delete(state.clientToQueryMap, clientID)
	}

	// remove the client from query map.
	// if query has no other clients subscribed, remove it.
	delete(state.queryToSubscriptionMap[qStr], clientID)
	if len(state.queryToSubscriptionMap[qStr]) == 0 {
		delete(state.queryToSubscriptionMap, qStr)
	}

	// decrease ref counter in queries
	state.queries[qStr].refCount--
	// remove the query if nobody else is using it
	if state.queries[qStr].refCount == 0 {
		delete(state.queries, qStr)
	}
}

func (state *state) removeAll(clientID string, reason error) {
	queryMap, ok := state.clientToQueryMap[clientID]
	if !ok {
		return
	}

	for qStr := range queryMap {
		subscription := state.queryToSubscriptionMap[qStr][clientID]
		subscription.mtx.Lock()
		subscription.err = reason
		subscription.mtx.Unlock()
		close(subscription.cancelled)

		// remove the client from query map.
		// if query has no other clients subscribed, remove it.
		delete(state.queryToSubscriptionMap[qStr], clientID)
		if len(state.queryToSubscriptionMap[qStr]) == 0 {
			delete(state.queryToSubscriptionMap, qStr)
		}

		// decrease ref counter in queries
		state.queries[qStr].refCount--
		// remove the query if nobody else is using it
		if state.queries[qStr].refCount == 0 {
			delete(state.queries, qStr)
		}
	}

	// remove the client.
	delete(state.clientToQueryMap, clientID)
}

func (state *state) send(msg interface{}, tags TagMap) {
	for qStr, clientToSubscriptionMap := range state.queryToSubscriptionMap {
		q := state.queries[qStr].q
		if q.Matches(tags) {
			for clientID, subscription := range clientToSubscriptionMap {
				if cap(subscription.out) == 0 {
					// block on unbuffered channel
					subscription.out <- MsgAndTags{msg, tags}
				} else {
					// don't block on buffered channels
					select {
					case subscription.out <- MsgAndTags{msg, tags}:
					default:
						state.remove(clientID, q, ErrOutOfCapacity)
					}
				}
			}
		}
	}
}
