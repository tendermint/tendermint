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
// Overflow strategies (incoming publish requests):
//
// 1) drop - drops publish requests when there are too many of them
// 2) wait - blocks until the server is ready to accept more publish requests (default)
//
// Subscribe/Unsubscribe calls are always blocking.
//
// Overflow strategies (outgoing messages):
//
// 1) skip - do not send a message if the client is busy or slow (default)
// 2) wait - wait until the client is ready to accept new messages
//
package pubsub

import (
	"errors"
	"fmt"

	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"
)

type operation int

const (
	sub operation = iota
	pub
	unsub
	shutdown
)

type overflowStrategy int

const (
	drop overflowStrategy = iota
	wait
)

var (
	ErrorOverflow = errors.New("Server overflowed")
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
}

// Server allows clients to subscribe/unsubscribe for messages, pubsling
// messages with or without tags, and manages internal state.
type Server struct {
	cmn.BaseService

	cmds chan cmd

	overflowStrategy   overflowStrategy
	slowClientStrategy overflowStrategy
}

// Option sets a parameter for the server.
type Option func(*Server)

// NewServer returns a new server. See the commentary on the Option functions
// for a detailed description of how to configure buffering and overflow
// behavior. If no options are provided, the resulting server's queue is
// unbuffered and it blocks when overflowed.
func NewServer(options ...Option) *Server {
	s := &Server{overflowStrategy: wait, slowClientStrategy: drop}
	s.BaseService = *cmn.NewBaseService(nil, "PubSub", s)

	for _, option := range options {
		option(s)
	}

	if s.cmds == nil { // if BufferCapacity was not set, create unbuffered channel
		s.cmds = make(chan cmd)
	}

	return s
}

// BufferCapacity allows you to specify capacity for the internal server's
// queue. Since the server, given Y subscribers, could only process X messages,
// this option could be used to survive spikes (e.g. high amount of
// transactions during peak hours).
func BufferCapacity(cap int) Option {
	return func(s *Server) {
		if cap > 0 {
			s.cmds = make(chan cmd, cap)
		}
	}
}

// OverflowStrategyDrop will tell the server to drop messages when it can't
// process more messages.
func OverflowStrategyDrop() Option {
	return func(s *Server) {
		s.overflowStrategy = drop
	}
}

// OverflowStrategyWait will tell the server to block and wait for some time
// for server to process other messages. Default strategy.
func OverflowStrategyWait() func(*Server) {
	return func(s *Server) {
		s.overflowStrategy = wait
	}
}

// WaitSlowClients will tell the server to block and wait until subscriber
// reads a messages even if it is fast enough to process them.
func WaitSlowClients() func(*Server) {
	return func(s *Server) {
		s.slowClientStrategy = wait
	}
}

// SkipSlowClients will tell the server to skip subscriber if it is busy
// processing previous message(s). Default strategy.
func SkipSlowClients() func(*Server) {
	return func(s *Server) {
		s.slowClientStrategy = drop
	}
}

// Subscribe returns a channel on which messages matching the given query can
// be received. If the subscription already exists old channel will be closed
// and new one returned.
func (s *Server) Subscribe(clientID string, query Query, out chan<- interface{}) {
	s.cmds <- cmd{op: sub, clientID: clientID, query: query, ch: out}
}

// Unsubscribe unsubscribes the given client from the query.
func (s *Server) Unsubscribe(clientID string, query Query) {
	s.cmds <- cmd{op: unsub, clientID: clientID, query: query}
}

// Unsubscribe unsubscribes the given channel.
func (s *Server) UnsubscribeAll(clientID string) {
	s.cmds <- cmd{op: unsub, clientID: clientID}
}

// Publish publishes the given message.
func (s *Server) Publish(msg interface{}) error {
	return s.PublishWithTags(msg, make(map[string]interface{}))
}

// PublishWithTags publishes the given message with a set of tags. This set of
// tags will be matched with client queries. If there is a match, the message
// will be sent to a client.
func (s *Server) PublishWithTags(msg interface{}, tags map[string]interface{}) error {
	pubCmd := cmd{op: pub, msg: msg, tags: tags}
	switch s.overflowStrategy {
	case drop:
		select {
		case s.cmds <- pubCmd:
		default:
			s.Logger.Error("Server overflowed, dropping message...", "msg", msg, "tags", fmt.Sprintf("%v", tags))
			return ErrorOverflow
		}
	case wait:
		s.cmds <- pubCmd
	}
	return nil
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

// OnStart implements Service.OnStart by creating a main loop.
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
			for clientID, _ := range state.clients {
				state.removeAll(clientID)
			}
			break loop
		case sub:
			state.add(cmd.clientID, cmd.query, cmd.ch)
		case pub:
			state.send(cmd.msg, cmd.tags, s.slowClientStrategy, s.Logger)
		}
	}
}

func (state *state) add(clientID string, q Query, ch chan<- interface{}) {
	// add query if needed
	if clientToChannelMap, ok := state.queries[q]; !ok {
		state.queries[q] = make(map[string]chan<- interface{})
	} else {
		// check if already subscribed
		if oldCh, ok := clientToChannelMap[clientID]; ok {
			close(oldCh)
		}
	}
	state.queries[q][clientID] = ch

	// add client if needed
	if _, ok := state.clients[clientID]; !ok {
		state.clients[clientID] = make(map[Query]struct{})
	}
	state.clients[clientID][q] = struct{}{}

	// create subscription
	clientToChannelMap := state.queries[q]
	clientToChannelMap[clientID] = ch
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

	for q, _ := range queryMap {
		ch := state.queries[q][clientID]
		close(ch)

		delete(state.queries[q], clientID)
	}

	delete(state.clients, clientID)
}

func (state *state) send(msg interface{}, tags map[string]interface{}, slowClientStrategy overflowStrategy, logger log.Logger) {
	for q, clientToChannelMap := range state.queries {
		if q.Matches(tags) {
			for clientID, ch := range clientToChannelMap {
				switch slowClientStrategy {
				case drop:
					select {
					case ch <- msg:
					default:
						logger.Error("Wanted to send a message, but the client is busy", "msg", msg, "tags", fmt.Sprintf("%v", tags), "clientID", clientID)
					}
				case wait:
					ch <- msg
				}
			}
		}
	}
}
