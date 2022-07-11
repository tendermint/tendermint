package p2p

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/gogo/protobuf/proto"

	"github.com/tendermint/tendermint/types"
)

// Envelope contains a message with sender/receiver routing info.
type Envelope struct {
	From      types.NodeID  // sender (empty if outbound)
	To        types.NodeID  // receiver (empty if inbound)
	Broadcast bool          // send to all connected peers (ignores To)
	Message   proto.Message // message payload
	ChannelID ChannelID
}

func (e Envelope) IsZero() bool {
	return e.From == "" && e.To == "" && e.Message == nil
}

// Wrapper is a Protobuf message that can contain a variety of inner messages
// (e.g. via oneof fields). If a Channel's message type implements Wrapper, the
// Router will automatically wrap outbound messages and unwrap inbound messages,
// such that reactors do not have to do this themselves.
type Wrapper interface {
	proto.Message

	// Wrap will take a message and wrap it in this one if possible.
	Wrap(proto.Message) error

	// Unwrap will unwrap the inner message contained in this message.
	Unwrap() (proto.Message, error)
}

type Channel interface {
	fmt.Stringer

	Err() error

	Send(context.Context, Envelope) error
	SendError(context.Context, PeerError) error
	Receive(context.Context) *ChannelIterator
}

// PeerError is a peer error reported via Channel.Error.
//
// FIXME: This currently just disconnects the peer, which is too simplistic.
// For example, some errors should be logged, some should cause disconnects,
// and some should ban the peer.
//
// FIXME: This should probably be replaced by a more general PeerBehavior
// concept that can mark good and bad behavior and contributes to peer scoring.
// It should possibly also allow reactors to request explicit actions, e.g.
// disconnection or banning, in addition to doing this based on aggregates.
type PeerError struct {
	NodeID types.NodeID
	Err    error
	Fatal  bool
}

func (pe PeerError) Error() string { return fmt.Sprintf("peer=%q: %s", pe.NodeID, pe.Err.Error()) }
func (pe PeerError) Unwrap() error { return pe.Err }

// legacyChannel is a bidirectional channel to exchange Protobuf messages with peers.
// Each message is wrapped in an Envelope to specify its sender and receiver.
type legacyChannel struct {
	ID    ChannelID
	inCh  <-chan Envelope  // inbound messages (peers to reactors)
	outCh chan<- Envelope  // outbound messages (reactors to peers)
	errCh chan<- PeerError // peer error reporting

	name string
}

// NewChannel creates a new channel. It is primarily for internal and test
// use, reactors should use Router.OpenChannel().
func NewChannel(id ChannelID, name string, inCh <-chan Envelope, outCh chan<- Envelope, errCh chan<- PeerError) Channel {
	return &legacyChannel{
		ID:    id,
		name:  name,
		inCh:  inCh,
		outCh: outCh,
		errCh: errCh,
	}
}

// Send blocks until the envelope has been sent, or until ctx ends.
// An error only occurs if the context ends before the send completes.
func (ch *legacyChannel) Send(ctx context.Context, envelope Envelope) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case ch.outCh <- envelope:
		return nil
	}
}

func (ch *legacyChannel) Err() error { return nil }

// SendError blocks until the given error has been sent, or ctx ends.
// An error only occurs if the context ends before the send completes.
func (ch *legacyChannel) SendError(ctx context.Context, pe PeerError) error {
	if errors.Is(pe.Err, context.Canceled) || errors.Is(pe.Err, context.DeadlineExceeded) {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case ch.errCh <- pe:
		return nil
	}
}

func (ch *legacyChannel) String() string { return fmt.Sprintf("p2p.Channel<%d:%s>", ch.ID, ch.name) }

// Receive returns a new unbuffered iterator to receive messages from ch.
// The iterator runs until ctx ends.
func (ch *legacyChannel) Receive(ctx context.Context) *ChannelIterator {
	iter := &ChannelIterator{
		pipe: make(chan Envelope), // unbuffered
	}
	go func(pipe chan<- Envelope) {
		defer close(iter.pipe)
		for {
			select {
			case <-ctx.Done():
				return
			case envelope := <-ch.inCh:
				select {
				case <-ctx.Done():
					return
				case pipe <- envelope:
				}
			}
		}
	}(iter.pipe)
	return iter
}

// ChannelIterator provides a context-aware path for callers
// (reactors) to process messages from the P2P layer without relying
// on the implementation details of the P2P layer. Channel provides
// access to it's Outbound stream as an iterator, and the
// MergedChannelIterator makes it possible to combine multiple
// channels into a single iterator.
type ChannelIterator struct {
	pipe    chan Envelope
	current *Envelope
}

// Next returns true when the Envelope value has advanced, and false
// when the context is canceled or iteration should stop. If an iterator has returned false,
// it will never return true again.
// in general, use Next, as in:
//
//     for iter.Next(ctx) {
//          envelope := iter.Envelope()
//          // ... do things ...
//     }
//
func (iter *ChannelIterator) Next(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		iter.current = nil
		return false
	case envelope, ok := <-iter.pipe:
		if !ok {
			iter.current = nil
			return false
		}

		iter.current = &envelope

		return true
	}
}

// Envelope returns the current Envelope object held by the
// iterator. When the last call to Next returned true, Envelope will
// return a non-nil object. If Next returned false then Envelope is
// always nil.
func (iter *ChannelIterator) Envelope() *Envelope { return iter.current }

// MergedChannelIterator produces an iterator that merges the
// messages from the given channels in arbitrary order.
//
// This allows the caller to consume messages from multiple channels
// without needing to manage the concurrency separately.
func MergedChannelIterator(ctx context.Context, chs ...Channel) *ChannelIterator {
	iter := &ChannelIterator{
		pipe: make(chan Envelope), // unbuffered
	}
	wg := new(sync.WaitGroup)

	for _, ch := range chs {
		wg.Add(1)
		go func(ch Channel, pipe chan<- Envelope) {
			defer wg.Done()
			iter := ch.Receive(ctx)
			for iter.Next(ctx) {
				select {
				case <-ctx.Done():
					return
				case pipe <- *iter.Envelope():
				}
			}
		}(ch, iter.pipe)
	}

	done := make(chan struct{})
	go func() { defer close(done); wg.Wait() }()

	go func() {
		defer close(iter.pipe)
		// we could return early if the context is canceled,
		// but this is safer because it means the pipe stays
		// open until all of the ch worker threads end, which
		// should happen very quickly.
		<-done
	}()

	return iter
}
