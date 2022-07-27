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
	minReceiveRequestInterval = 200 * time.Millisecond

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
	nextRequestInterval time.Duration

	// the total number of unique peers added
	totalPeers int
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

	r.nextRequestInterval = minReceiveRequestInterval
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

		var numAdded int
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
				numAdded++
				logger.Debug("added PEX address", "address", peerAddress)
			}
		}
		r.calculateNextRequestTime(numAdded)

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

		var numAdded int
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
				numAdded++
				logger.Debug("added V2 PEX address", "address", peerAddress)
			}
		}
		r.calculateNextRequestTime(numAdded)

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
	return time.After(r.nextRequestInterval)
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
		r.Logger.Debug("no available peers to send a PEX request to (retrying)")
		return
	}

	// Select an arbitrary peer from the available set.
	var peerID types.NodeID
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
func (r *ReactorV2) calculateNextRequestTime(added int) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	r.totalPeers += added

	// If the peer store is nearly full, wait the maximum interval.
	if ratio := r.peerManager.PeerRatio(); ratio >= 0.95 {
<<<<<<< HEAD
		r.Logger.Debug("Peer manager is nearly full",
			"sleep_period", fullCapacityInterval, "ratio", ratio)
		r.nextRequestInterval = fullCapacityInterval
		return
=======
		r.logger.Debug("Peer manager is nearly full",
			"sleep_period", fullCapacityInterval,
			"ratio", ratio)
		return fullCapacityInterval
>>>>>>> 48147e1fb (logging: implement lazy sprinting (#8898))
	}

	// If there are no available peers to query, poll less aggressively.
	if len(r.availablePeers) == 0 {
		r.Logger.Debug("No available peers to send a PEX request",
			"sleep_period", noAvailablePeersWaitPeriod)
		r.nextRequestInterval = noAvailablePeersWaitPeriod
		return
	}

	// Reaching here, there are available peers to query and the peer store
	// still has space. Estimate our knowledge of the network from the latest
	// update and choose a new interval.
	base := float64(minReceiveRequestInterval) / float64(len(r.availablePeers))
	multiplier := float64(r.totalPeers+1) / float64(added+1) // +1 to avert zero division
	r.nextRequestInterval = time.Duration(base*multiplier*multiplier) + minReceiveRequestInterval
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
