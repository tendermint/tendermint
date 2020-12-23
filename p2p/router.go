package p2p

import (
	"context"
	"errors"
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
// On startup, the router spawns off two primary goroutines that maintain
// connections to peers and run for the lifetime of the router:
//
//   Router.dialPeers(): in a loop, asks the peerStore to dispense an
//   eligible peer to connect to, and attempts to resolve and dial each
//   address until successful.
//
//   Router.acceptPeers(): in a loop, waits for the next inbound connection
//   from a peer, and attempts to claim it in the peerStore.
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
	logger     log.Logger
	transports map[Protocol]Transport
	store      *peerStore

	// FIXME: Consider using sync.Map.
	peerMtx    sync.RWMutex
	peerQueues map[NodeID]queue

	// FIXME: We don't strictly need to use a mutex for this if we seal the
	// channels on router start. This depends on whether we want to allow
	// dynamic channels in the future.
	channelMtx      sync.RWMutex
	channelQueues   map[ChannelID]queue
	channelMessages map[ChannelID]proto.Message

	peerUpdatesCh   chan PeerUpdate
	peerUpdatesMtx  sync.RWMutex
	peerUpdatesSubs map[*PeerUpdatesCh]*PeerUpdatesCh // keyed by struct identity (address)

	// stopCh is used to signal router shutdown, by closing the channel.
	stopCh chan struct{}
}

// NewRouter creates a new Router, dialing the given peers.
//
// FIXME: providing protocol/transport maps is cumbersome in tests, we should
// consider adding Protocols() to the Transport interface instead and register
// protocol/transport mappings automatically on a first-come basis.
func NewRouter(logger log.Logger, transports map[Protocol]Transport, peers []PeerAddress) *Router {
	router := &Router{
		logger:          logger,
		transports:      transports,
		store:           newPeerStore(),
		stopCh:          make(chan struct{}),
		channelQueues:   map[ChannelID]queue{},
		channelMessages: map[ChannelID]proto.Message{},
		peerQueues:      map[NodeID]queue{},
		peerUpdatesCh:   make(chan PeerUpdate, 1),
		peerUpdatesSubs: map[*PeerUpdatesCh]*PeerUpdatesCh{},
	}
	router.BaseService = service.NewBaseService(logger, "router", router)

	for _, address := range peers {
		if err := router.store.Add(address); err != nil {
			logger.Error("failed to add peer", "address", address, "err", err)
		}
	}

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
		return nil, fmt.Errorf("channel %v already exist", id)
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
			envelope.channel = channel.id

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
					r.logger.Error("dropping message for non-connected peer", "peer", envelope.To)
					continue
				}

				select {
				case peerQueue.enqueue() <- envelope:
				case <-peerQueue.closed():
					r.logger.Error("dropping message for non-connected peer", "peer", envelope.To)
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
	for {
		select {
		case <-r.stopCh:
			return
		default:
		}

		conn, err := transport.Accept(context.Background())
		switch err {
		case nil:
		case ErrTransportClosed{}, io.EOF:
			r.logger.Info("transport closed, stopping accept routine", "transport", transport)
			return
		default:
			r.logger.Error("failed to accept connection", "transport", transport, "err", err)
			return
		}

		peerID := conn.NodeInfo().NodeID
		if r.store.Claim(peerID) == nil {
			r.logger.Error("already connected to peer, rejecting connection", "peer", peerID)
			_ = conn.Close()
			continue
		}

		queue := newFIFOQueue()
		r.peerMtx.Lock()
		r.peerQueues[peerID] = queue
		r.peerMtx.Unlock()

		go func() {
			defer func() {
				r.peerMtx.Lock()
				delete(r.peerQueues, peerID)
				r.peerMtx.Unlock()
				queue.close()
				_ = conn.Close()
				r.store.Return(peerID)
			}()

			r.routePeer(peerID, conn, queue)
		}()
	}
}

// dialPeers maintains outbound connections to peers.
func (r *Router) dialPeers() {
	for {
		select {
		case <-r.stopCh:
			return
		default:
		}

		peer := r.store.Dispense()
		if peer == nil {
			r.logger.Debug("no eligible peers, sleeping")
			select {
			case <-time.After(time.Second):
				continue
			case <-r.stopCh:
				return
			}
		}

		go func() {
			defer r.store.Return(peer.ID)
			conn, err := r.dialPeer(peer)
			if err != nil {
				r.logger.Error("failed to dial peer, will retry", "peer", peer.ID)
				return
			}
			defer conn.Close()

			queue := newFIFOQueue()
			defer queue.close()
			r.peerMtx.Lock()
			r.peerQueues[peer.ID] = queue
			r.peerMtx.Unlock()

			defer func() {
				r.peerMtx.Lock()
				delete(r.peerQueues, peer.ID)
				r.peerMtx.Unlock()
			}()

			r.routePeer(peer.ID, conn, queue)
		}()
	}
}

// dialPeer attempts to connect to a peer.
func (r *Router) dialPeer(peer *storePeer) (Connection, error) {
	ctx := context.Background()

	for _, address := range peer.Addresses {
		resolveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		r.logger.Info("resolving peer address", "peer", peer.ID, "address", address)
		endpoints, err := address.Resolve(resolveCtx)
		if err != nil {
			r.logger.Error("failed to resolve address", "address", address, "err", err)
			continue
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
				r.logger.Error("failed to dial endpoint", "endpoint", endpoint)
			} else {
				r.logger.Info("connected to peer", "peer", peer.ID, "endpoint", endpoint)
				return conn, nil
			}
		}
	}
	return nil, errors.New("failed to connect to peer")
}

// routePeer routes inbound messages from a peer to channels, and also sends
// outbound queued messages to the peer. It will close the connection and send
// queue, using this as a signal to coordinate the internal receivePeer() and
// sendPeer() goroutines.
func (r *Router) routePeer(peerID NodeID, conn Connection, sendQueue queue) {
	// FIXME: Peer updates should probably be handled by the peer store.
	r.peerUpdatesCh <- PeerUpdate{
		PeerID: peerID,
		Status: PeerStatusUp,
	}
	defer func() {
		r.peerUpdatesCh <- PeerUpdate{
			PeerID: peerID,
			Status: PeerStatusDown,
		}
	}()

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
		case queue.enqueue() <- Envelope{channel: ChannelID(chID), From: peerID, Message: msg}:
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
			_, err = conn.SendMessage(byte(envelope.channel), bz)
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

// SubscribePeerUpdates creates a new peer updates subscription. The caller must
// consume the peer updates in a timely fashion, since delivery is guaranteed and
// will block peer connection/disconnection otherwise.
func (r *Router) SubscribePeerUpdates() (*PeerUpdatesCh, error) {
	peerUpdates := NewPeerUpdates()
	r.peerUpdatesMtx.Lock()
	r.peerUpdatesSubs[peerUpdates] = peerUpdates
	r.peerUpdatesMtx.Unlock()

	go func() {
		select {
		case <-peerUpdates.Done():
			r.peerUpdatesMtx.Lock()
			delete(r.peerUpdatesSubs, peerUpdates)
			r.peerUpdatesMtx.Unlock()
		case <-r.stopCh:
		}
	}()
	return peerUpdates, nil
}

// broadcastPeerUpdates broadcasts peer updates received from the router
// to all subscriptions.
func (r *Router) broadcastPeerUpdates() {
	for {
		select {
		case peerUpdate := <-r.peerUpdatesCh:
			subs := []*PeerUpdatesCh{}
			r.peerUpdatesMtx.RLock()
			for _, sub := range r.peerUpdatesSubs {
				subs = append(subs, sub)
			}
			r.peerUpdatesMtx.RUnlock()

			for _, sub := range subs {
				select {
				case sub.updatesCh <- peerUpdate:
				case <-sub.doneCh:
				case <-r.stopCh:
					return
				}
			}

		case <-r.stopCh:
			return
		}
	}
}

// OnStart implements service.Service.
func (r *Router) OnStart() error {
	go r.broadcastPeerUpdates()
	go r.dialPeers()
	for _, transport := range r.transports {
		go r.acceptPeers(transport)
	}
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
