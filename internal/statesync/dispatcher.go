package statesync

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/light/provider"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

var (
	errNoConnectedPeers    = errors.New("no available peers to dispatch request to")
	errUnsolicitedResponse = errors.New("unsolicited light block response")
	errPeerAlreadyBusy     = errors.New("peer is already processing a request")
	errDisconnected        = errors.New("dispatcher disconnected")
)

// A Dispatcher multiplexes concurrent requests by multiple peers for light blocks.
// Only one request per peer can be sent at a time. Subsequent concurrent requests will
// report an error from the LightBlock method.
// NOTE: It is not the responsibility of the dispatcher to verify the light blocks.
type Dispatcher struct {
	// the channel with which to send light block requests on
	requestCh p2p.Channel

	mtx sync.Mutex
	// all pending calls that have been dispatched and are awaiting an answer
	calls map[types.NodeID]chan *types.LightBlock
}

func NewDispatcher(requestChannel p2p.Channel) *Dispatcher {
	return &Dispatcher{
		requestCh: requestChannel,
		calls:     make(map[types.NodeID]chan *types.LightBlock),
	}
}

// LightBlock uses the request channel to fetch a light block from a given peer
// tracking, the call and waiting for the reactor to pass back the response. A nil
// LightBlock response is used to signal that the peer doesn't have the requested LightBlock.
func (d *Dispatcher) LightBlock(ctx context.Context, height int64, peer types.NodeID) (*types.LightBlock, error) {
	// dispatch the request to the peer
	callCh, err := d.dispatch(ctx, peer, height)
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
		return resp, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// dispatch takes a peer and allocates it a channel so long as it's not already
// busy and the receiving channel is still running. It then dispatches the message
func (d *Dispatcher) dispatch(ctx context.Context, peer types.NodeID, height int64) (chan *types.LightBlock, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	select {
	case <-ctx.Done():
		return nil, errDisconnected
	default:
	}

	ch := make(chan *types.LightBlock, 1)

	// check if a request for the same peer has already been made
	if _, ok := d.calls[peer]; ok {
		close(ch)
		return ch, errPeerAlreadyBusy
	}
	d.calls[peer] = ch

	// send request
	if err := d.requestCh.Send(ctx, p2p.Envelope{
		To: peer,
		Message: &ssproto.LightBlockRequest{
			Height: uint64(height),
		},
	}); err != nil {
		close(ch)
		return ch, err
	}

	return ch, nil
}

// Respond allows the underlying process which receives requests on the
// requestCh to respond with the respective light block. A nil response is used to
// represent that the receiver of the request does not have a light block at that height.
func (d *Dispatcher) Respond(ctx context.Context, lb *tmproto.LightBlock, peer types.NodeID) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	// check that the response came from a request
	answerCh, ok := d.calls[peer]
	if !ok {
		// this can also happen if the response came in after the timeout
		return errUnsolicitedResponse
	}

	// If lb is nil we take that to mean that the peer didn't have the requested light
	// block and thus pass on the nil to the caller.
	if lb == nil {
		select {
		case answerCh <- nil:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	block, err := types.LightBlockFromProto(lb)
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case answerCh <- block:
		return nil
	}
}

// Close shuts down the dispatcher and cancels any pending calls awaiting responses.
// Peers awaiting responses that have not arrived are delivered a nil block.
func (d *Dispatcher) Close() {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	for peer := range d.calls {
		delete(d.calls, peer)
		// don't close the channel here as it's closed in
		// other handlers, and would otherwise get garbage
		// collected.
	}
}

//----------------------------------------------------------------

// BlockProvider is a p2p based light provider which uses a dispatcher connected
// to the state sync reactor to serve light blocks to the light client
//
// TODO: This should probably be moved over to the light package but as we're
// not yet officially supporting p2p light clients we'll leave this here for now.
//
// NOTE: BlockProvider will return an error with concurrent calls. However, we don't
// need a mutex because a light client (and the backfill process) will never call a
// method more than once at the same time
type BlockProvider struct {
	peer       types.NodeID
	chainID    string
	dispatcher *Dispatcher
}

// Creates a block provider which implements the light client Provider interface.
func NewBlockProvider(peer types.NodeID, chainID string, dispatcher *Dispatcher) *BlockProvider {
	return &BlockProvider{
		peer:       peer,
		chainID:    chainID,
		dispatcher: dispatcher,
	}
}

// LightBlock fetches a light block from the peer at a specified height returning either a
// light block or an appropriate error.
func (p *BlockProvider) LightBlock(ctx context.Context, height int64) (*types.LightBlock, error) {
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
	default:
		return nil, provider.ErrUnreliableProvider{Reason: err}
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
func (p *BlockProvider) ReportEvidence(ctx context.Context, ev types.Evidence) error {
	return nil
}

// String implements stringer interface
func (p *BlockProvider) String() string { return string(p.peer) }

// Returns the ID address of the provider (NodeID of peer)
func (p *BlockProvider) ID() string { return string(p.peer) }

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

func (l *peerList) Contains(id types.NodeID) bool {
	l.mtx.Lock()
	defer l.mtx.Unlock()

	for _, p := range l.peers {
		if id == p {
			return true
		}
	}

	return false
}
