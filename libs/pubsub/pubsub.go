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

type cmd struct {
	op       operation
	query    Query
	ch       chan<- interface{}
	clientID string
	msg      interface{}
	tags     TagMap
}

// Query defines an interface for a query to be used for subscribing.
type Query interface {
	Matches(tags TagMap) bool
	String() string
}

// Server allows clients to subscribe/unsubscribe for messages, publishing
// messages with or without tags, and manages internal state.
type Server struct {
	cmn.BaseService

	cmds    chan cmd
	cmdsCap int

	mtx           sync.RWMutex
	subscriptions map[string]map[string]Query // subscriber -> query (string) -> Query
}

// Option sets a parameter for the server.
type Option func(*Server)

// TagMap is used to associate tags to a message.
// They can be queried by subscribers to choose messages they will received.
type TagMap interface {
	// Get returns the value for a key, or nil if no value is present.
	// The ok result indicates whether value was found in the tags.
	Get(key string) (value string, ok bool)
	// Len returns the number of tags.
	Len() int
}

type tagMap map[string]string

var _ TagMap = (*tagMap)(nil)

// NewTagMap constructs a new immutable tag set from a map.
func NewTagMap(data map[string]string) TagMap {
	return tagMap(data)
}

// Get returns the value for a key, or nil if no value is present.
// The ok result indicates whether value was found in the tags.
func (ts tagMap) Get(key string) (value string, ok bool) {
	value, ok = ts[key]
	return
}

// Len returns the number of tags.
func (ts tagMap) Len() int {
	return len(ts)
}

// NewServer returns a new server. See the commentary on the Option functions
// for a detailed description of how to configure buffering. If no options are
// provided, the resulting server's queue is unbuffered.
func NewServer(options ...Option) *Server {
	s := &Server{
		subscriptions: make(map[string]map[string]Query),
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

// Subscribe creates a subscription for the given client. It accepts a channel
// on which messages matching the given query can be received. An error will be
// returned to the caller if the context is canceled or if subscription already
// exist for pair clientID and query.
func (s *Server) Subscribe(ctx context.Context, clientID string, query Query, out chan<- interface{}) error {
	s.mtx.RLock()
	clientSubscriptions, ok := s.subscriptions[clientID]
	if ok {
		_, ok = clientSubscriptions[query.String()]
	}
	s.mtx.RUnlock()
	if ok {
		return ErrAlreadySubscribed
	}

	select {
	case s.cmds <- cmd{op: sub, clientID: clientID, query: query, ch: out}:
		s.mtx.Lock()
		if _, ok = s.subscriptions[clientID]; !ok {
			s.subscriptions[clientID] = make(map[string]Query)
		}
		// preserve original query
		// see Unsubscribe
		s.subscriptions[clientID][query.String()] = query
		s.mtx.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-s.Quit():
		return nil
	}
}

// Unsubscribe removes the subscription on the given query. An error will be
// returned to the caller if the context is canceled or if subscription does
// not exist.
func (s *Server) Unsubscribe(ctx context.Context, clientID string, query Query) error {
	var origQuery Query
	s.mtx.RLock()
	clientSubscriptions, ok := s.subscriptions[clientID]
	if ok {
		origQuery, ok = clientSubscriptions[query.String()]
	}
	s.mtx.RUnlock()
	if !ok {
		return ErrSubscriptionNotFound
	}

	// original query is used here because we're using pointers as map keys
	select {
	case s.cmds <- cmd{op: unsub, clientID: clientID, query: origQuery}:
		s.mtx.Lock()
		delete(clientSubscriptions, query.String())
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
	// query -> client -> ch
	queries map[Query]map[string]chan<- interface{}
	// client -> query -> struct{}
	clients map[string]map[Query]struct{}
}

// OnStart implements Service.OnStart by starting the server.
func (s *Server) OnStart() error {
	go s.loop(state{
		queries: make(map[Query]map[string]chan<- interface{}),
		clients: make(map[string]map[Query]struct{}),
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
				state.remove(cmd.clientID, cmd.query)
			} else {
				state.removeAll(cmd.clientID)
			}
		case shutdown:
			for clientID := range state.clients {
				state.removeAll(clientID)
			}
			break loop
		case sub:
			state.add(cmd.clientID, cmd.query, cmd.ch)
		case pub:
			state.send(cmd.msg, cmd.tags)
		}
	}
}

func (state *state) add(clientID string, q Query, ch chan<- interface{}) {

	// initialize clientToChannelMap per query if needed
	if _, ok := state.queries[q]; !ok {
		state.queries[q] = make(map[string]chan<- interface{})
	}

	// create subscription
	state.queries[q][clientID] = ch

	// add client if needed
	if _, ok := state.clients[clientID]; !ok {
		state.clients[clientID] = make(map[Query]struct{})
	}
	state.clients[clientID][q] = struct{}{}
}

func (state *state) remove(clientID string, q Query) {
	clientToChannelMap, ok := state.queries[q]
	if !ok {
		return
	}

	ch, ok := clientToChannelMap[clientID]
	if ok {
		close(ch)

		delete(state.clients[clientID], q)

		// if it not subscribed to anything else, remove the client
		if len(state.clients[clientID]) == 0 {
			delete(state.clients, clientID)
		}

		delete(state.queries[q], clientID)
		if len(state.queries[q]) == 0 {
			delete(state.queries, q)
		}
	}
}

func (state *state) removeAll(clientID string) {
	queryMap, ok := state.clients[clientID]
	if !ok {
		return
	}

	for q := range queryMap {
		ch := state.queries[q][clientID]
		close(ch)

		delete(state.queries[q], clientID)
		if len(state.queries[q]) == 0 {
			delete(state.queries, q)
		}
	}
	delete(state.clients, clientID)
}

func (state *state) send(msg interface{}, tags TagMap) {
	for q, clientToChannelMap := range state.queries {
		if q.Matches(tags) {
			for _, ch := range clientToChannelMap {
				ch <- msg
			}
		}
	}
}
