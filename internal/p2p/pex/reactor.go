package pex

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/conn"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	protop2p "github.com/tendermint/tendermint/proto/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

var (
	_ service.Service = (*Reactor)(nil)
	_ p2p.Wrapper     = (*protop2p.PexMessage)(nil)
)

const (
	// PexChannel is a channel for PEX messages
	PexChannel = 0x00

	// over-estimate of max NetAddress size
	// hexID (40) + IP (16) + Port (2) + Name (100) ...
	// NOTE: dont use massive DNS name ..
	maxAddressSize = 256

	// max addresses returned by GetSelection
	// NOTE: this must match "maxMsgSize"
	maxGetSelection = 250

	// NOTE: amplification factor!
	// small request results in up to maxMsgSize response
	maxMsgSize = maxAddressSize * maxGetSelection

	// the minimum time one peer can send another request to the same peer
	minReceiveRequestInterval = 100 * time.Millisecond

	// the maximum amount of addresses that can be included in a response
	maxAddresses = 100

	// How long to wait when there are no peers available before trying again
	noAvailablePeersWaitPeriod = 1 * time.Second

	// indicates the ping rate of the pex reactor when the peer store is full.
	// The reactor should still look to add new peers in order to flush out low
	// scoring peers that are still in the peer store
	fullCapacityInterval = 10 * time.Minute
)

// TODO: We should decide whether we want channel descriptors to be housed
// within each reactor (as they are now) or, considering that the reactor doesn't
// really need to care about the channel descriptors, if they should be housed
// in the node module.
func ChannelDescriptor() *conn.ChannelDescriptor {
	return &conn.ChannelDescriptor{
		ID:                  PexChannel,
		MessageType:         new(protop2p.PexMessage),
		Priority:            1,
		SendQueueCapacity:   10,
		RecvMessageCapacity: maxMsgSize,
		RecvBufferCapacity:  128,
		Name:                "pex",
	}
}

// The peer exchange or PEX reactor supports the peer manager by sending
// requests to other peers for addresses that can be given to the peer manager
// and at the same time advertises addresses to peers that need more.
//
// The reactor is able to tweak the intensity of it's search by decreasing or
// increasing the interval between each request. It tracks connected peers via
// a linked list, sending a request to the node at the front of the list and
// adding it to the back of the list once a response is received.
type Reactor struct {
	service.BaseService
	logger log.Logger

	peerManager *p2p.PeerManager
	chCreator   p2p.ChannelCreator
	peerEvents  p2p.PeerEventSubscriber
	// list of available peers to loop through and send peer requests to
	availablePeers map[types.NodeID]struct{}

	mtx sync.RWMutex

	// requestsSent keeps track of which peers the PEX reactor has sent requests
	// to. This prevents the sending of spurious responses.
	// NOTE: If a node never responds, they will remain in this map until a
	// peer down status update is sent
	requestsSent map[types.NodeID]struct{}

	// lastReceivedRequests keeps track of when peers send a request to prevent
	// peers from sending requests too often (as defined by
	// minReceiveRequestInterval).
	lastReceivedRequests map[types.NodeID]time.Time

	// the total number of unique peers added
	totalPeers int
}

// NewReactor returns a reference to a new reactor.
func NewReactor(
	logger log.Logger,
	peerManager *p2p.PeerManager,
	channelCreator p2p.ChannelCreator,
	peerEvents p2p.PeerEventSubscriber,
) *Reactor {
	r := &Reactor{
		logger:               logger,
		peerManager:          peerManager,
		chCreator:            channelCreator,
		peerEvents:           peerEvents,
		availablePeers:       make(map[types.NodeID]struct{}),
		requestsSent:         make(map[types.NodeID]struct{}),
		lastReceivedRequests: make(map[types.NodeID]time.Time),
	}

	r.BaseService = *service.NewBaseService(logger, "PEX", r)
	return r
}

// OnStart starts separate go routines for each p2p Channel and listens for
// envelopes on each. In addition, it also listens for peer updates and handles
// messages on that p2p channel accordingly. The caller must be sure to execute
// OnStop to ensure the outbound p2p Channels are closed.
func (r *Reactor) OnStart(ctx context.Context) error {
	channel, err := r.chCreator(ctx, ChannelDescriptor())
	if err != nil {
		return err
	}

	peerUpdates := r.peerEvents(ctx)
	go r.processPexCh(ctx, channel)
	go r.processPeerUpdates(ctx, peerUpdates)
	return nil
}

// OnStop stops the reactor by signaling to all spawned goroutines to exit and
// blocking until they all exit.
func (r *Reactor) OnStop() {}

// processPexCh implements a blocking event loop where we listen for p2p
// Envelope messages from the pexCh.
func (r *Reactor) processPexCh(ctx context.Context, pexCh p2p.Channel) {
	incoming := make(chan *p2p.Envelope)
	go func() {
		defer close(incoming)
		iter := pexCh.Receive(ctx)
		for iter.Next(ctx) {
			select {
			case <-ctx.Done():
				return
			case incoming <- iter.Envelope():
			}
		}
	}()

	// Initially, we will request peers quickly to bootstrap.  This duration
	// will be adjusted upward as knowledge of the network grows.
	var nextPeerRequest = minReceiveRequestInterval

	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		timer.Reset(nextPeerRequest)

		select {
		case <-ctx.Done():
			return

		case <-timer.C:
			// Send a request for more peer addresses.
			if err := r.sendRequestForPeers(ctx, pexCh); err != nil {
				return
				// TODO(creachadair): Do we really want to stop processing the PEX
				// channel just because of an error here?
			}

			// Note we do not update the poll timer upon making a request, only
			// when we receive an update that updates our priors.

		case envelope, ok := <-incoming:
			if !ok {
				return // channel closed
			}

			// A request from another peer, or a response to one of our requests.
			dur, err := r.handlePexMessage(ctx, envelope, pexCh)
			if err != nil {
				r.logger.Error("failed to process message", "ch_id", envelope.ChannelID, "envelope", envelope, "err", err)
				if serr := pexCh.SendError(ctx, p2p.PeerError{
					NodeID: envelope.From,
					Err:    err,
				}); serr != nil {
					return
				}
			} else if dur != 0 {
				// We got a useful result; update the poll timer.
				nextPeerRequest = dur
			}
		}
	}
}

// processPeerUpdates initiates a blocking process where we listen for and handle
// PeerUpdate messages. When the reactor is stopped, we will catch the signal and
// close the p2p PeerUpdatesCh gracefully.
func (r *Reactor) processPeerUpdates(ctx context.Context, peerUpdates *p2p.PeerUpdates) {
	for {
		select {
		case <-ctx.Done():
			return
		case peerUpdate := <-peerUpdates.Updates():
			r.processPeerUpdate(peerUpdate)
		}
	}
}

// handlePexMessage handles envelopes sent from peers on the PexChannel.
// If an update was received, a new polling interval is returned; otherwise the
// duration is 0.
func (r *Reactor) handlePexMessage(ctx context.Context, envelope *p2p.Envelope, pexCh p2p.Channel) (time.Duration, error) {
	logger := r.logger.With("peer", envelope.From)

	switch msg := envelope.Message.(type) {
	case *protop2p.PexRequest:
		// Verify that this peer hasn't sent us another request too recently.
		if err := r.markPeerRequest(envelope.From); err != nil {
			return 0, err
		}

		// Fetch peers from the peer manager, convert NodeAddresses into URL
		// strings, and send them back to the caller.
		nodeAddresses := r.peerManager.Advertise(envelope.From, maxAddresses)
		pexAddresses := make([]protop2p.PexAddress, len(nodeAddresses))
		for idx, addr := range nodeAddresses {
			pexAddresses[idx] = protop2p.PexAddress{
				URL: addr.String(),
			}
		}
		return 0, pexCh.Send(ctx, p2p.Envelope{
			To:      envelope.From,
			Message: &protop2p.PexResponse{Addresses: pexAddresses},
		})

	case *protop2p.PexResponse:
		// Verify that this response corresponds to one of our pending requests.
		if err := r.markPeerResponse(envelope.From); err != nil {
			return 0, err
		}

		// Verify that the response does not exceed the safety limit.
		if len(msg.Addresses) > maxAddresses {
			return 0, fmt.Errorf("peer sent too many addresses (%d > maxiumum %d)",
				len(msg.Addresses), maxAddresses)
		}

		var numAdded int
		for _, pexAddress := range msg.Addresses {
			peerAddress, err := p2p.ParseNodeAddress(pexAddress.URL)
			if err != nil {
				continue
			}
			added, err := r.peerManager.Add(peerAddress)
			if err != nil {
				logger.Error("failed to add PEX address", "address", peerAddress, "err", err)
				continue
			}
			if added {
				numAdded++
				logger.Debug("added PEX address", "address", peerAddress)
			}
		}

		return r.calculateNextRequestTime(numAdded), nil

	default:
		return 0, fmt.Errorf("received unknown message: %T", msg)
	}
}

// processPeerUpdate processes a PeerUpdate. For added peers, PeerStatusUp, we
// send a request for addresses.
func (r *Reactor) processPeerUpdate(peerUpdate p2p.PeerUpdate) {
	r.logger.Debug("received PEX peer update", "peer", peerUpdate.NodeID, "status", peerUpdate.Status)

	r.mtx.Lock()
	defer r.mtx.Unlock()

	switch peerUpdate.Status {
	case p2p.PeerStatusUp:
		r.availablePeers[peerUpdate.NodeID] = struct{}{}
	case p2p.PeerStatusDown:
		delete(r.availablePeers, peerUpdate.NodeID)
		delete(r.requestsSent, peerUpdate.NodeID)
		delete(r.lastReceivedRequests, peerUpdate.NodeID)
	default:
	}
}

// sendRequestForPeers chooses a peer from the set of available peers and sends
// that peer a request for more peer addresses. The chosen peer is moved into
// the requestsSent bucket so that we will not attempt to contact them again
// until they've replied or updated.
func (r *Reactor) sendRequestForPeers(ctx context.Context, pexCh p2p.Channel) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if len(r.availablePeers) == 0 {
		// no peers are available
		r.logger.Debug("no available peers to send a PEX request to (retrying)")
		return nil
	}

	// Select an arbitrary peer from the available set.
	var peerID types.NodeID
	for peerID = range r.availablePeers {
		break
	}

	if err := pexCh.Send(ctx, p2p.Envelope{
		To:      peerID,
		Message: &protop2p.PexRequest{},
	}); err != nil {
		return err
	}

	// Move the peer from available to pending.
	delete(r.availablePeers, peerID)
	r.requestsSent[peerID] = struct{}{}

	return nil
}

// calculateNextRequestTime selects how long we should wait before attempting
// to send out another request for peer addresses.
//
// This implements a simplified proportional control mechanism to poll more
// often when our knowledge of the network is incomplete, and less often as our
// knowledge grows. To estimate our knowledge of the network, we use the
// fraction of "new" peers (addresses we have not previously seen) to the total
// so far observed. When we first join the network, this fraction will be close
// to 1, meaning most new peers are "new" to us, and as we discover more peers,
// the fraction will go toward zero.
//
// The minimum interval will be minReceiveRequestInterval to ensure we will not
// request from any peer more often than we would allow them to do from us.
func (r *Reactor) calculateNextRequestTime(added int) time.Duration {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	r.totalPeers += added

	// If the peer store is nearly full, wait the maximum interval.
	if ratio := r.peerManager.PeerRatio(); ratio >= 0.95 {
		r.logger.Debug("Peer manager is nearly full",
			"sleep_period", fullCapacityInterval,
			"ratio", ratio)
		return fullCapacityInterval
	}

	// If there are no available peers to query, poll less aggressively.
	if len(r.availablePeers) == 0 {
		r.logger.Debug("No available peers to send a PEX request",
			"sleep_period", noAvailablePeersWaitPeriod)
		return noAvailablePeersWaitPeriod
	}

	// Reaching here, there are available peers to query and the peer store
	// still has space. Estimate our knowledge of the network from the latest
	// update and choose a new interval.
	base := float64(minReceiveRequestInterval) / float64(len(r.availablePeers))
	multiplier := float64(r.totalPeers+1) / float64(added+1) // +1 to avert zero division
	return time.Duration(base*multiplier*multiplier) + minReceiveRequestInterval
}

func (r *Reactor) markPeerRequest(peer types.NodeID) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if lastRequestTime, ok := r.lastReceivedRequests[peer]; ok {
		if d := time.Since(lastRequestTime); d < minReceiveRequestInterval {
			return fmt.Errorf("peer %v sent PEX request too soon (%v < minimum %v)",
				peer, d, minReceiveRequestInterval)
		}
	}
	r.lastReceivedRequests[peer] = time.Now()
	return nil
}

func (r *Reactor) markPeerResponse(peer types.NodeID) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	// check if a request to this peer was sent
	if _, ok := r.requestsSent[peer]; !ok {
		return fmt.Errorf("peer sent a PEX response when none was requested (%v)", peer)
	}
	delete(r.requestsSent, peer)
	// attach to the back of the list so that the peer can be used again for
	// future requests

	r.availablePeers[peer] = struct{}{}
	return nil
}
