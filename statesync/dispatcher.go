package statesync

import (
	"context"
	"errors"
	"time"

	"github.com/tendermint/tendermint/libs/clist"
	"github.com/tendermint/tendermint/light/provider"
	"github.com/tendermint/tendermint/p2p"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
	proto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

var noConnectedPeers = errors.New("no available peers to dispatch request to")

// dispatcher keeps a list of peers and allows concurrent requests for light
// blocks. NOTE: It is not the responsibility of the dispatcher to verify the
// light blocks.
type dispatcher struct {
	connectedPeers *clist.CList
	requestCh      chan<- p2p.Envelope
	calls          map[p2p.NodeID]chan *types.LightBlock
}

func newDispatcher(requestCh chan<- p2p.Envelope) *dispatcher {
	return &dispatcher{
		connectedPeers: clist.New(),
		requestCh:      requestCh,
		calls:          make(map[p2p.NodeID]chan *types.LightBlock),
	}
}

func (d *dispatcher) LightBlock(ctx context.Context, height int64) (*types.LightBlock, p2p.NodeID, error) {
	if d.connectedPeers.Len() == 0 {
		return nil, "", noConnectedPeers
	}
	// fetch the next peer id in the list and request a light block from that
	// peer
	peer := d.nextPeer()
	lb, err := d.lightBlock(ctx, height, peer)
	return lb, peer, err
}

func (d *dispatcher) Providers(chainID string) []provider.Provider {
	providers := make([]provider.Provider, d.connectedPeers.Len())
	index := 0
	for e := d.connectedPeers.Front(); e != nil; e = e.Next() {
		providers[index] = &blockProvider{
			peer:       e.Value.(p2p.NodeID),
			dispatcher: d,
		}
	}
	return providers
}

func (d *dispatcher) lightBlock(ctx context.Context, height int64, peer p2p.NodeID) (*types.LightBlock, error) {
	d.requestCh <- p2p.Envelope{
		To: peer,
		Message: &ssproto.LightBlockRequest{
			Height: uint64(height),
		},
	}
	d.calls[peer] = make(chan *types.LightBlock, 1)

	select {
	case resp := <-d.calls[peer]:
		return resp, nil

	case <-ctx.Done():
		return nil, nil
	}
}

func (d *dispatcher) Len() int {
	return d.connectedPeers.Len()
}

func (d *dispatcher) respond(lb *proto.LightBlock, peer p2p.NodeID) error {
	// check that the response came from a request
	receiveCh, ok := d.calls[peer]
	if !ok {
		return errors.New("unsolicited light block response")
	}

	block, err := types.LightBlockFromProto(lb)
	if err != nil {
		return err
	}

	receiveCh <- block
	close(receiveCh)
	delete(d.calls, peer)
	return nil
}

func (d *dispatcher) addPeer(peer p2p.NodeID) {
	d.connectedPeers.PushBack(peer)
}

func (d *dispatcher) removePeer(peerID p2p.NodeID) {
	for e := d.connectedPeers.Front(); e != nil; e = e.Next() {
		if e.Value == peerID {
			d.connectedPeers.Remove(e)
			e.DetachPrev()
			break
		}
	}
}

func (d *dispatcher) nextPeer() p2p.NodeID {
	// NOTE: this should never be nil
	return d.connectedPeers.FrontWait().Value.(p2p.NodeID)
}

// blockProvider is a p2p based light provider which uses a dispatcher connected
// to the state sync reactor to serve light blocks to the light client
//
// TODO: This should probably be moved over to the light package but as we're
// not yet officially supporting p2p light clients we'll leave this here for now.
type blockProvider struct {
	peer       p2p.NodeID
	chainID    string
	timeout    time.Duration
	dispatcher *dispatcher
}

func (p *blockProvider) LightBlock(ctx context.Context, height int64) (*types.LightBlock, error) {
	// FIXME: The provider doesn't know if the dispatcher is still connected to
	// that peer. If the connection is dropped for whatever reason the
	// dispatcher needs to be able to relay this back to the provider so it can
	// return ErrConnectionClosed instead of ErrNoResponse
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	lb, err := p.dispatcher.lightBlock(ctx, height, p.peer)
	if err != nil {
		return nil, provider.ErrBadLightBlock{Reason: err}
	}

	if lb == nil {
		return nil, provider.ErrNoResponse
	}

	if err := lb.ValidateBasic(p.chainID); err != nil {
		return nil, provider.ErrBadLightBlock{Reason: err}
	}

	return lb, nil
}

// ReportEvidence should allow for the light client to report any light client
// attacks. This is a no op as there currently isn't a way to wire this up to
// the evidence reactor (we should endeavour to do this in the future)
func (p *blockProvider) ReportEvidence(ctx context.Context, ev types.Evidence) error {
	return nil
}

// String implements stringer interface
func (p *blockProvider) String() string { return string(p.peer) }
