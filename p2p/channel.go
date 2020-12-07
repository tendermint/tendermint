package p2p

import (
	"sync"

	"github.com/gogo/protobuf/proto"
)

// ChannelID is an arbitrary channel ID.
type ChannelID uint16

// Envelope specifies the message receiver and sender.
type Envelope struct {
	From      PeerID        // Message sender, or empty for outbound messages.
	To        PeerID        // Message receiver, or empty for inbound messages.
	Broadcast bool          // Send message to all connected peers, ignoring To.
	Message   proto.Message // Payload.
}

// Channel is a bidirectional channel for Protobuf message exchange with peers.
//
// Channel is safe for concurrent use by multiple goroutines.
type Channel struct {
	closeOnce sync.Once

	// ID contains the channel ID.
	id ChannelID

	// messageType specifies the type of messages exchanged via the channel, and
	// is used e.g. for automatic unmarshaling.
	messageType proto.Message

	// In is a channel for receiving inbound messages. Envelope.From is always
	// set.
	inCh chan Envelope

	// Out is a channel for sending outbound messages. Envelope.To or Broadcast
	// must be set, otherwise the message is discarded.
	outCh chan Envelope

	// Error is a channel for reporting peer errors to the router, typically used
	// when peers send an invalid or malignant message.
	errCh chan PeerError

	// doneCh is used to signal that a Channel is closed. A Channel is bi-directional
	// and should be closed by the reactor, where as the router is responsible
	// for explicitly closing the internal In channel.
	doneCh chan struct{}
}

// NewChannel returns a reference to a new p2p Channel. It is the reactor's
// responsibility to close the Channel. After a channel is closed, the router may
// safely and explicitly close the internal In channel.
func NewChannel(id ChannelID, mType proto.Message, in, out chan Envelope, err chan PeerError) *Channel {
	return &Channel{
		id:          id,
		messageType: mType,
		inCh:        in,
		outCh:       out,
		errCh:       err,
		doneCh:      make(chan struct{}),
	}
}

// ID returns the Channel's ID.
func (c *Channel) ID() ChannelID {
	return c.id
}

// In returns a read-only inbound go channel. This go channel should be used by
// reactors to consume Envelopes sent from peers.
func (c *Channel) In() <-chan Envelope {
	return c.inCh
}

// Out returns a write-only outbound go channel. This go channel should be used
// by reactors to route Envelopes to other peers.
func (c *Channel) Out() chan<- Envelope {
	return c.outCh
}

// Error returns a write-only outbound go channel designated for peer errors only.
// This go channel should be used by reactors to send peer errors when consuming
// Envelopes sent from other peers.
func (c *Channel) Error() chan<- PeerError {
	return c.errCh
}

// Close closes the outbound channel and marks the Channel as done. Internally,
// the outbound Out and peer error Error channels are closed. It is the reactor's
// responsibility to invoke Close. After a channel is closed, the router may
// safely and explicitly close the internal In channel. Any send on the Out or
// Error channel will panic after the Channel is closed.
func (c *Channel) Close() error {
	c.closeOnce.Do(func() {
		close(c.doneCh)
		close(c.outCh)
		close(c.errCh)
	})

	return nil
}

// Done returns the Channel's internal channel that should be used by a router
// to signal when it is safe to explicitly close the internal In channel.
func (c *Channel) Done() <-chan struct{} {
	return c.doneCh
}

// Wrapper is a Protobuf message that can contain a variety of inner messages.
// If a Channel's message type implements Wrapper, the channel will
// automatically (un)wrap passed messages using the container type, such that
// the channel can transparently support multiple message types.
type Wrapper interface {
	// Wrap will take a message and wrap it in this one.
	Wrap(proto.Message) error

	// Unwrap will unwrap the inner message contained in this message.
	Unwrap() (proto.Message, error)
}
