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

	cmn "github.com/tendermint/tmlibs/common"
)

type operation int

const (
	sub operation = iota
	pub
	unsub
	shutdown
)

type cmd struct {
	op       operation
	query    Query
	ch       chan<- interface{}
	clientID string
	msg      interface{}
	tags     map[string]interface{}
}

// Query defines an interface for a query to be used for subscribing.
type Query interface {
	Matches(tags map[string]interface{}) bool
	String() string
}

// Server allows clients to subscribe/unsubscribe for messages, publishing
// messages with or without tags, and manages internal state.
type Server struct {
	cmn.BaseService

	cmds    chan cmd
	cmdsCap int

	mtx           sync.RWMutex
	subscriptions map[string]map[string]struct{} // subscriber -> query -> struct{}
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
		return errors.New("already subscribed")
	}

	select {
	case s.cmds <- cmd{op: sub, clientID: clientID, query: query, ch: out}:
		s.mtx.Lock()
		if _, ok = s.subscriptions[clientID]; !ok {
			s.subscriptions[clientID] = make(map[string]struct{})
		}
		s.subscriptions[clientID][query.String()] = struct{}{}
		s.mtx.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
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
		return errors.New("subscription not found")
	}

	select {
	case s.cmds <- cmd{op: unsub, clientID: clientID, query: query}:
		s.mtx.Lock()
		delete(clientSubscriptions, query.String())
		s.mtx.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// UnsubscribeAll removes all client subscriptions. An error will be returned
// to the caller if the context is canceled or if subscription does not exist.
func (s *Server) UnsubscribeAll(ctx context.Context, clientID string) error {
	s.mtx.RLock()
	_, ok := s.subscriptions[clientID]
	s.mtx.RUnlock()
	if !ok {
		return errors.New("subscription not found")
	}

	select {
	case s.cmds <- cmd{op: unsub, clientID: clientID}:
		s.mtx.Lock()
		delete(s.subscriptions, clientID)
		s.mtx.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Publish publishes the given message. An error will be returned to the caller
// if the context is canceled.
func (s *Server) Publish(ctx context.Context, msg interface{}) error {
	return s.PublishWithTags(ctx, msg, make(map[string]interface{}))
}

// PublishWithTags publishes the given message with the set of tags. The set is
// matched with clients queries. If there is a match, the message is sent to
// the client.
func (s *Server) PublishWithTags(ctx context.Context, msg interface{}, tags map[string]interface{}) error {
	select {
	case s.cmds <- cmd{op: pub, msg: msg, tags: tags}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
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
	// add query if needed
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
	}

	delete(state.clients, clientID)
}

func (state *state) send(msg interface{}, tags map[string]interface{}) {
	for q, clientToChannelMap := range state.queries {
		if q.Matches(tags) {
			for _, ch := range clientToChannelMap {
				ch <- msg
			}
		}
	}
}
