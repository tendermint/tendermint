package p2p

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/tychoish/emt"

	"github.com/tendermint/tendermint/internal/libs/protoio"
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
	go func(pipe chan Envelope) {
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
		go func(ch Channel, pipe chan Envelope) {
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

type libp2pChannelImpl struct {
	chDesc  *ChannelDescriptor
	pubsub  *pubsub.PubSub
	host    host.Host
	topic   *pubsub.Topic
	recvCh  chan Envelope
	chainID string
	wrapper Wrapper

	// thread-safe error aggregator used to collect errors seen
	// during receiving messages into an iterator.
	errs emt.Catcher

	// the context is passed when the channel is opened and should
	// cover the lifecycle of the channel itself. The contexts
	// passed into methods should cover the lifecycle of the
	// operations they represent.
	ctx context.Context
}

func NewLibP2PChannel(ctx context.Context, chainID string, chDesc *ChannelDescriptor, ps *pubsub.PubSub, h host.Host) (Channel, error) {
	ch := &libp2pChannelImpl{
		ctx:     ctx,
		chDesc:  chDesc,
		pubsub:  ps,
		host:    h,
		chainID: chainID,
		recvCh:  make(chan Envelope, chDesc.RecvMessageCapacity),
		errs:    emt.NewCatcher(),
	}
	topic, err := ps.Join(ch.canonicalizedTopicName())
	if err != nil {
		return nil, err
	}
	ch.topic = topic

	if w, ok := chDesc.MessageType.(Wrapper); ok {
		ch.wrapper = w
	}

	return ch, nil
}

func (ch *libp2pChannelImpl) String() string {
	return fmt.Sprintf("Channel<%s>", ch.canonicalizedTopicName())
}

func (ch *libp2pChannelImpl) Err() error { return ch.errs.Resolve() }

func (ch *libp2pChannelImpl) canonicalizedTopicName() string {
	return fmt.Sprintf("%s.%s.%d", ch.chainID, ch.chDesc.Name, ch.chDesc.ID)
}

func (ch *libp2pChannelImpl) Receive(ctx context.Context) *ChannelIterator {
	// TODO: consider caching an iterator in the channel, or
	// erroring if this gets called more than once.
	//
	// While it's safe to register a handler more than once, we
	// could get into a dodgy situation where if you call receive
	// more  than once, the subsequently messages won't be routed
	// correctly.
	iter := &ChannelIterator{
		pipe: make(chan Envelope),
	}

	ch.host.SetStreamHandler(protocol.ID(ch.canonicalizedTopicName()), func(stream network.Stream) {
		// TODO: properly capture the max message size here.
		reader := protoio.NewDelimitedReader(bufio.NewReader(stream), ch.chDesc.RecvBufferCapacity*2)

		remote := stream.Conn().RemotePeer()

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		go func() { <-ctx.Done(); ch.errs.Add(stream.Close()) }()

		for {
			payload := proto.Clone(ch.chDesc.MessageType)

			if _, err := reader.ReadMsg(payload); err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return
				}
				ch.errs.Add(err)
				continue
			}
			select {
			case <-ctx.Done():
				return
			case iter.pipe <- Envelope{
				From:      types.NodeID(remote),
				Message:   payload,
				ChannelID: ch.chDesc.ID,
			}:
			}
		}
	})

	sub, err := ch.topic.Subscribe()
	if err != nil {
		ch.errs.Add(err)
		return nil
	}

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case e := <-ch.recvCh:
				select {
				case <-ctx.Done():
					return
				case iter.pipe <- e:
					continue
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			msg, err := sub.Next(ctx)
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return
				}

				ch.errs.Add(err)
				return
			}

			payload := proto.Clone(ch.chDesc.MessageType)
			if err := proto.Unmarshal(msg.Data, payload); err != nil {
				ch.errs.Add(err)
				return
			}
			if wrapper, ok := payload.(Wrapper); ok {
				if payload, err = wrapper.Unwrap(); err != nil {
					ch.errs.Add(err)
					return
				}
			}

			select {
			case <-ctx.Done():
				return
			case iter.pipe <- Envelope{
				From:      types.NodeID(msg.From),
				Message:   payload,
				ChannelID: ch.chDesc.ID,
			}:
			}
		}
	}()

	// TODO: this is probably wrong in it's current form: the
	// handler for point-to-point messages could still end up
	// trying to send into the pipe after things close.
	go func() { wg.Wait(); defer close(iter.pipe) }()

	return iter
}

func (ch *libp2pChannelImpl) Send(ctx context.Context, e Envelope) error {
	if ch.wrapper != nil {
		msg := proto.Clone(ch.wrapper)
		if err := msg.(Wrapper).Wrap(e.Message); err != nil {
			return err
		}

		e.Message = msg
	}

	if e.Broadcast {
		e.From = types.NodeID(ch.host.ID())
		bz, err := proto.Marshal(e.Message)
		if err != nil {
			return err
		}

		return ch.topic.Publish(ctx, bz)
	}

	switch ch.host.Network().Connectedness(peer.ID(e.To)) {
	case network.CannotConnect:
		return fmt.Errorf("cannot connect to %q", e.To)
	default:
		stream, err := ch.getStream(peer.ID(e.To))
		if err != nil {
			return err
		}

		writer := protoio.NewDelimitedWriter(bufio.NewWriter(stream))
		_, err = writer.WriteMsg(e.Message)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ch *libp2pChannelImpl) getStream(peer peer.ID) (network.Stream, error) {
	conns := ch.host.Network().ConnsToPeer(peer)
	pid := protocol.ID(ch.canonicalizedTopicName())
	if len(conns) > 0 {
		for cidx := range conns {
			streams := conns[cidx].GetStreams()
			for sidx := range streams {
				stream := streams[sidx]
				if stream.Protocol() == pid && stream.Stat().Direction == network.DirOutbound {
					return stream, nil
				}
			}
		}

	}

	conn, err := ch.host.Network().DialPeer(ch.ctx, peer)
	if err != nil {
		return nil, err
	}

	stream, err := ch.host.NewStream(ch.ctx, conn.RemotePeer(), pid)
	if err != nil {
		return nil, err
	}

	return stream, nil
}

func (ch *libp2pChannelImpl) SendError(ctx context.Context, pe PeerError) error {
	if errors.Is(pe.Err, context.Canceled) || errors.Is(pe.Err, context.DeadlineExceeded) || ctx.Err() != nil {
		return nil
	}

	// TODO: change handling of errors to peers. This problably
	// shouldn't be handled as a property of the channel, and
	// rather as part of some peer-info/network-management
	// interface, but we can do it here for now, to ensure compatibility.
	//
	// Closing the peer is the same behavior as the legacy system,
	// and seems less drastic than blacklisting the peer forever.
	return ch.host.Network().ClosePeer(peer.ID(pe.NodeID))
}
