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
	maxAddresses uint16 = 100

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
	pexCh       *p2p.Channel
	peerUpdates *p2p.PeerUpdates

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
func NewReactor(
	ctx context.Context,
	logger log.Logger,
	peerManager *p2p.PeerManager,
	channelCreator p2p.ChannelCreator,
	peerUpdates *p2p.PeerUpdates,
) (*Reactor, error) {

	channel, err := channelCreator(ctx, ChannelDescriptor())
	if err != nil {
		return nil, err
	}

	r := &Reactor{
		logger:               logger,
		peerManager:          peerManager,
		pexCh:                channel,
		peerUpdates:          peerUpdates,
		availablePeers:       make(map[types.NodeID]struct{}),
		requestsSent:         make(map[types.NodeID]struct{}),
		lastReceivedRequests: make(map[types.NodeID]time.Time),
	}

	r.BaseService = *service.NewBaseService(logger, "PEX", r)
	return r, nil
}

// OnStart starts separate go routines for each p2p Channel and listens for
// envelopes on each. In addition, it also listens for peer updates and handles
// messages on that p2p channel accordingly. The caller must be sure to execute
// OnStop to ensure the outbound p2p Channels are closed.
func (r *Reactor) OnStart(ctx context.Context) error {
	go r.processPexCh(ctx)
	go r.processPeerUpdates(ctx)
	return nil
}

// OnStop stops the reactor by signaling to all spawned goroutines to exit and
// blocking until they all exit.
func (r *Reactor) OnStop() {}

// processPexCh implements a blocking event loop where we listen for p2p
// Envelope messages from the pexCh.
func (r *Reactor) processPexCh(ctx context.Context) {
	timer := time.NewTimer(0)
	defer timer.Stop()

	r.mtx.Lock()
	var (
		duration = r.calculateNextRequestTime()
		err      error
	)
	r.mtx.Unlock()

	incoming := make(chan *p2p.Envelope)
	go func() {
		defer close(incoming)
		iter := r.pexCh.Receive(ctx)
		for iter.Next(ctx) {
			select {
			case <-ctx.Done():
				return
			case incoming <- iter.Envelope():
			}
		}
	}()

	for {
		timer.Reset(duration)

		select {
		case <-ctx.Done():
			return

		// outbound requests for new peers
		case <-timer.C:
			duration, err = r.sendRequestForPeers(ctx)
			if err != nil {
				return
			}
		// inbound requests for new peers or responses to requests sent by this
		// reactor
		case envelope, ok := <-incoming:
			if !ok {
				return
			}
			duration, err = r.handleMessage(ctx, r.pexCh.ID, envelope)
			if err != nil {
				r.logger.Error("failed to process message", "ch_id", r.pexCh.ID, "envelope", envelope, "err", err)
				if serr := r.pexCh.SendError(ctx, p2p.PeerError{
					NodeID: envelope.From,
					Err:    err,
				}); serr != nil {
					return
				}
			}

		}
	}
}

// processPeerUpdates initiates a blocking process where we listen for and handle
// PeerUpdate messages. When the reactor is stopped, we will catch the signal and
// close the p2p PeerUpdatesCh gracefully.
func (r *Reactor) processPeerUpdates(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case peerUpdate := <-r.peerUpdates.Updates():
			r.processPeerUpdate(peerUpdate)
		}
	}
}

// handlePexMessage handles envelopes sent from peers on the PexChannel.
func (r *Reactor) handlePexMessage(ctx context.Context, envelope *p2p.Envelope) (time.Duration, error) {
	logger := r.logger.With("peer", envelope.From)

	switch msg := envelope.Message.(type) {
	case *protop2p.PexRequest:
		// check if the peer hasn't sent a prior request too close to this one
		// in time
		if err := r.markPeerRequest(envelope.From); err != nil {
			return time.Minute, err
		}

		// request peers from the peer manager and parse the NodeAddresses into
		// URL strings
		nodeAddresses := r.peerManager.Advertise(envelope.From, maxAddresses)
		pexAddresses := make([]protop2p.PexAddress, len(nodeAddresses))
		for idx, addr := range nodeAddresses {
			pexAddresses[idx] = protop2p.PexAddress{
				URL: addr.String(),
			}
		}
		if err := r.pexCh.Send(ctx, p2p.Envelope{
			To:      envelope.From,
			Message: &protop2p.PexResponse{Addresses: pexAddresses},
		}); err != nil {
			return 0, err
		}

		return time.Second, nil
	case *protop2p.PexResponse:
		// check if the response matches a request that was made to that peer
		if err := r.markPeerResponse(envelope.From); err != nil {
			return time.Minute, err
		}

		// check the size of the response
		if len(msg.Addresses) > int(maxAddresses) {
			return 10 * time.Minute, fmt.Errorf("peer sent too many addresses (max: %d, got: %d)",
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
				logger.Error("failed to add PEX address", "address", peerAddress, "err", err)
			}
			if added {
				r.newPeers++
				logger.Debug("added PEX address", "address", peerAddress)
			}
			r.totalPeers++
		}

		return 10 * time.Minute, nil
	default:
		return time.Second, fmt.Errorf("received unknown message: %T", msg)
	}
}

// handleMessage handles an Envelope sent from a peer on a specific p2p Channel.
// It will handle errors and any possible panics gracefully. A caller can handle
// any error returned by sending a PeerError on the respective channel.
func (r *Reactor) handleMessage(ctx context.Context, chID p2p.ChannelID, envelope *p2p.Envelope) (duration time.Duration, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("panic in processing message: %v", e)
			r.logger.Error(
				"recovering from processing message panic",
				"err", err,
				"stack", string(debug.Stack()),
			)
		}
	}()

	r.logger.Debug("received PEX message", "peer", envelope.From)

	switch chID {
	case p2p.ChannelID(PexChannel):
		duration, err = r.handlePexMessage(ctx, envelope)
	default:
		err = fmt.Errorf("unknown channel ID (%d) for envelope (%v)", chID, envelope)
	}

	return
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

// sendRequestForPeers pops the first peerID off the list and sends the
// peer a request for more peer addresses. The function then moves the
// peer into the requestsSent bucket and calculates when the next request
// time should be
func (r *Reactor) sendRequestForPeers(ctx context.Context) (time.Duration, error) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if len(r.availablePeers) == 0 {
		// no peers are available
		r.logger.Debug("no available peers to send request to, waiting...")
		return noAvailablePeersWaitPeriod, nil
	}
	var peerID types.NodeID

	// use range to get a random peer.
	for peerID = range r.availablePeers {
		break
	}

	// send out the pex request
	if err := r.pexCh.Send(ctx, p2p.Envelope{
		To:      peerID,
		Message: &protop2p.PexRequest{},
	}); err != nil {
		return 0, err
	}

	// remove the peer from the abvailable peers list and mark it in the requestsSent map
	delete(r.availablePeers, peerID)
	r.requestsSent[peerID] = struct{}{}

	dur := r.calculateNextRequestTime()
	r.logger.Debug("peer request sent", "next_request_time", dur)
	return dur, nil
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
//
// CONTRACT: The caller must hold r.mtx exclusively when calling this method.
func (r *Reactor) calculateNextRequestTime() time.Duration {
	// check if the peer store is full. If so then there is no need
	// to send peer requests too often
	if ratio := r.peerManager.PeerRatio(); ratio >= 0.95 {
		r.logger.Debug("peer manager near full ratio, sleeping...",
			"sleep_period", fullCapacityInterval, "ratio", ratio)
		return fullCapacityInterval
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
	return baseTime * time.Duration(r.discoveryRatio)
}

func (r *Reactor) markPeerRequest(peer types.NodeID) error {
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
