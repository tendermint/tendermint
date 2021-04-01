package pex

import (
	"context"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/libs/clist"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/p2p"
	protop2p "github.com/tendermint/tendermint/proto/tendermint/p2p"
)

var (
	_ service.Service = (*ReactorV2)(nil)
	_ p2p.Wrapper     = (*protop2p.PexMessage)(nil)
)

// TODO: Consolidate with params file.
const (
	// the minimum time one peer can send another request to the same peer
	minReceiveRequestInterval = 300 * time.Millisecond

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

// ReactorV2 is a PEX reactor for the new P2P stack. The legacy reactor
// is Reactor.
//
// FIXME: Rename this when Reactor is removed, and consider moving to p2p/.
//
// The peer explorer or PEX reactor supports the peer manager by sending
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
	availablePeers *clist.CList
	// requestsSent keeps track of which peers the PEX reactor has sent requests
	// to. This prevents the sending of spurious responses.
	// NOTE: If a node never responds, they will remain in this map until a
	// peer down status update is sent
	requestsSent map[p2p.NodeID]struct{}
	// lastReceivedRequests keeps track of when peers send a request to prevent
	// peers from sending requests too often (as defined by
	// minReceiveRequestInterval).
	lastReceivedRequests map[p2p.NodeID]time.Time

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
		availablePeers:       clist.New(),
		requestsSent:         make(map[p2p.NodeID]struct{}),
		lastReceivedRequests: make(map[p2p.NodeID]time.Time),
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

		case <-r.closeCh:
			r.Logger.Debug("stopped listening on PEX channel; closing...")
			return
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
		// check if the peer hasn't sent a prior request too close to this one
		// in time
		if lastRequestTime, ok := r.lastReceivedRequests[envelope.From]; ok {
			if time.Now().Before(lastRequestTime.Add(minReceiveRequestInterval)) {
				return fmt.Errorf("peer sent a request too close after a prior one. Minimum interval: %v",
					minReceiveRequestInterval)
			}
		}
		r.lastReceivedRequests[envelope.From] = time.Now()

		pexAddresses := r.resolve(r.peerManager.Advertise(envelope.From, maxAddresses), maxAddresses)
		r.pexCh.Out <- p2p.Envelope{
			To:      envelope.From,
			Message: &protop2p.PexResponse{Addresses: pexAddresses},
		}

	case *protop2p.PexResponse:
		// check if a request to this peer was sent
		if _, ok := r.requestsSent[envelope.From]; !ok {
			return fmt.Errorf("peer sent a PEX response when none was requested (%v)", envelope.From)
		}
		delete(r.requestsSent, envelope.From)
		// attach to the back of the list so that the peer can be used again for
		// future requests
		r.availablePeers.PushBack(envelope.From)

		// check the size of the response
		if len(msg.Addresses) > int(maxAddresses) {
			return fmt.Errorf("peer sent too many addresses (max: %d, got: %d)",
				maxAddresses,
				len(msg.Addresses),
			)
		}

		for _, pexAddress := range msg.Addresses {
			peerAddress, err := p2p.ParseNodeAddress(
				fmt.Sprintf("%s@%s:%d", pexAddress.ID, pexAddress.IP, pexAddress.Port))
			if err != nil {
				continue
			}
			added, err := r.peerManager.Add(peerAddress)
			if err != nil {
				logger.Debug("failed to register PEX address", "address", peerAddress, "err", err)
			}
			if added {
				r.newPeers++
				logger.Debug("added PEX address", "address", peerAddress)
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
func (r *ReactorV2) resolve(addresses []p2p.NodeAddress, limit uint16) []protop2p.PexAddress {
	pexAddresses := make([]protop2p.PexAddress, 0, len(addresses))
	for _, address := range addresses {
		ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
		endpoints, err := address.Resolve(ctx)
		cancel()
		if err != nil {
			r.Logger.Debug("failed to resolve address", "address", address, "err", err)
			continue
		}
		for _, endpoint := range endpoints {
			if len(pexAddresses) >= int(limit) {
				return pexAddresses

			} else if endpoint.IP != nil {
				// PEX currently only supports IP-networked transports (as
				// opposed to e.g. p2p.MemoryTransport).
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
	switch peerUpdate.Status {
	case p2p.PeerStatusUp:
		r.availablePeers.PushBack(peerUpdate.NodeID)
	case p2p.PeerStatusDown:
		r.removePeer(peerUpdate.NodeID)
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
	peer := r.availablePeers.Front()
	if peer == nil {
		// no peers are available
		r.Logger.Debug("no available peers to send request to, waiting...")
		r.nextRequestTime = time.Now().Add(noAvailablePeersWaitPeriod)
		return
	}
	peerID := peer.Value.(p2p.NodeID)

	r.pexCh.Out <- p2p.Envelope{
		To:      peerID,
		Message: &protop2p.PexRequest{},
	}

	r.availablePeers.Remove(peer)
	r.requestsSent[peerID] = struct{}{}

	r.calculateNextRequestTime()
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
// MaxInterval = 100 * 100 * baseTime ~= 16mins for 10 peers
func (r *ReactorV2) calculateNextRequestTime() {
	// check if the peer store is full. If so then there is no need
	// to send peer requests too often
	if r.peerManager.Capacity() > 0.95 {
		r.nextRequestTime = time.Now().Add(fullCapacityInterval)
		return
	}

	// baseTime represents the shortest interval that we can send peer requests
	// in. For example if we have 10 peers and we can't send a message to the
	// same peer every 500ms, then we can send a request every 50ms. In practice
	// we use a safety margin of 2, ergo 100ms
	baseTime := minReceiveRequestInterval / time.Duration(r.availablePeers.Len()*2)

	if r.totalPeers > 0 || r.discoveryRatio == 0 {
		// find the ratio of new peers. NOTE: We add 1 to both sides to avoid
		// divide by zero problems
		ratio := float32(r.totalPeers+1) / float32(r.newPeers+1)
		// square the ratio in order to get non linear time intervals
		r.discoveryRatio = ratio * ratio
		r.newPeers = 0
		r.totalPeers = 0
	}
	interval := baseTime * time.Duration(r.discoveryRatio)
	r.nextRequestTime = time.Now().Add(interval)
}

func (r *ReactorV2) removePeer(id p2p.NodeID) {
	for e := r.availablePeers.Front(); e != nil; e = e.Next() {
		if e.Value == id {
			r.availablePeers.Remove(e)
			e.DetachPrev()
			break
		}
	}
	delete(r.requestsSent, id)
	delete(r.lastReceivedRequests, id)
}
