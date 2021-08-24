package statesync

import (
	"context"
	"errors"
	"fmt"
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
	errDisconnected        = errors.New("provider has been disconnected")
)

// dispatcher multiplexes concurrent requests by multiple peers for light blocks.
// Only one request per peer can be sent at a time
// NOTE: It is not the responsibility of the dispatcher to verify the light blocks.
type Dispatcher struct {
	// the channel with which to send light block requests on
	requestCh chan<- p2p.Envelope
	// timeout for light block delivery (immutable)
	timeout time.Duration

	mtx sync.Mutex
	// all pending calls that have been dispatched and are awaiting an answer
	calls map[types.NodeID]chan *types.LightBlock
	// signals whether the underlying reactor is still running
	running bool
}

func NewDispatcher(requestCh chan<- p2p.Envelope, timeout time.Duration) *Dispatcher {
	return &Dispatcher{
		timeout:   timeout,
		requestCh: requestCh,
		calls:     make(map[types.NodeID]chan *types.LightBlock),
		running:   true,
	}
}

// LightBlock uses the request channel to fetch a light block from the next peer
// in a list, tracks the call and waits for the reactor to pass along the response
func (d *Dispatcher) LightBlock(ctx context.Context, height int64, peer types.NodeID) (*types.LightBlock, error) {
	// dispatch the request to the peer
	callCh, err := d.dispatch(peer, height)
	if err != nil {
		return nil, err
	}

	// clean up the call after a response is returned
	defer func() {
		d.mtx.Lock()
		defer d.mtx.Unlock()
		if call, ok := d.calls[peer]; ok {
			delete(d.calls, peer)
			close(call)
		}
	}()

	// wait for a response, cancel or timeout
	select {
	case resp := <-callCh:
		fmt.Printf("received response, height %d peer %v\n", height, peer)
		return resp, nil

	case <-ctx.Done():
		return nil, ctx.Err()

	case <-time.After(d.timeout):
		return nil, errNoResponse
	}
}

// dispatch takes a peer and allocates it a channel so long as it's not already
// busy and the receiving channel is still running. It then dispatches the message
func (d *Dispatcher) dispatch(peer types.NodeID, height int64) (chan *types.LightBlock, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	ch := make(chan *types.LightBlock, 1)

	// check if the dispatcher is running or not
	if !d.running {
		close(ch)
		return ch, errDisconnected
	}

	// check if a request for the same peer has already been made
	if _, ok := d.calls[peer]; ok {
		close(ch)
		return ch, errPeerAlreadyBusy
	}
	d.calls[peer] = ch

	// send request
	fmt.Printf("sending request dispatch, height %d peer %v\n", height, peer)
	d.requestCh <- p2p.Envelope{
		To: peer,
		Message: &ssproto.LightBlockRequest{
			Height: uint64(height),
		},
	}
	fmt.Printf("sent request dispatch, height %d peer %v\n", height, peer)
	return ch, nil
}

// respond allows the underlying process which receives requests on the
// requestCh to respond with the respective light block
func (d *Dispatcher) Respond(lb *proto.LightBlock, peer types.NodeID) error {
	fmt.Printf("trying to respond with light block for height %d from %v\n", lb.SignedHeader.Header.Height, peer)
	d.mtx.Lock()
	defer d.mtx.Unlock()
	fmt.Printf("responding with light block for height %d from %v\n", lb.SignedHeader.Header.Height, peer)

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
		return err
	}

	answerCh <- block
	return nil
}

func (d *Dispatcher) Stop() {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	d.running = false
	for peer, call := range d.calls {
		delete(d.calls, peer)
		close(call)
	}
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
	dispatcher *Dispatcher
}

// Creates a block provider which implements the light client Provider interface.
func NewBlockProvider(peer types.NodeID, chainID string, dispatcher *Dispatcher) *blockProvider {
	return &blockProvider{
		peer:       peer,
		chainID:    chainID,
		dispatcher: dispatcher,
	}
}

// LightBlock fetches a light block from the peer at a specified height returning either a light block
// or an appropriate error. Concurrently unsafe
func (p *blockProvider) LightBlock(ctx context.Context, height int64) (*types.LightBlock, error) {
	fmt.Println("fetching block for block provider")
	lb, err := p.dispatcher.LightBlock(ctx, height, p.peer)
	switch err {
	case nil:
		if lb == nil {
			return nil, provider.ErrLightBlockNotFound
		}
	case context.DeadlineExceeded, context.Canceled:
		return nil, err
	case errPeerAlreadyBusy:
		return nil, provider.ErrLightBlockNotFound
	case errNoResponse:
		return nil, provider.ErrNoResponse
	default:
		return nil, provider.ErrUnreliableProvider{Reason: err.Error()}
	}

	// check that the height requested is the same one returned
	if lb.Height != height {
		return nil, provider.ErrBadLightBlock{
			Reason: fmt.Errorf("expected height %d, got height %d", height, lb.Height),
		}
	}

	// perform basic validation
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
type peerList struct {
	mtx     sync.Mutex
	peers   []types.NodeID
	waiting []chan types.NodeID
}

func newPeerList() *peerList {
	return &peerList{
		peers:   make([]types.NodeID, 0),
		waiting: make([]chan types.NodeID, 0),
	}
}

func (l *peerList) Len() int {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	return len(l.peers)
}

func (l *peerList) Pop(ctx context.Context) types.NodeID {
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

func (l *peerList) Append(peer types.NodeID) {
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

func (l *peerList) Remove(peer types.NodeID) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	for i, p := range l.peers {
		if p == peer {
			l.peers = append(l.peers[:i], l.peers[i+1:]...)
			return
		}
	}
}

func (l *peerList) All() []types.NodeID {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	return l.peers
}
