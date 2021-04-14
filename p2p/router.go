package p2p

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
)

const queueBufferDefault = 4096

// ChannelID is an arbitrary channel ID.
type ChannelID uint16

// Envelope contains a message with sender/receiver routing info.
type Envelope struct {
	From      NodeID        // sender (empty if outbound)
	To        NodeID        // receiver (empty if inbound)
	Broadcast bool          // send to all connected peers (ignores To)
	Message   proto.Message // message payload

	// channelID is for internal Router use, set on outbound messages to inform
	// the sendPeer() goroutine which transport channel to use.
	//
	// FIXME: If we migrate the Transport API to a byte-oriented multi-stream
	// API, this will no longer be necessary since each channel will be mapped
	// onto a stream during channel/peer setup. See:
	// https://github.com/tendermint/spec/pull/227
	channelID ChannelID
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
	NodeID NodeID
	Err    error
}

// Channel is a bidirectional channel to exchange Protobuf messages with peers,
// wrapped in Envelope to specify routing info (i.e. sender/receiver).
type Channel struct {
	ID    ChannelID
	In    <-chan Envelope  // inbound messages (peers to reactors)
	Out   chan<- Envelope  // outbound messages (reactors to peers)
	Error chan<- PeerError // peer error reporting

	messageType proto.Message // the channel's message type, used for unmarshaling
	closeCh     chan struct{}
	closeOnce   sync.Once
}

// NewChannel creates a new channel. It is primarily for internal and test
// use, reactors should use Router.OpenChannel().
func NewChannel(
	id ChannelID,
	messageType proto.Message,
	inCh <-chan Envelope,
	outCh chan<- Envelope,
	errCh chan<- PeerError,
) *Channel {
	return &Channel{
		ID:          id,
		messageType: messageType,
		In:          inCh,
		Out:         outCh,
		Error:       errCh,
		closeCh:     make(chan struct{}),
	}
}

// Close closes the channel. Future sends on Out and Error will panic. The In
// channel remains open to avoid having to synchronize Router senders, which
// should use Done() to detect channel closure instead.
func (c *Channel) Close() {
	c.closeOnce.Do(func() {
		close(c.closeCh)
		close(c.Out)
		close(c.Error)
	})
}

// Done returns a channel that's closed when Channel.Close() is called.
func (c *Channel) Done() <-chan struct{} {
	return c.closeCh
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

	// QueueType must be "wdrr" (Weighed Deficit Round Robin),
	// "priority", or FIFO. Defaults to FIFO.
	QueueType string

	// MaxIncommingConnectionsPerIP limits the number of incoming
	// connections per IP address. Defaults to 100.
	MaxIncommingConnectionsPerIP uint

	// IncomingConnectionWindow describes how often an IP address
	// can attempt to create a new connection. Defaults to 10
	// milliseconds, and cannot be less than 1 millisecond.
	IncomingConnectionWindow time.Duration

	// FilterPeerByIP is used by the router to inject filtering
	// behavior for new incoming connections. The router passes
	// the remote IP of the incoming connection the port number as
	// arguments. Functions should return an error to reject the
	// peer.
	FilterPeerByIP func(context.Context, net.IP, uint16) error

	// FilterPeerByID is used by the router to inject filtering
	// behavior for new incoming connections. The router passes
	// the NodeID of the node before completing the connection,
	// but this occurs after the handshake is complete. Filter by
	// IP address to filter before the handshake. Functions should
	// return an error to reject the peer.
	FilterPeerByID func(context.Context, NodeID) error
}

const (
	queueTypeFifo     = "fifo"
	queueTypePriority = "priority"
	queueTypeWDRR     = "wdrr"
)

// Validate validates router options.
func (o *RouterOptions) Validate() error {
	switch o.QueueType {
	case "":
		o.QueueType = queueTypeFifo
	case queueTypeFifo, queueTypeWDRR, queueTypePriority:
		// pass
	default:
		return fmt.Errorf("queue type %q is not supported", o.QueueType)
	}

	switch {
	case o.IncomingConnectionWindow == 0:
		o.IncomingConnectionWindow = 100 * time.Millisecond
	case o.IncomingConnectionWindow < time.Millisecond:
		return fmt.Errorf("incomming connection window must be grater than 1m [%s]",
			o.IncomingConnectionWindow)
	}

	if o.MaxIncommingConnectionsPerIP == 0 {
		o.MaxIncommingConnectionsPerIP = 100
	}

	return nil
}

// Router manages peer connections and routes messages between peers and reactor
// channels. It takes a PeerManager for peer lifecycle management (e.g. which
// peers to dial and when) and a set of Transports for connecting and
// communicating with peers.
//
// On startup, three main goroutines are spawned to maintain peer connections:
//
//   dialPeers(): in a loop, calls PeerManager.DialNext() to get the next peer
//   address to dial and spawns a goroutine that dials the peer, handshakes
//   with it, and begins to route messages if successful.
//
//   acceptPeers(): in a loop, waits for an inbound connection via
//   Transport.Accept() and spawns a goroutine that handshakes with it and
//   begins to route messages if successful.
//
//   evictPeers(): in a loop, calls PeerManager.EvictNext() to get the next
//   peer to evict, and disconnects it by closing its message queue.
//
// When a peer is connected, an outbound peer message queue is registered in
// peerQueues, and routePeer() is called to spawn off two additional goroutines:
//
//   sendPeer(): waits for an outbound message from the peerQueues queue,
//   marshals it, and passes it to the peer transport which delivers it.
//
//   receivePeer(): waits for an inbound message from the peer transport,
//   unmarshals it, and passes it to the appropriate inbound channel queue
//   in channelQueues.
//
// When a reactor opens a channel via OpenChannel, an inbound channel message
// queue is registered in channelQueues, and a channel goroutine is spawned:
//
//   routeChannel(): waits for an outbound message from the channel, looks
//   up the recipient peer's outbound message queue in peerQueues, and submits
//   the message to it.
//
// All channel sends in the router are blocking. It is the responsibility of the
// queue interface in peerQueues and channelQueues to prioritize and drop
// messages as appropriate during contention to prevent stalls and ensure good
// quality of service.
type Router struct {
	*service.BaseService

	logger             log.Logger
	metrics            *Metrics
	options            RouterOptions
	nodeInfo           NodeInfo
	privKey            crypto.PrivKey
	peerManager        *PeerManager
	chDescs            []ChannelDescriptor
	transports         []Transport
	connTracker        connectionTracker
	protocolTransports map[Protocol]Transport
	stopCh             chan struct{} // signals Router shutdown

	peerMtx      sync.RWMutex
	peerQueues   map[NodeID]queue // outbound messages per peer for all channels
	queueFactory func(int) queue

	// FIXME: We don't strictly need to use a mutex for this if we seal the
	// channels on router start. This depends on whether we want to allow
	// dynamic channels in the future.
	channelMtx      sync.RWMutex
	channelQueues   map[ChannelID]queue // inbound messages from all peers to a single channel
	channelMessages map[ChannelID]proto.Message
}

// NewRouter creates a new Router. The given Transports must already be
// listening on appropriate interfaces, and will be closed by the Router when it
// stops.
func NewRouter(
	logger log.Logger,
	metrics *Metrics,
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
		logger:   logger,
		metrics:  metrics,
		nodeInfo: nodeInfo,
		privKey:  privKey,
		connTracker: newConnTracker(
			options.MaxIncommingConnectionsPerIP,
			options.IncomingConnectionWindow),
		chDescs:            make([]ChannelDescriptor, 0),
		transports:         transports,
		protocolTransports: map[Protocol]Transport{},
		peerManager:        peerManager,
		options:            options,
		stopCh:             make(chan struct{}),
		channelQueues:      map[ChannelID]queue{},
		channelMessages:    map[ChannelID]proto.Message{},
		peerQueues:         map[NodeID]queue{},
	}

	router.BaseService = service.NewBaseService(logger, "router", router)

	qf, err := router.createQueueFactory()
	if err != nil {
		return nil, err
	}

	router.queueFactory = qf

	for _, transport := range transports {
		for _, protocol := range transport.Protocols() {
			if _, ok := router.protocolTransports[protocol]; !ok {
				router.protocolTransports[protocol] = transport
			}
		}
	}

	return router, nil
}

func (r *Router) createQueueFactory() (func(int) queue, error) {
	switch r.options.QueueType {
	case queueTypeFifo:
		return newFIFOQueue, nil

	case queueTypePriority:
		return func(size int) queue {
			if size%2 != 0 {
				size++
			}

			q := newPQScheduler(r.logger, r.metrics, r.chDescs, uint(size)/2, uint(size)/2, defaultCapacity)
			q.start()
			return q
		}, nil

	case queueTypeWDRR:
		return func(size int) queue {
			if size%2 != 0 {
				size++
			}

			q := newWDRRScheduler(r.logger, r.metrics, r.chDescs, uint(size)/2, uint(size)/2, defaultCapacity)
			q.start()
			return q
		}, nil

	default:
		return nil, fmt.Errorf("cannot construct queue of type %q", r.options.QueueType)
	}
}

// AddChannelDescriptors adds a set of ChannelDescriptors to the reactor. Note,
// this should be called before the router is started and any connections are made.
func (r *Router) AddChannelDescriptors(chDescs []*ChannelDescriptor) {
	for _, chDesc := range chDescs {
		r.chDescs = append(r.chDescs, *chDesc)
	}
}

// OpenChannel opens a new channel for the given message type. The caller must
// close the channel when done, before stopping the Router. messageType is the
// type of message passed through the channel (used for unmarshaling), which can
// implement Wrapper to automatically (un)wrap multiple message types in a
// wrapper message. The caller may provide a size to make the channel buffered,
// which internally makes the inbound, outbound, and error channel buffered.
func (r *Router) OpenChannel(id ChannelID, messageType proto.Message, size int) (*Channel, error) {
	if size == 0 {
		size = queueBufferDefault
	}

	r.channelMtx.Lock()
	defer r.channelMtx.Unlock()

	if _, ok := r.channelQueues[id]; ok {
		return nil, fmt.Errorf("channel %v already exists", id)
	}

	queue := r.queueFactory(size)
	outCh := make(chan Envelope, size)
	errCh := make(chan PeerError, size)
	channel := NewChannel(id, messageType, queue.dequeue(), outCh, errCh)

	var wrapper Wrapper
	if w, ok := messageType.(Wrapper); ok {
		wrapper = w
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

		r.routeChannel(id, outCh, errCh, wrapper)
	}()

	return channel, nil
}

// routeChannel receives outbound channel messages and routes them to the
// appropriate peer. It also receives peer errors and reports them to the peer
// manager. It returns when either the outbound channel or error channel is
// closed, or the Router is stopped. wrapper is an optional message wrapper
// for messages, see Wrapper for details.
func (r *Router) routeChannel(
	chID ChannelID,
	outCh <-chan Envelope,
	errCh <-chan PeerError,
	wrapper Wrapper,
) {
	for {
		select {
		case envelope, ok := <-outCh:
			if !ok {
				return
			}

			// Mark the envelope with the channel ID to allow sendPeer() to pass
			// it on to Transport.SendMessage().
			envelope.channelID = chID

			// wrap the message in a wrapper message, if requested
			if wrapper != nil {
				msg := proto.Clone(wrapper)
				if err := msg.(Wrapper).Wrap(envelope.Message); err != nil {
					r.Logger.Error("failed to wrap message", "channel", chID, "err", err)
					continue
				}

				envelope.Message = msg
			}

			// collect peer queues to pass the message via
			var queues []queue
			if envelope.Broadcast {
				r.peerMtx.RLock()

				queues = make([]queue, 0, len(r.peerQueues))
				for _, q := range r.peerQueues {
					queues = append(queues, q)
				}

				r.peerMtx.RUnlock()
			} else {
				r.peerMtx.RLock()
				q, ok := r.peerQueues[envelope.To]
				r.peerMtx.RUnlock()

				if !ok {
					r.logger.Debug("dropping message for unconnected peer", "peer", envelope.To, "channel", chID)
					continue
				}

				queues = []queue{q}
			}

			// send message to peers
			for _, q := range queues {
				start := time.Now().UTC()

				select {
				case q.enqueue() <- envelope:
					r.metrics.RouterPeerQueueSend.Observe(time.Since(start).Seconds())

				case <-q.closed():
					r.logger.Debug("dropping message for unconnected peer", "peer", envelope.To, "channel", chID)

				case <-r.stopCh:
					return
				}
			}

		case peerError, ok := <-errCh:
			if !ok {
				return
			}

			r.logger.Error("peer error, evicting", "peer", peerError.NodeID, "err", peerError.Err)

			if err := r.peerManager.Errored(peerError.NodeID, peerError.Err); err != nil {
				r.logger.Error("failed to report peer error", "peer", peerError.NodeID, "err", err)
			}

		case <-r.stopCh:
			return
		}
	}
}

func (r *Router) filterPeersIP(ctx context.Context, ip net.IP, port uint16) error {
	if r.options.FilterPeerByIP == nil {
		return nil
	}

	return r.options.FilterPeerByIP(ctx, ip, port)
}

func (r *Router) filterPeersID(ctx context.Context, id NodeID) error {
	if r.options.FilterPeerByID == nil {
		return nil
	}

	return r.options.FilterPeerByID(ctx, id)
}

// acceptPeers accepts inbound connections from peers on the given transport,
// and spawns goroutines that route messages to/from them.
func (r *Router) acceptPeers(transport Transport) {
	r.logger.Debug("starting accept routine", "transport", transport)
	ctx := r.stopCtx()
	for {
		conn, err := transport.Accept()
		switch err {
		case nil:
		case io.EOF:
			r.logger.Debug("stopping accept routine", "transport", transport)
			return
		default:
			r.logger.Error("failed to accept connection", "transport", transport, "err", err)
			return
		}

		incomingIP := conn.RemoteEndpoint().IP
		if err := r.connTracker.AddConn(incomingIP); err != nil {
			closeErr := conn.Close()
			r.logger.Debug("rate limiting incoming peer",
				"err", err,
				"ip", incomingIP.String(),
				"closeErr", closeErr)

			return
		}

		// Spawn a goroutine for the handshake, to avoid head-of-line blocking.
		go r.openConnection(ctx, conn)

	}
}

func (r *Router) openConnection(ctx context.Context, conn Connection) {
	defer conn.Close()
	defer r.connTracker.RemoveConn(conn.RemoteEndpoint().IP)

	re := conn.RemoteEndpoint()
	incomingIP := re.IP

	if err := r.filterPeersIP(ctx, incomingIP, re.Port); err != nil {
		r.logger.Debug("peer filtered by IP",
			"ip", incomingIP.String(),
			"err", err)
		return
	}

	// FIXME: The peer manager may reject the peer during Accepted()
	// after we've handshaked with the peer (to find out which peer it
	// is). However, because the handshake has no ack, the remote peer
	// will think the handshake was successful and start sending us
	// messages.
	//
	// This can cause problems in tests, where a disconnection can cause
	// the local node to immediately redial, while the remote node may
	// not have completed the disconnection yet and therefore reject the
	// reconnection attempt (since it thinks we're still connected from
	// before).
	//
	// The Router should do the handshake and have a final ack/fail
	// message to make sure both ends have accepted the connection, such
	// that it can be coordinated with the peer manager.
	peerInfo, _, err := r.handshakePeer(ctx, conn, "")
	switch {
	case errors.Is(err, context.Canceled):
		return
	case err != nil:
		r.logger.Error("peer handshake failed", "endpoint", conn, "err", err)
		return
	}

	if err := r.filterPeersID(ctx, peerInfo.NodeID); err != nil {
		r.logger.Debug("peer filtered by node ID",
			"node", peerInfo.NodeID,
			"err", err)
		return
	}

	if err := r.peerManager.Accepted(peerInfo.NodeID); err != nil {
		r.logger.Error("failed to accept connection", "peer", peerInfo.NodeID, "err", err)
		return
	}

	r.metrics.Peers.Add(1)
	queue := r.queueFactory(queueBufferDefault)

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
		} else {
			r.metrics.Peers.Add(-1)
		}
	}()

	if err := r.peerManager.Ready(peerInfo.NodeID); err != nil {
		r.logger.Error("failed to mark peer as ready", "peer", peerInfo.NodeID, "err", err)
		return
	}

	r.routePeer(peerInfo.NodeID, conn, queue)
}

// dialPeers maintains outbound connections to peers by dialing them.
func (r *Router) dialPeers() {
	r.logger.Debug("starting dial routine")
	ctx := r.stopCtx()
	for {
		address, err := r.peerManager.DialNext(ctx)
		switch {
		case errors.Is(err, context.Canceled):
			r.logger.Debug("stopping dial routine")
			return
		case err != nil:
			r.logger.Error("failed to find next peer to dial", "err", err)
			return
		}

		// Spawn off a goroutine to actually dial the peer, so that we can
		// dial multiple peers in parallel.
		go func() {
			conn, err := r.dialPeer(ctx, address)
			switch {
			case errors.Is(err, context.Canceled):
				return
			case err != nil:
				r.logger.Error("failed to dial peer", "peer", address, "err", err)
				if err = r.peerManager.DialFailed(address); err != nil {
					r.logger.Error("failed to report dial failure", "peer", address, "err", err)
				}
				return
			}
			defer conn.Close()

			peerID := address.NodeID
			_, _, err = r.handshakePeer(ctx, conn, peerID)
			switch {
			case errors.Is(err, context.Canceled):
				return
			case err != nil:
				r.logger.Error("failed to handshake with peer", "peer", address, "err", err)
				if err = r.peerManager.DialFailed(address); err != nil {
					r.logger.Error("failed to report dial failure", "peer", address, "err", err)
				}
				return
			}

			if err = r.peerManager.Dialed(address); err != nil {
				r.logger.Error("failed to dial peer", "peer", address, "err", err)
				return
			}

			r.metrics.Peers.Add(1)

			peerQueue := r.getOrMakeQueue(peerID)
			defer func() {
				r.peerMtx.Lock()
				delete(r.peerQueues, peerID)
				r.peerMtx.Unlock()

				peerQueue.close()

				if err := r.peerManager.Disconnected(peerID); err != nil {
					r.logger.Error("failed to disconnect peer", "peer", address, "err", err)
				} else {
					r.metrics.Peers.Add(-1)
				}
			}()

			if err := r.peerManager.Ready(peerID); err != nil {
				r.logger.Error("failed to mark peer as ready", "peer", address, "err", err)
				return
			}

			r.routePeer(peerID, conn, peerQueue)
		}()
	}
}

func (r *Router) getOrMakeQueue(peerID NodeID) queue {
	r.peerMtx.Lock()
	defer r.peerMtx.Unlock()

	if peerQueue, ok := r.peerQueues[peerID]; ok {
		return peerQueue
	}

	peerQueue := r.queueFactory(queueBufferDefault)
	r.peerQueues[peerID] = peerQueue
	return peerQueue
}

// dialPeer connects to a peer by dialing it.
func (r *Router) dialPeer(ctx context.Context, address NodeAddress) (Connection, error) {
	resolveCtx := ctx
	if r.options.ResolveTimeout > 0 {
		var cancel context.CancelFunc
		resolveCtx, cancel = context.WithTimeout(resolveCtx, r.options.ResolveTimeout)
		defer cancel()
	}

	r.logger.Debug("resolving peer address", "peer", address)
	endpoints, err := address.Resolve(resolveCtx)
	switch {
	case err != nil:
		return nil, fmt.Errorf("failed to resolve address %q: %w", address, err)
	case len(endpoints) == 0:
		return nil, fmt.Errorf("address %q did not resolve to any endpoints", address)
	}

	for _, endpoint := range endpoints {
		transport, ok := r.protocolTransports[endpoint.Protocol]
		if !ok {
			r.logger.Error("no transport found for protocol", "endpoint", endpoint)
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
			r.logger.Error("failed to dial endpoint", "peer", address.NodeID, "endpoint", endpoint, "err", err)
		} else {
			r.logger.Debug("dialed peer", "peer", address.NodeID, "endpoint", endpoint)
			return conn, nil
		}
	}
	return nil, errors.New("all endpoints failed")
}

// handshakePeer handshakes with a peer, validating the peer's information. If
// expectID is given, we check that the peer's info matches it.
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
	if NodeIDFromPubKey(peerKey) != peerInfo.NodeID {
		return peerInfo, peerKey, fmt.Errorf("peer's public key did not match its node ID %q (expected %q)",
			peerInfo.NodeID, NodeIDFromPubKey(peerKey))
	}
	if expectID != "" && expectID != peerInfo.NodeID {
		return peerInfo, peerKey, fmt.Errorf("expected to connect with peer %q, got %q",
			expectID, peerInfo.NodeID)
	}
	return peerInfo, peerKey, nil
}

// routePeer routes inbound and outbound messages between a peer and the reactor
// channels. It will close the given connection and send queue when done, or if
// they are closed elsewhere it will cause this method to shut down and return.
func (r *Router) routePeer(peerID NodeID, conn Connection, sendQueue queue) {
	r.logger.Info("peer connected", "peer", peerID, "endpoint", conn)

	errCh := make(chan error, 2)

	go func() {
		errCh <- r.receivePeer(peerID, conn)
	}()

	go func() {
		errCh <- r.sendPeer(peerID, conn, sendQueue)
	}()

	err := <-errCh
	_ = conn.Close()
	sendQueue.close()

	if e := <-errCh; err == nil {
		// The first err was nil, so we update it with the second err, which may
		// or may not be nil.
		err = e
	}

	switch err {
	case nil, io.EOF:
		r.logger.Info("peer disconnected", "peer", peerID, "endpoint", conn)

	default:
		r.logger.Error("peer failure", "peer", peerID, "endpoint", conn, "err", err)
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
			r.logger.Debug("dropping message for unknown channel", "peer", peerID, "channel", chID)
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

		start := time.Now().UTC()

		select {
		case queue.enqueue() <- Envelope{From: peerID, Message: msg}:
			r.metrics.PeerReceiveBytesTotal.With(
				"chID", fmt.Sprint(chID),
				"peer_id", string(peerID)).Add(float64(proto.Size(msg)))
			r.metrics.RouterChannelQueueSend.Observe(time.Since(start).Seconds())
			r.logger.Debug("received message", "peer", peerID, "message", msg)

		case <-queue.closed():
			r.logger.Debug("channel closed, dropping message", "peer", peerID, "channel", chID)

		case <-r.stopCh:
			return nil
		}
	}
}

// sendPeer sends queued messages to a peer.
func (r *Router) sendPeer(peerID NodeID, conn Connection, peerQueue queue) error {
	for {
		start := time.Now().UTC()

		select {
		case envelope := <-peerQueue.dequeue():
			r.metrics.RouterPeerQueueRecv.Observe(time.Since(start).Seconds())
			if envelope.Message == nil {
				r.logger.Error("dropping nil message", "peer", peerID)
				continue
			}

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

		case <-peerQueue.closed():
			return nil

		case <-r.stopCh:
			return nil
		}
	}
}

// evictPeers evicts connected peers as requested by the peer manager.
func (r *Router) evictPeers() {
	r.logger.Debug("starting evict routine")
	ctx := r.stopCtx()

	for {
		peerID, err := r.peerManager.EvictNext(ctx)

		switch {
		case errors.Is(err, context.Canceled):
			r.logger.Debug("stopping evict routine")
			return

		case err != nil:
			r.logger.Error("failed to find next peer to evict", "err", err)
			return
		}

		r.logger.Info("evicting peer", "peer", peerID)

		r.peerMtx.RLock()
		queue, ok := r.peerQueues[peerID]
		r.peerMtx.RUnlock()

		if ok {
			queue.close()
		}
	}
}

// OnStart implements service.Service.
func (r *Router) OnStart() error {
	go r.dialPeers()
	go r.evictPeers()

	for _, transport := range r.transports {
		go r.acceptPeers(transport)
	}

	return nil
}

// OnStop implements service.Service.
//
// All channels must be closed by OpenChannel() callers before stopping the
// router, to prevent blocked channel sends in reactors. Channels are not closed
// here, since that would cause any reactor senders to panic, so it is the
// sender's responsibility.
func (r *Router) OnStop() {
	// Signal router shutdown.
	close(r.stopCh)

	// Close transport listeners (unblocks Accept calls).
	for _, transport := range r.transports {
		if err := transport.Close(); err != nil {
			r.logger.Error("failed to close transport", "transport", transport, "err", err)
		}
	}

	// Collect all remaining queues, and wait for them to close.
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

	for _, q := range queues {
		<-q.closed()
	}
}

// stopCtx returns a new context that is canceled when the router stops.
func (r *Router) stopCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		<-r.stopCh
		cancel()
	}()

	return ctx
}
