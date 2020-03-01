package reactor

import (
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

const timeout = 3 * time.Second

// Dispatcher manages requests and responses to specific peers.
type Dispatcher struct {
	sync.Mutex
	logger   log.Logger
	chHeader map[p2p.ID]map[int64][]chan<- *types.SignedHeader
}

// NewDispatcher creates a new dispatcher
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		logger:   log.NewNopLogger(),
		chHeader: make(map[p2p.ID]map[int64][]chan<- *types.SignedHeader),
	}
}

// RequestSignedHeader synchronously requests a signed header from a peer.
func (d *Dispatcher) RequestSignedHeader(peer p2p.Peer, height int64) (*types.SignedHeader, error) {
	d.Lock()
	ch := make(chan *types.SignedHeader)
	if _, ok := d.chHeader[peer.ID()]; !ok {
		d.chHeader[peer.ID()] = make(map[int64][]chan<- *types.SignedHeader)
	}
	d.chHeader[peer.ID()][height] = append(d.chHeader[peer.ID()][height], ch)
	d.Unlock()

	d.logger.Info("Requesting signed header", "peer", peer.ID(), "height", height)
	if !peer.Send(LiteChannel, cdc.MustMarshalBinaryBare(&signedHeaderRequestMessage{
		Height: height,
	})) {
		return nil, errors.Errorf("failed to request signed header from %q", peer.ID())
	}

	select {
	case resp := <-ch:
		d.logger.Info("Received signed header", "peer", peer.ID(), "height", height)
		return resp, nil
	case <-time.After(timeout):
		return nil, errors.New("timed out requesting signed header")
	}
}

// RespondSignedHeader provides a signed header response from a peer. The header can be nil
// if it was not found.
func (d *Dispatcher) RespondSignedHeader(peer p2p.Peer, reqHeight int64, header *types.SignedHeader) {
	d.Lock()
	defer d.Unlock()
	// Need to check peer map existence explicitly, since we'll be writing to it below
	if _, ok := d.chHeader[peer.ID()]; !ok {
		return
	}
	for _, ch := range d.chHeader[peer.ID()][reqHeight] {
		// Send header if the channel has a blocked listener, otherwise ignore it (e.g. timed out)
		select {
		case ch <- header:
		default:
		}
	}
	d.chHeader[peer.ID()][reqHeight] = make([]chan<- *types.SignedHeader, 0)
}
