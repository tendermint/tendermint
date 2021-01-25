package p2p

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
)

// Router manages peer connections and routes messages between peers and reactor
// channels. This is an early prototype.
//
// Channels are registered via OpenChannel(). When called, we register an input
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
	transports  map[Protocol]Transport
	peerManager *PeerManager

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

// NewRouter creates a new Router, dialing the given peers.
//
// FIXME: providing protocol/transport maps is cumbersome in tests, we should
// consider adding Protocols() to the Transport interface instead and register
// protocol/transport mappings automatically on a first-come basis.
func NewRouter(logger log.Logger, peerManager *PeerManager, transports map[Protocol]Transport) *Router {
	router := &Router{
		logger:          logger,
		transports:      transports,
		peerManager:     peerManager,
		stopCh:          make(chan struct{}),
		channelQueues:   map[ChannelID]queue{},
		channelMessages: map[ChannelID]proto.Message{},
		peerQueues:      map[NodeID]queue{},
	}
	router.BaseService = service.NewBaseService(logger, "router", router)
	return router
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
			// FIXME: We just disconnect the peer for now
			r.logger.Error("peer error, disconnecting", "peer", peerError.PeerID, "err", peerError.Err)
			r.peerMtx.RLock()
			peerQueue, ok := r.peerQueues[peerError.PeerID]
			r.peerMtx.RUnlock()
			if ok {
				peerQueue.close()
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
		conn, err := transport.Accept(ctx)
		switch err {
		case nil:
		case ErrTransportClosed{}, io.EOF, context.Canceled:
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

			peerID := conn.NodeInfo().NodeID
			if err := r.peerManager.Accepted(peerID); err != nil {
				r.logger.Error("failed to accept connection", "peer", peerID, "err", err)
				return
			}

			queue := newFIFOQueue()
			r.peerMtx.Lock()
			r.peerQueues[peerID] = queue
			r.peerMtx.Unlock()
			r.peerManager.Ready(peerID)

			defer func() {
				r.peerMtx.Lock()
				delete(r.peerQueues, peerID)
				r.peerMtx.Unlock()
				queue.close()
				if err := r.peerManager.Disconnected(peerID); err != nil {
					r.logger.Error("failed to disconnect peer", "peer", peerID, "err", err)
				}
			}()

			r.routePeer(peerID, conn, queue)
		}()
	}
}

// dialPeers maintains outbound connections to peers.
func (r *Router) dialPeers() {
	ctx := r.stopCtx()
	for {
		peerID, address, err := r.peerManager.DialNext(ctx)
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
			conn, err := r.dialPeer(address)
			if err != nil {
				r.logger.Error("failed to dial peer", "peer", peerID)
				if err = r.peerManager.DialFailed(peerID, address); err != nil {
					r.logger.Error("failed to report dial failure", "peer", peerID, "err", err)
				}
				return
			}
			defer conn.Close()

			if err = r.peerManager.Dialed(peerID, address); err != nil {
				r.logger.Error("failed to dial peer", "peer", peerID, "err", err)
				return
			}

			queue := newFIFOQueue()
			r.peerMtx.Lock()
			r.peerQueues[peerID] = queue
			r.peerMtx.Unlock()
			r.peerManager.Ready(peerID)

			defer func() {
				r.peerMtx.Lock()
				delete(r.peerQueues, peerID)
				r.peerMtx.Unlock()
				queue.close()
				if err := r.peerManager.Disconnected(peerID); err != nil {
					r.logger.Error("failed to disconnect peer", "peer", peerID, "err", err)
				}
			}()

			r.routePeer(peerID, conn, queue)
		}()
	}
}

// dialPeer attempts to connect to a peer.
func (r *Router) dialPeer(address PeerAddress) (Connection, error) {
	ctx := context.Background()

	resolveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	r.logger.Info("resolving peer address", "address", address)

	endpoints, err := address.Resolve(resolveCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address %q: %w", address, err)
	}

	for _, endpoint := range endpoints {
		t, ok := r.transports[endpoint.Protocol]
		if !ok {
			r.logger.Error("no transport found for protocol", "protocol", endpoint.Protocol)
			continue
		}

		dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		conn, err := t.Dial(dialCtx, endpoint)
		if err != nil {
			r.logger.Error("failed to dial endpoint", "endpoint", endpoint, "err", err)
		} else {
			r.logger.Info("connected to peer", "peer", address.ID, "endpoint", endpoint)
			return conn, nil
		}
	}
	return nil, fmt.Errorf("failed to connect to peer via %q", address)
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
		queue, ok := r.channelQueues[ChannelID(chID)]
		messageType := r.channelMessages[ChannelID(chID)]
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
		// FIXME: ReceiveMessage() should return ChannelID.
		case queue.enqueue() <- Envelope{channelID: ChannelID(chID), From: peerID, Message: msg}:
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

			// FIXME: SendMessage() should take ChannelID.
			_, err = conn.SendMessage(byte(envelope.channelID), bz)
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
