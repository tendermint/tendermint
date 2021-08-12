package statesync

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/light/provider"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
	proto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

var (
	errNoConnectedPeers    = errors.New("no available peers to dispatch request to")
	errUnsolicitedResponse = errors.New("unsolicited light block response")
	errNoResponse          = errors.New("peer failed to respond within timeout")
	errPeerAlreadyBusy     = errors.New("peer is already processing a request")
	errDisconnected        = errors.New("dispatcher has been disconnected")
)

// dispatcher keeps a list of peers and allows concurrent requests for light
// blocks. NOTE: It is not the responsibility of the dispatcher to verify the
// light blocks.
type dispatcher struct {
	// a pool of peers to send light block request too
	availablePeers *peerlist
	requestCh      chan<- p2p.Envelope
	// timeout for light block delivery (immutable)
	timeout time.Duration

	mtx sync.Mutex
	// the set of providers that the dispatcher is providing for (is distinct
	// from available peers)
	providers map[types.NodeID]struct{}
	// all pending calls that have been dispatched and are awaiting an answer
	calls map[types.NodeID]chan *types.LightBlock
	// signals whether the underlying reactor is still running
	running bool
}

func newDispatcher(requestCh chan<- p2p.Envelope, timeout time.Duration) *dispatcher {
	return &dispatcher{
		availablePeers: newPeerList(),
		timeout:        timeout,
		requestCh:      requestCh,
		providers:      make(map[types.NodeID]struct{}),
		calls:          make(map[types.NodeID]chan *types.LightBlock),
		running:        true,
	}
}

// LightBlock uses the request channel to fetch a light block from the next peer
// in a list, tracks the call and waits for the reactor to pass along the response
func (d *dispatcher) LightBlock(ctx context.Context, height int64) (*types.LightBlock, types.NodeID, error) {
	d.mtx.Lock()
	// check that the dispatcher is connected to the reactor
	if !d.running {
		return nil, "", errDisconnected
	}
	// check to see that the dispatcher is connected to at least one peer
	if d.availablePeers.Len() == 0 && len(d.calls) == 0 {
		d.mtx.Unlock()
		return nil, "", errNoConnectedPeers
	}
	d.mtx.Unlock()

	// fetch the next peer id in the list and request a light block from that
	// peer
	peer := d.availablePeers.Pop(ctx)
	lb, err := d.lightBlock(ctx, height, peer)

	// append the peer back to the list
	d.availablePeers.Append(peer)
	return lb, peer, err
}

// Providers turns the dispatcher into a set of providers (per peer) which can
// be used by a light client
func (d *dispatcher) Providers(chainID string) []provider.Provider {
	providers := make([]provider.Provider, d.availablePeers.Len())
	for i := 0; i < cap(providers); i++ {
		peer := d.availablePeers.Pop(context.Background())
		providers[i] = d.CreateProvider(peer, chainID)
	}
	return providers
}

// Creates an individual provider from a peer id that the dispatcher is
// connected with.
func (d *dispatcher) CreateProvider(peer types.NodeID, chainID string) provider.Provider {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	d.availablePeers.Remove(peer)
	d.providers[peer] = struct{}{}
	return &blockProvider{
		peer:       peer,
		dispatcher: d,
		chainID:    chainID,
		timeout:    d.timeout,
	}
}

func (d *dispatcher) stop() {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	d.running = false
	for peer, call := range d.calls {
		close(call)
		delete(d.calls, peer)
	}
}

func (d *dispatcher) lightBlock(ctx context.Context, height int64, peer types.NodeID) (*types.LightBlock, error) {
	// dispatch the request to the peer
	callCh, err := d.dispatch(peer, height)
	if err != nil {
		return nil, err
	}

	defer func() {
		d.mtx.Lock()
		defer d.mtx.Unlock()
		if call, ok := d.calls[peer]; ok {
			close(call)
			delete(d.calls, peer)
		}
	}()

	// wait for a response, cancel or timeout
	select {
	case resp := <-callCh:
		return resp, nil

	case <-ctx.Done():
		return nil, ctx.Err()

	case <-time.After(d.timeout):
		return nil, errNoResponse
	}
}

// respond allows the underlying process which receives requests on the
// requestCh to respond with the respective light block
func (d *dispatcher) respond(lb *proto.LightBlock, peer types.NodeID) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	// check that the response came from a request
	answerCh, ok := d.calls[peer]
	if !ok {
		// this can also happen if the response came in after the timeout
		return errUnsolicitedResponse
	}

	if lb == nil {
		answerCh <- nil
		return nil
	}

	block, err := types.LightBlockFromProto(lb)
	if err != nil {
		answerCh <- nil
		return err
	}

	answerCh <- block
	return nil
}

// addPeer adds a peer to the dispatcher
func (d *dispatcher) addPeer(peer types.NodeID) {
	d.availablePeers.Append(peer)
}

// removePeer removes a peer from the dispatcher
func (d *dispatcher) removePeer(peer types.NodeID) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	if call, ok := d.calls[peer]; ok {
		call <- nil
		close(call)
		delete(d.calls, peer)
	} else {
		d.availablePeers.Remove(peer)
	}
	delete(d.providers, peer)
}

// peerCount returns the amount of peers that the dispatcher is connected with
func (d *dispatcher) peerCount() int {
	return d.availablePeers.Len()
}

func (d *dispatcher) isConnected(peer types.NodeID) bool {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	_, ok := d.providers[peer]
	return ok
}

// dispatch takes a peer and allocates it a channel so long as it's not already
// busy and the receiving channel is still running. It then dispatches the message
func (d *dispatcher) dispatch(peer types.NodeID, height int64) (chan *types.LightBlock, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	ch := make(chan *types.LightBlock, 1)

	// check if the dispatcher is running or not
	if !d.running {
		close(ch)
		return ch, errDisconnected
	}

	// this should happen only if we add the same peer twice (somehow)
	if _, ok := d.calls[peer]; ok {
		close(ch)
		return ch, errPeerAlreadyBusy
	}
	d.calls[peer] = ch

	// send request
	d.requestCh <- p2p.Envelope{
		To: peer,
		Message: &ssproto.LightBlockRequest{
			Height: uint64(height),
		},
	}
	return ch, nil
}

//----------------------------------------------------------------

// blockProvider is a p2p based light provider which uses a dispatcher connected
// to the state sync reactor to serve light blocks to the light client
//
// TODO: This should probably be moved over to the light package but as we're
// not yet officially supporting p2p light clients we'll leave this here for now.
type blockProvider struct {
	peer       types.NodeID
	chainID    string
	timeout    time.Duration
	dispatcher *dispatcher
}

func (p *blockProvider) LightBlock(ctx context.Context, height int64) (*types.LightBlock, error) {
	// check if the underlying reactor is still connected with the peer
	if !p.dispatcher.isConnected(p.peer) {
		return nil, provider.ErrConnectionClosed
	}

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	lb, err := p.dispatcher.lightBlock(ctx, height, p.peer)
	switch err {
	case nil:
		if lb == nil {
			return nil, provider.ErrLightBlockNotFound
		}
	case context.DeadlineExceeded, context.Canceled:
		return nil, err
	case errNoResponse:
		return nil, provider.ErrNoResponse
	default:
		return nil, provider.ErrUnreliableProvider{Reason: err.Error()}
	}

	if err := lb.ValidateBasic(p.chainID); err != nil {
		return nil, provider.ErrBadLightBlock{Reason: err}
	}

	return lb, nil
}

// ReportEvidence should allow for the light client to report any light client
// attacks. This is a no op as there currently isn't a way to wire this up to
// the evidence reactor (we should endeavor to do this in the future but for now
// it's not critical for backwards verification)
func (p *blockProvider) ReportEvidence(ctx context.Context, ev types.Evidence) error {
	return nil
}

// String implements stringer interface
func (p *blockProvider) String() string { return string(p.peer) }

//----------------------------------------------------------------

// peerList is a rolling list of peers. This is used to distribute the load of
// retrieving blocks over all the peers the reactor is connected to
type peerlist struct {
	mtx     sync.Mutex
	peers   []types.NodeID
	waiting []chan types.NodeID
}

func newPeerList() *peerlist {
	return &peerlist{
		peers:   make([]types.NodeID, 0),
		waiting: make([]chan types.NodeID, 0),
	}
}

func (l *peerlist) Len() int {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	return len(l.peers)
}

func (l *peerlist) Pop(ctx context.Context) types.NodeID {
	l.mtx.Lock()
	if len(l.peers) == 0 {
		// if we don't have any peers in the list we block until a peer is
		// appended
		wait := make(chan types.NodeID, 1)
		l.waiting = append(l.waiting, wait)
		// unlock whilst waiting so that the list can be appended to
		l.mtx.Unlock()
		select {
		case peer := <-wait:
			return peer

		case <-ctx.Done():
			return ""
		}
	}

	peer := l.peers[0]
	l.peers = l.peers[1:]
	l.mtx.Unlock()
	return peer
}

func (l *peerlist) Append(peer types.NodeID) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	if len(l.waiting) > 0 {
		wait := l.waiting[0]
		l.waiting = l.waiting[1:]
		wait <- peer
		close(wait)
	} else {
		l.peers = append(l.peers, peer)
	}
}

func (l *peerlist) Remove(peer types.NodeID) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	for i, p := range l.peers {
		if p == peer {
			l.peers = append(l.peers[:i], l.peers[i+1:]...)
			return
		}
	}
}

func (l *peerlist) Peers() []types.NodeID {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	return l.peers
}
