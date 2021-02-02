package p2p

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
)

// ChannelID is an arbitrary channel ID.
type ChannelID uint16

// Envelope specifies the message receiver and sender.
type Envelope struct {
	From      NodeID        // Message sender, or empty for outbound messages.
	To        NodeID        // Message receiver, or empty for inbound messages.
	Broadcast bool          // Send message to all connected peers, ignoring To.
	Message   proto.Message // Payload.

	// For internal use in the Router.
	channelID ChannelID
}

// Strip strips internal information from the envelope. Primarily used for
// testing, such that returned envelopes can be compared with literals.
func (e Envelope) Strip() Envelope {
	e.channelID = 0
	return e
}

// PeerError is a peer error reported via the Error channel.
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
	NodeID NodeID
	Err    error
}

// Channel is a bidirectional channel for Protobuf message exchange with peers.
// A Channel is safe for concurrent use by multiple goroutines.
type Channel struct {
	closeOnce sync.Once

	// id defines the unique channel ID.
	id ChannelID

	// messageType specifies the type of messages exchanged via the channel, and
	// is used e.g. for automatic unmarshaling.
	messageType proto.Message

	// inCh is a channel for receiving inbound messages. Envelope.From is always
	// set.
	inCh chan Envelope

	// outCh is a channel for sending outbound messages. Envelope.To or Broadcast
	// must be set, otherwise the message is discarded.
	outCh chan Envelope

	// errCh is a channel for reporting peer errors to the router, typically used
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
func NewChannel(id ChannelID, mType proto.Message, in, out chan Envelope, errCh chan PeerError) *Channel {
	return &Channel{
		id:          id,
		messageType: mType,
		inCh:        in,
		outCh:       out,
		errCh:       errCh,
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
// the outbound outCh and peer error errCh channels are closed. It is the reactor's
// responsibility to invoke Close. Any send on the Out or Error channel will
// panic after the Channel is closed.
//
// NOTE: After a Channel is closed, the router may safely assume it can no longer
// send on the internal inCh, however it should NEVER explicitly close it as
// that could result in panics by sending on a closed channel.
func (c *Channel) Close() {
	c.closeOnce.Do(func() {
		close(c.doneCh)
		close(c.outCh)
		close(c.errCh)
	})
}

// Done returns the Channel's internal channel that should be used by a router
// to signal when it is safe to send on the internal inCh go channel.
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

// RouterOptions specifies options for a Router.
type RouterOptions struct {
	// ResolveTimeout is the timeout for resolving NodeAddress URLs.
	// 0 means no timeout.
	ResolveTimeout time.Duration

	// DialTimeout is the timeout for dialing a peer. 0 means no timeout.
	DialTimeout time.Duration

	// HandshakeTimeout is the timeout for handshaking with a peer. 0 means
	// no timeout.
	HandshakeTimeout time.Duration
}

// Validate validates the options.
func (o *RouterOptions) Validate() error {
	return nil
}

// Router manages peer connections and routes messages between peers and reactor
// channels. It takes a PeerManager for peer lifecycle management (e.g. which
// peers to dial and when) and a set of Transports for connecting to and
// communicating with peers. Transports should be set up to listen on any
// desired interfaces before handing them to the Router, and the Router will
// take care of closing them when shutting down.
//
// message queue for the channel in channelQueues and spawn off a goroutine for
// Router.routeChannel(). This goroutine reads off outbound messages and puts
// them in the appropriate peer message queue, and processes peer errors which
// will close (and thus disconnect) the appriate peer queue. It runs until
// either the channel is closed by the caller or the router is stopped, at which
// point the input message queue is closed and removed.
//
// On startup, the router spawns off three primary goroutines that maintain
// connections to peers and run for the lifetime of the router:
//
//   Router.dialPeers(): in a loop, asks the PeerManager for the next peer
//   address to contact, resolves it into endpoints, and attempts to dial
//   each one.
//
//   Router.acceptPeers(): in a loop, waits for the next inbound connection
//   from a peer, and checks with the PeerManager if it should be accepted.
//
//   Router.evictPeers(): in a loop, asks the PeerManager for any connected
//   peers to evict, and disconnects them.
//
// Once either an inbound or outbound connection has been made, an outbound
// message queue is registered in Router.peerQueues and a goroutine is spawned
// off for Router.routePeer() which will spawn off additional goroutines for
// Router.sendPeer() that sends outbound messages from the peer queue over the
// connection and for Router.receivePeer() that reads inbound messages from
// the connection and places them in the appropriate channel queue. When either
// goroutine exits, the connection and peer queue is closed, which will cause
// the other goroutines to close as well.
//
// The peerStore is used to coordinate peer connections, by only allowing a peer
// to be claimed (owned) by a single caller at a time (both for outbound and
// inbound connections). This is done either via peerStore.Dispense() which
// dispenses and claims an eligible peer to dial, or via peerStore.Claim() which
// attempts to claim a given peer for an inbound connection. Peers must be
// returned to the peerStore with peerStore.Return() to release the claim. Over
// time, the peerStore will also do peer scheduling and prioritization, e.g.
// ensuring we do exponential backoff on dial failures and connecting to
// more important peers first (such as persistent peers and validators).
//
// An additional goroutine Router.broadcastPeerUpdates() is also spawned off
// on startup, which consumes peer updates from Router.peerUpdatesCh (currently
// only connections and disconnections), and broadcasts them to all peer update
// subscriptions registered via SubscribePeerUpdates().
//
// On router shutdown, we close Router.stopCh which will signal to all
// goroutines to terminate. This in turn will cause all pending channel/peer
// queues to close, and we wait for this as a signal that goroutines have ended.
//
// All message scheduling should be limited to the queue implementations used
// for channel queues and peer queues. All message sending throughout the router
// is blocking, and if any messages should be dropped or buffered this is the
// sole responsibility of the queue, such that we can limit this logic to a
// single place. There is currently only a FIFO queue implementation that always
// blocks and never drops messages, but this must be improved with other
// implementations. The only exception is that all message sending must also
// select on appropriate channel/queue/router closure signals, to avoid blocking
// forever on a channel that has no consumer.
type Router struct {
	*service.BaseService

	logger      log.Logger
	nodeInfo    NodeInfo
	privKey     crypto.PrivKey
	transports  map[Protocol]Transport
	peerManager *PeerManager
	options     RouterOptions

	// FIXME: Consider using sync.Map.
	peerMtx    sync.RWMutex
	peerQueues map[NodeID]queue

	// FIXME: We don't strictly need to use a mutex for this if we seal the
	// channels on router start. This depends on whether we want to allow
	// dynamic channels in the future.
	channelMtx      sync.RWMutex
	channelQueues   map[ChannelID]queue
	channelMessages map[ChannelID]proto.Message

	// stopCh is used to signal router shutdown, by closing the channel.
	stopCh chan struct{}
}

// NewRouter creates a new Router.
func NewRouter(
	logger log.Logger,
	nodeInfo NodeInfo,
	privKey crypto.PrivKey,
	peerManager *PeerManager,
	transports []Transport,
	options RouterOptions,
) (*Router, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}

	router := &Router{
		logger:          logger,
		nodeInfo:        nodeInfo,
		privKey:         privKey,
		transports:      map[Protocol]Transport{},
		peerManager:     peerManager,
		options:         options,
		stopCh:          make(chan struct{}),
		channelQueues:   map[ChannelID]queue{},
		channelMessages: map[ChannelID]proto.Message{},
		peerQueues:      map[NodeID]queue{},
	}
	router.BaseService = service.NewBaseService(logger, "router", router)

	for _, transport := range transports {
		for _, protocol := range transport.Protocols() {
			if _, ok := router.transports[protocol]; !ok {
				router.transports[protocol] = transport
			}
		}
	}

	return router, nil
}

// OpenChannel opens a new channel for the given message type. The caller must
// close the channel when done, and this must happen before the router stops.
func (r *Router) OpenChannel(id ChannelID, messageType proto.Message) (*Channel, error) {
	// FIXME: NewChannel should take directional channels so we can pass
	// queue.dequeue() instead of reaching inside for queue.queueCh.
	queue := newFIFOQueue()
	channel := NewChannel(id, messageType, queue.queueCh, make(chan Envelope), make(chan PeerError))

	r.channelMtx.Lock()
	defer r.channelMtx.Unlock()

	if _, ok := r.channelQueues[id]; ok {
		return nil, fmt.Errorf("channel %v already exists", id)
	}
	r.channelQueues[id] = queue
	r.channelMessages[id] = messageType

	go func() {
		defer func() {
			r.channelMtx.Lock()
			delete(r.channelQueues, id)
			delete(r.channelMessages, id)
			r.channelMtx.Unlock()
			queue.close()
		}()
		r.routeChannel(channel)
	}()

	return channel, nil
}

// routeChannel receives outbound messages and errors from a channel and routes
// them to the appropriate peer. It returns when either the channel is closed or
// the router is shutting down.
func (r *Router) routeChannel(channel *Channel) {
	for {
		select {
		case envelope, ok := <-channel.outCh:
			if !ok {
				return
			}

			// FIXME: This is a bit unergonomic, maybe it'd be better for Wrap()
			// to return a wrapped copy.
			if _, ok := channel.messageType.(Wrapper); ok {
				wrapper := proto.Clone(channel.messageType)
				if err := wrapper.(Wrapper).Wrap(envelope.Message); err != nil {
					r.Logger.Error("failed to wrap message", "err", err)
					continue
				}
				envelope.Message = wrapper
			}
			envelope.channelID = channel.id

			if envelope.Broadcast {
				r.peerMtx.RLock()
				peerQueues := make(map[NodeID]queue, len(r.peerQueues))
				for peerID, peerQueue := range r.peerQueues {
					peerQueues[peerID] = peerQueue
				}
				r.peerMtx.RUnlock()

				for peerID, peerQueue := range peerQueues {
					e := envelope
					e.Broadcast = false
					e.To = peerID
					select {
					case peerQueue.enqueue() <- e:
					case <-peerQueue.closed():
					case <-r.stopCh:
						return
					}
				}

			} else {
				r.peerMtx.RLock()
				peerQueue, ok := r.peerQueues[envelope.To]
				r.peerMtx.RUnlock()
				if !ok {
					r.logger.Error("dropping message for non-connected peer",
						"peer", envelope.To, "channel", channel.id)
					continue
				}

				select {
				case peerQueue.enqueue() <- envelope:
				case <-peerQueue.closed():
					r.logger.Error("dropping message for non-connected peer",
						"peer", envelope.To, "channel", channel.id)
				case <-r.stopCh:
					return
				}
			}

		case peerError, ok := <-channel.errCh:
			if !ok {
				return
			}
			// FIXME: We just evict the peer for now.
			r.logger.Error("peer error, evicting", "peer", peerError.NodeID, "err", peerError.Err)
			if err := r.peerManager.Errored(peerError.NodeID, peerError.Err); err != nil {
				r.logger.Error("failed to report peer error", "peer", peerError.NodeID, "err", err)
			}

		case <-channel.Done():
			return
		case <-r.stopCh:
			return
		}
	}
}

// acceptPeers accepts inbound connections from peers on the given transport.
func (r *Router) acceptPeers(transport Transport) {
	ctx := r.stopCtx()
	for {
		// FIXME: We may need transports to enforce some sort of rate limiting
		// here (e.g. by IP address), or alternatively have PeerManager.Accepted()
		// do it for us.
		//
		// FIXME: Even though PeerManager enforces MaxConnected, we may want to
		// limit the maximum number of active connections here too, since e.g.
		// an adversary can open a ton of connections and then just hang during
		// the handshake, taking up TCP socket descriptors.
		//
		// FIXME: The old P2P stack rejected multiple connections for the same IP
		// unless P2PConfig.AllowDuplicateIP is true -- it's better to limit this
		// by peer ID rather than IP address, so this hasn't been implemented and
		// probably shouldn't (?).
		//
		// FIXME: The old P2P stack supported ABCI-based IP address filtering via
		// /p2p/filter/addr/<ip> queries, do we want to implement this here as well?
		// Filtering by node ID is probably better.
		conn, err := transport.Accept()
		switch err {
		case nil:
		case io.EOF:
			r.logger.Debug("stopping accept routine", "transport", transport)
			return
		default:
			r.logger.Error("failed to accept connection", "transport", transport, "err", err)
			continue
		}

		go func() {
			defer func() {
				_ = conn.Close()
			}()

			// FIXME: Because we do the handshake in each transport, rather than
			// here in the Router, the remote peer will think they've
			// successfully connected and start sending us messages, although we
			// can end up rejecting the connection here. This can e.g. cause
			// problems in tests, where because of race conditions a
			// disconnection can cause the local node to immediately redial,
			// while the remote node may not have completed the disconnection
			// registration yet and reject the accept below.
			//
			// The Router should do the handshake, and we should check with the
			// peer manager before completing the handshake -- this probably
			// requires protocol changes to send an additional message when the
			// handshake is accepted.
			peerInfo, _, err := r.handshakePeer(ctx, conn, "")
			if err == context.Canceled {
				return
			} else if err != nil {
				r.logger.Error("failed to handshake with peer", "err", err)
				return
			}
			if err := r.peerManager.Accepted(peerInfo.NodeID); err != nil {
				r.logger.Error("failed to accept connection", "peer", peerInfo.NodeID, "err", err)
				return
			}

			queue := newFIFOQueue()
			r.peerMtx.Lock()
			r.peerQueues[peerInfo.NodeID] = queue
			r.peerMtx.Unlock()

			defer func() {
				r.peerMtx.Lock()
				delete(r.peerQueues, peerInfo.NodeID)
				r.peerMtx.Unlock()
				queue.close()
				if err := r.peerManager.Disconnected(peerInfo.NodeID); err != nil {
					r.logger.Error("failed to disconnect peer", "peer", peerInfo.NodeID, "err", err)
				}
			}()

			if err := r.peerManager.Ready(peerInfo.NodeID); err != nil {
				r.logger.Error("failed to mark peer as ready", "peer", peerInfo.NodeID, "err", err)
				return
			}

			r.routePeer(peerInfo.NodeID, conn, queue)
		}()
	}
}

// dialPeers maintains outbound connections to peers.
func (r *Router) dialPeers() {
	ctx := r.stopCtx()
	for {
		address, err := r.peerManager.DialNext(ctx)
		switch err {
		case nil:
		case context.Canceled:
			r.logger.Debug("stopping dial routine")
			return
		default:
			r.logger.Error("failed to find next peer to dial", "err", err)
			return
		}

		go func() {
			peerID := address.NodeID
			conn, err := r.dialPeer(ctx, address)
			if errors.Is(err, context.Canceled) {
				return
			} else if err != nil {
				r.logger.Error("failed to dial peer", "peer", peerID, "err", err)
				if err = r.peerManager.DialFailed(address); err != nil {
					r.logger.Error("failed to report dial failure", "peer", peerID, "err", err)
				}
				return
			}
			defer conn.Close()

			_, _, err = r.handshakePeer(ctx, conn, peerID)
			if errors.Is(err, context.Canceled) {
				return
			} else if err != nil {
				r.logger.Error("failed to handshake with peer", "peer", peerID, "err", err)
				if err = r.peerManager.DialFailed(address); err != nil {
					r.logger.Error("failed to report dial failure", "peer", peerID, "err", err)
				}
				return
			}

			if err = r.peerManager.Dialed(address); err != nil {
				r.logger.Error("failed to dial peer", "peer", peerID, "err", err)
				return
			}

			queue := newFIFOQueue()
			r.peerMtx.Lock()
			r.peerQueues[peerID] = queue
			r.peerMtx.Unlock()

			defer func() {
				r.peerMtx.Lock()
				delete(r.peerQueues, peerID)
				r.peerMtx.Unlock()
				queue.close()
				if err := r.peerManager.Disconnected(peerID); err != nil {
					r.logger.Error("failed to disconnect peer", "peer", peerID, "err", err)
				}
			}()

			if err := r.peerManager.Ready(peerID); err != nil {
				r.logger.Error("failed to mark peer as ready", "peer", peerID, "err", err)
				return
			}

			r.routePeer(peerID, conn, queue)
		}()
	}
}

// dialPeer connects to a peer by dialing it.
func (r *Router) dialPeer(ctx context.Context, address NodeAddress) (Connection, error) {
	r.logger.Info("resolving peer address", "address", address)
	resolveCtx := ctx
	if r.options.ResolveTimeout > 0 {
		var cancel context.CancelFunc
		resolveCtx, cancel = context.WithTimeout(resolveCtx, r.options.ResolveTimeout)
		defer cancel()
	}
	endpoints, err := address.Resolve(resolveCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address %q: %w", address, err)
	}

	for _, endpoint := range endpoints {
		transport, ok := r.transports[endpoint.Protocol]
		if !ok {
			r.logger.Error("no transport found for endpoint protocol", "endpoint", endpoint)
			continue
		}

		dialCtx := ctx
		if r.options.DialTimeout > 0 {
			var cancel context.CancelFunc
			dialCtx, cancel = context.WithTimeout(dialCtx, r.options.DialTimeout)
			defer cancel()
		}

		// FIXME: When we dial and handshake the peer, we should pass it
		// appropriate address(es) it can use to dial us back. It can't use our
		// remote endpoint, since TCP uses different port numbers for outbound
		// connections than it does for inbound. Also, we may need to vary this
		// by the peer's endpoint, since e.g. a peer on 192.168.0.0 can reach us
		// on a private address on this endpoint, but a peer on the public
		// Internet can't and needs a different public address.
		conn, err := transport.Dial(dialCtx, endpoint)
		if err != nil {
			r.logger.Error("failed to dial endpoint", "endpoint", endpoint, "err", err)
		} else {
			r.logger.Info("connected to peer", "peer", address.NodeID, "endpoint", endpoint)
			return conn, nil
		}
	}
	return nil, fmt.Errorf("failed to connect to peer via %q", address)
}

// handshakePeer handshakes with a peer, validating the peer's information. If
// expectID is given, we check that the peer's public key matches it.
func (r *Router) handshakePeer(ctx context.Context, conn Connection, expectID NodeID) (NodeInfo, crypto.PubKey, error) {
	if r.options.HandshakeTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.options.HandshakeTimeout)
		defer cancel()
	}
	peerInfo, peerKey, err := conn.Handshake(ctx, r.nodeInfo, r.privKey)
	if err != nil {
		return peerInfo, peerKey, err
	}
	if err = peerInfo.Validate(); err != nil {
		return peerInfo, peerKey, fmt.Errorf("invalid handshake NodeInfo: %w", err)
	}
	if expectID != "" && expectID != peerInfo.NodeID {
		return peerInfo, peerKey, fmt.Errorf("expected to connect with peer %q, got %q",
			expectID, peerInfo.NodeID)
	}
	if NodeIDFromPubKey(peerKey) != peerInfo.NodeID {
		return peerInfo, peerKey, fmt.Errorf("peer's public key did not match its node ID %q (expected %q)",
			peerInfo.NodeID, NodeIDFromPubKey(peerKey))
	}
	if peerInfo.NodeID == r.nodeInfo.NodeID {
		return peerInfo, peerKey, errors.New("rejecting handshake with self")
	}
	return peerInfo, peerKey, nil
}

// routePeer routes inbound messages from a peer to channels, and also sends
// outbound queued messages to the peer. It will close the connection and send
// queue, using this as a signal to coordinate the internal receivePeer() and
// sendPeer() goroutines. It blocks until the peer is done, e.g. when the
// connection or queue is closed.
func (r *Router) routePeer(peerID NodeID, conn Connection, sendQueue queue) {
	r.logger.Info("routing peer", "peer", peerID)
	resultsCh := make(chan error, 2)
	go func() {
		resultsCh <- r.receivePeer(peerID, conn)
	}()
	go func() {
		resultsCh <- r.sendPeer(peerID, conn, sendQueue)
	}()

	err := <-resultsCh
	_ = conn.Close()
	sendQueue.close()
	if e := <-resultsCh; err == nil {
		// The first err was nil, so we update it with the second result,
		// which may or may not be nil.
		err = e
	}
	switch err {
	case nil, io.EOF, ErrTransportClosed{}:
		r.logger.Info("peer disconnected", "peer", peerID)
	default:
		r.logger.Error("peer failure", "peer", peerID, "err", err)
	}
}

// receivePeer receives inbound messages from a peer, deserializes them and
// passes them on to the appropriate channel.
func (r *Router) receivePeer(peerID NodeID, conn Connection) error {
	for {
		chID, bz, err := conn.ReceiveMessage()
		if err != nil {
			return err
		}

		r.channelMtx.RLock()
		queue, ok := r.channelQueues[chID]
		messageType := r.channelMessages[chID]
		r.channelMtx.RUnlock()
		if !ok {
			r.logger.Error("dropping message for unknown channel", "peer", peerID, "channel", chID)
			continue
		}

		msg := proto.Clone(messageType)
		if err := proto.Unmarshal(bz, msg); err != nil {
			r.logger.Error("message decoding failed, dropping message", "peer", peerID, "err", err)
			continue
		}
		if wrapper, ok := msg.(Wrapper); ok {
			msg, err = wrapper.Unwrap()
			if err != nil {
				r.logger.Error("failed to unwrap message", "err", err)
				continue
			}
		}

		select {
		case queue.enqueue() <- Envelope{channelID: chID, From: peerID, Message: msg}:
			r.logger.Debug("received message", "peer", peerID, "message", msg)
		case <-queue.closed():
			r.logger.Error("channel closed, dropping message", "peer", peerID, "channel", chID)
		case <-r.stopCh:
			return nil
		}
	}
}

// sendPeer sends queued messages to a peer.
func (r *Router) sendPeer(peerID NodeID, conn Connection, queue queue) error {
	for {
		select {
		case envelope := <-queue.dequeue():
			bz, err := proto.Marshal(envelope.Message)
			if err != nil {
				r.logger.Error("failed to marshal message", "peer", peerID, "err", err)
				continue
			}

			_, err = conn.SendMessage(envelope.channelID, bz)
			if err != nil {
				return err
			}
			r.logger.Debug("sent message", "peer", envelope.To, "message", envelope.Message)

		case <-queue.closed():
			return nil

		case <-r.stopCh:
			return nil
		}
	}
}

// evictPeers evicts connected peers as requested by the peer manager.
func (r *Router) evictPeers() {
	ctx := r.stopCtx()
	for {
		peerID, err := r.peerManager.EvictNext(ctx)
		switch err {
		case nil:
		case context.Canceled:
			r.logger.Debug("stopping evict routine")
			return
		default:
			r.logger.Error("failed to find next peer to evict", "err", err)
			return
		}

		r.logger.Info("evicting peer", "peer", peerID)
		r.peerMtx.RLock()
		if queue, ok := r.peerQueues[peerID]; ok {
			queue.close()
		}
		r.peerMtx.RUnlock()
	}
}

// OnStart implements service.Service.
func (r *Router) OnStart() error {
	go r.dialPeers()
	for _, transport := range r.transports {
		go r.acceptPeers(transport)
	}
	go r.evictPeers()
	return nil
}

// OnStop implements service.Service.
//
// FIXME: This needs to close transports as well.
func (r *Router) OnStop() {
	// Collect all active queues, so we can wait for them to close.
	queues := []queue{}
	r.channelMtx.RLock()
	for _, q := range r.channelQueues {
		queues = append(queues, q)
	}
	r.channelMtx.RUnlock()
	r.peerMtx.RLock()
	for _, q := range r.peerQueues {
		queues = append(queues, q)
	}
	r.peerMtx.RUnlock()

	// Signal router shutdown, and wait for queues (and thus goroutines)
	// to complete.
	close(r.stopCh)
	for _, q := range queues {
		<-q.closed()
	}
}

// stopCtx returns a context that is cancelled when the router stops.
func (r *Router) stopCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-r.stopCh
		cancel()
	}()
	return ctx
}
