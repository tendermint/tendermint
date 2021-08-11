package pex

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/conn"
	"github.com/tendermint/tendermint/libs/log"
	tmmath "github.com/tendermint/tendermint/libs/math"
	"github.com/tendermint/tendermint/libs/service"
	protop2p "github.com/tendermint/tendermint/proto/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

var (
	_ service.Service = (*ReactorV2)(nil)
	_ p2p.Wrapper     = (*protop2p.PexMessage)(nil)
)

// TODO: Consolidate with params file.
// See https://github.com/tendermint/tendermint/issues/6371
const (
	// the minimum time one peer can send another request to the same peer
	minReceiveRequestInterval = 100 * time.Millisecond

	// the maximum amount of addresses that can be included in a response
	maxAddresses uint16 = 100

	// allocated time to resolve a node address into a set of endpoints
	resolveTimeout = 3 * time.Second

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
func ChannelDescriptor() conn.ChannelDescriptor {
	return conn.ChannelDescriptor{
		ID:                  PexChannel,
		Priority:            1,
		SendQueueCapacity:   10,
		RecvMessageCapacity: maxMsgSize,
		RecvBufferCapacity:  32,
		MaxSendBytes:        200,
	}
}

// ReactorV2 is a PEX reactor for the new P2P stack. The legacy reactor
// is Reactor.
//
// FIXME: Rename this when Reactor is removed, and consider moving to p2p/.
//
// The peer exchange or PEX reactor supports the peer manager by sending
// requests to other peers for addresses that can be given to the peer manager
// and at the same time advertises addresses to peers that need more.
//
// The reactor is able to tweak the intensity of it's search by decreasing or
// increasing the interval between each request. It tracks connected peers via
// a linked list, sending a request to the node at the front of the list and
// adding it to the back of the list once a response is received.
type ReactorV2 struct {
	service.BaseService

	peerManager *p2p.PeerManager
	pexCh       *p2p.Channel
	peerUpdates *p2p.PeerUpdates
	closeCh     chan struct{}

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

	// the time when another request will be sent
	nextRequestTime time.Time

	// keep track of how many new peers to existing peers we have received to
	// extrapolate the size of the network
	newPeers   uint32
	totalPeers uint32

	// discoveryRatio is the inverse ratio of new peers to old peers squared.
	// This is multiplied by the minimum duration to calculate how long to wait
	// between each request.
	discoveryRatio float32
}

// NewReactor returns a reference to a new reactor.
func NewReactorV2(
	logger log.Logger,
	peerManager *p2p.PeerManager,
	pexCh *p2p.Channel,
	peerUpdates *p2p.PeerUpdates,
) *ReactorV2 {

	r := &ReactorV2{
		peerManager:          peerManager,
		pexCh:                pexCh,
		peerUpdates:          peerUpdates,
		closeCh:              make(chan struct{}),
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
func (r *ReactorV2) OnStart() error {
	go r.processPexCh()
	go r.processPeerUpdates()
	return nil
}

// OnStop stops the reactor by signaling to all spawned goroutines to exit and
// blocking until they all exit.
func (r *ReactorV2) OnStop() {
	// Close closeCh to signal to all spawned goroutines to gracefully exit. All
	// p2p Channels should execute Close().
	close(r.closeCh)

	// Wait for all p2p Channels to be closed before returning. This ensures we
	// can easily reason about synchronization of all p2p Channels and ensure no
	// panics will occur.
	<-r.pexCh.Done()
	<-r.peerUpdates.Done()
}

// processPexCh implements a blocking event loop where we listen for p2p
// Envelope messages from the pexCh.
func (r *ReactorV2) processPexCh() {
	defer r.pexCh.Close()

	for {
		select {
		case <-r.closeCh:
			r.Logger.Debug("stopped listening on PEX channel; closing...")
			return

		// outbound requests for new peers
		case <-r.waitUntilNextRequest():
			r.sendRequestForPeers()

		// inbound requests for new peers or responses to requests sent by this
		// reactor
		case envelope := <-r.pexCh.In:
			if err := r.handleMessage(r.pexCh.ID, envelope); err != nil {
				r.Logger.Error("failed to process message", "ch_id", r.pexCh.ID, "envelope", envelope, "err", err)
				r.pexCh.Error <- p2p.PeerError{
					NodeID: envelope.From,
					Err:    err,
				}
			}
		}
	}
}

// processPeerUpdates initiates a blocking process where we listen for and handle
// PeerUpdate messages. When the reactor is stopped, we will catch the signal and
// close the p2p PeerUpdatesCh gracefully.
func (r *ReactorV2) processPeerUpdates() {
	defer r.peerUpdates.Close()

	for {
		select {
		case peerUpdate := <-r.peerUpdates.Updates():
			r.processPeerUpdate(peerUpdate)

		case <-r.closeCh:
			r.Logger.Debug("stopped listening on peer updates channel; closing...")
			return
		}
	}
}

// handlePexMessage handles envelopes sent from peers on the PexChannel.
func (r *ReactorV2) handlePexMessage(envelope p2p.Envelope) error {
	logger := r.Logger.With("peer", envelope.From)

	switch msg := envelope.Message.(type) {

	case *protop2p.PexRequest:
		// Check if the peer hasn't sent a prior request too close to this one
		// in time.
		if err := r.markPeerRequest(envelope.From); err != nil {
			return err
		}

		// parse and send the legacy PEX addresses
		pexAddresses := r.resolve(r.peerManager.Advertise(envelope.From, maxAddresses))
		r.pexCh.Out <- p2p.Envelope{
			To:      envelope.From,
			Message: &protop2p.PexResponse{Addresses: pexAddresses},
		}

	case *protop2p.PexResponse:
		// check if the response matches a request that was made to that peer
		if err := r.markPeerResponse(envelope.From); err != nil {
			return err
		}

		// check the size of the response
		if len(msg.Addresses) > int(maxAddresses) {
			return fmt.Errorf("peer sent too many addresses (max: %d, got: %d)",
				maxAddresses,
				len(msg.Addresses),
			)
		}

		for _, pexAddress := range msg.Addresses {
			// no protocol is prefixed so we assume the default (mconn)
			peerAddress, err := p2p.ParseNodeAddress(
				fmt.Sprintf("%s@%s:%d", pexAddress.ID, pexAddress.IP, pexAddress.Port))
			if err != nil {
				continue
			}
			added, err := r.peerManager.Add(peerAddress)
			if err != nil {
				logger.Error("failed to add PEX address", "address", peerAddress, "err", err)
			}
			if added {
				r.newPeers++
				logger.Debug("added PEX address", "address", peerAddress)
			}
			r.totalPeers++
		}

	// V2 PEX MESSAGES
	case *protop2p.PexRequestV2:
		// check if the peer hasn't sent a prior request too close to this one
		// in time
		if err := r.markPeerRequest(envelope.From); err != nil {
			return err
		}

		// request peers from the peer manager and parse the NodeAddresses into
		// URL strings
		nodeAddresses := r.peerManager.Advertise(envelope.From, maxAddresses)
		pexAddressesV2 := make([]protop2p.PexAddressV2, len(nodeAddresses))
		for idx, addr := range nodeAddresses {
			pexAddressesV2[idx] = protop2p.PexAddressV2{
				URL: addr.String(),
			}
		}
		r.pexCh.Out <- p2p.Envelope{
			To:      envelope.From,
			Message: &protop2p.PexResponseV2{Addresses: pexAddressesV2},
		}

	case *protop2p.PexResponseV2:
		// check if the response matches a request that was made to that peer
		if err := r.markPeerResponse(envelope.From); err != nil {
			return err
		}

		// check the size of the response
		if len(msg.Addresses) > int(maxAddresses) {
			return fmt.Errorf("peer sent too many addresses (max: %d, got: %d)",
				maxAddresses,
				len(msg.Addresses),
			)
		}

		for _, pexAddress := range msg.Addresses {
			peerAddress, err := p2p.ParseNodeAddress(pexAddress.URL)
			if err != nil {
				continue
			}
			added, err := r.peerManager.Add(peerAddress)
			if err != nil {
				logger.Error("failed to add V2 PEX address", "address", peerAddress, "err", err)
			}
			if added {
				r.newPeers++
				logger.Debug("added V2 PEX address", "address", peerAddress)
			}
			r.totalPeers++
		}

	default:
		return fmt.Errorf("received unknown message: %T", msg)
	}

	return nil
}

// resolve resolves a set of peer addresses into PEX addresses.
//
// FIXME: This is necessary because the current PEX protocol only supports
// IP/port pairs, while the P2P stack uses NodeAddress URLs. The PEX protocol
// should really use URLs too, to exchange DNS names instead of IPs and allow
// different transport protocols (e.g. QUIC and MemoryTransport).
//
// FIXME: We may want to cache and parallelize this, but for now we'll just rely
// on the operating system to cache it for us.
func (r *ReactorV2) resolve(addresses []p2p.NodeAddress) []protop2p.PexAddress {
	limit := len(addresses)
	pexAddresses := make([]protop2p.PexAddress, 0, limit)

	for _, address := range addresses {
		ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
		endpoints, err := address.Resolve(ctx)
		r.Logger.Debug("resolved node address", "endpoints", endpoints)
		cancel()

		if err != nil {
			r.Logger.Debug("failed to resolve address", "address", address, "err", err)
			continue
		}

		for _, endpoint := range endpoints {
			r.Logger.Debug("checking endpint", "IP", endpoint.IP, "Port", endpoint.Port)
			if len(pexAddresses) >= limit {
				return pexAddresses

			} else if endpoint.IP != nil {
				r.Logger.Debug("appending pex address")
				// PEX currently only supports IP-networked transports (as
				// opposed to e.g. p2p.MemoryTransport).
				//
				// FIXME: as the PEX address contains no information about the
				// protocol, we jam this into the ID. We won't need to this once
				// we support URLs
				pexAddresses = append(pexAddresses, protop2p.PexAddress{
					ID:   string(address.NodeID),
					IP:   endpoint.IP.String(),
					Port: uint32(endpoint.Port),
				})
			}
		}
	}

	return pexAddresses
}

// handleMessage handles an Envelope sent from a peer on a specific p2p Channel.
// It will handle errors and any possible panics gracefully. A caller can handle
// any error returned by sending a PeerError on the respective channel.
func (r *ReactorV2) handleMessage(chID p2p.ChannelID, envelope p2p.Envelope) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("panic in processing message: %v", e)
			r.Logger.Error(
				"recovering from processing message panic",
				"err", err,
				"stack", string(debug.Stack()),
			)
		}
	}()

	r.Logger.Debug("received PEX message", "peer", envelope.From)

	switch chID {
	case p2p.ChannelID(PexChannel):
		err = r.handlePexMessage(envelope)

	default:
		err = fmt.Errorf("unknown channel ID (%d) for envelope (%v)", chID, envelope)
	}

	return err
}

// processPeerUpdate processes a PeerUpdate. For added peers, PeerStatusUp, we
// send a request for addresses.
func (r *ReactorV2) processPeerUpdate(peerUpdate p2p.PeerUpdate) {
	r.Logger.Debug("received PEX peer update", "peer", peerUpdate.NodeID, "status", peerUpdate.Status)

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

func (r *ReactorV2) waitUntilNextRequest() <-chan time.Time {
	return time.After(time.Until(r.nextRequestTime))
}

// sendRequestForPeers pops the first peerID off the list and sends the
// peer a request for more peer addresses. The function then moves the
// peer into the requestsSent bucket and calculates when the next request
// time should be
func (r *ReactorV2) sendRequestForPeers() {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if len(r.availablePeers) == 0 {
		// no peers are available
		r.Logger.Debug("no available peers to send request to, waiting...")
		r.nextRequestTime = time.Now().Add(noAvailablePeersWaitPeriod)

		return
	}
	var peerID types.NodeID

	// use range to get a random peer.
	for peerID = range r.availablePeers {
		break
	}

	// The node accommodates for both pex systems
	if r.isLegacyPeer(peerID) {
		r.pexCh.Out <- p2p.Envelope{
			To:      peerID,
			Message: &protop2p.PexRequest{},
		}
	} else {
		r.pexCh.Out <- p2p.Envelope{
			To:      peerID,
			Message: &protop2p.PexRequestV2{},
		}
	}

	// remove the peer from the abvailable peers list and mark it in the requestsSent map
	delete(r.availablePeers, peerID)
	r.requestsSent[peerID] = struct{}{}

	r.calculateNextRequestTime()
	r.Logger.Debug("peer request sent", "next_request_time", r.nextRequestTime)
}

// calculateNextRequestTime implements something of a proportional controller
// to estimate how often the reactor should be requesting new peer addresses.
// The dependent variable in this calculation is the ratio of new peers to
// all peers that the reactor receives. The interval is thus calculated as the
// inverse squared. In the beginning, all peers should be new peers.
// We  expect this ratio to be near 1 and thus the interval to be as short
// as possible. As the node becomes more familiar with the network the ratio of
// new nodes will plummet to a very small number, meaning the interval expands
// to its upper bound.
// CONTRACT: Must use a write lock as nextRequestTime is updated
func (r *ReactorV2) calculateNextRequestTime() {
	// check if the peer store is full. If so then there is no need
	// to send peer requests too often
	if ratio := r.peerManager.PeerRatio(); ratio >= 0.95 {
		r.Logger.Debug("peer manager near full ratio, sleeping...",
			"sleep_period", fullCapacityInterval, "ratio", ratio)
		r.nextRequestTime = time.Now().Add(fullCapacityInterval)
		return
	}

	// baseTime represents the shortest interval that we can send peer requests
	// in. For example if we have 10 peers and we can't send a message to the
	// same peer every 500ms, then we can send a request every 50ms. In practice
	// we use a safety margin of 2, ergo 100ms
	peers := tmmath.MinInt(len(r.availablePeers), 50)
	baseTime := minReceiveRequestInterval
	if peers > 0 {
		baseTime = minReceiveRequestInterval * 2 / time.Duration(peers)
	}

	if r.totalPeers > 0 || r.discoveryRatio == 0 {
		// find the ratio of new peers. NOTE: We add 1 to both sides to avoid
		// divide by zero problems
		ratio := float32(r.totalPeers+1) / float32(r.newPeers+1)
		// square the ratio in order to get non linear time intervals
		// NOTE: The longest possible interval for a network with 100 or more peers
		// where a node is connected to 50 of them is 2 minutes.
		r.discoveryRatio = ratio * ratio
		r.newPeers = 0
		r.totalPeers = 0
	}
	// NOTE: As ratio is always >= 1, discovery ratio is >= 1. Therefore we don't need to worry
	// about the next request time being less than the minimum time
	r.nextRequestTime = time.Now().Add(baseTime * time.Duration(r.discoveryRatio))
}

func (r *ReactorV2) markPeerRequest(peer types.NodeID) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if lastRequestTime, ok := r.lastReceivedRequests[peer]; ok {
		if time.Now().Before(lastRequestTime.Add(minReceiveRequestInterval)) {
			return fmt.Errorf("peer sent a request too close after a prior one. Minimum interval: %v",
				minReceiveRequestInterval)
		}
	}
	r.lastReceivedRequests[peer] = time.Now()
	return nil
}

func (r *ReactorV2) markPeerResponse(peer types.NodeID) error {
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

// all addresses must use a MCONN protocol for the peer to be considered part of the
// legacy p2p pex system
func (r *ReactorV2) isLegacyPeer(peer types.NodeID) bool {
	for _, addr := range r.peerManager.Addresses(peer) {
		if addr.Protocol != p2p.MConnProtocol {
			return false
		}
	}
	return true
}
