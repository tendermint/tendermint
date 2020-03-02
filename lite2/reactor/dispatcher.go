package reactor

import (
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

const timeout = 5 * time.Second

// call contains call state.
type call struct {
	peerID p2p.ID
	ch     chan<- Message
}

// Dispatcher dispatches request/response calls to peers.
type Dispatcher struct {
	sync.Mutex
	calls      map[uint64]*call
	nextCallID uint64
}

// NewDispatcher creates a new dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		calls: make(map[uint64]*call),
	}
}

// call synchronously sends a request and waits for the response.
func (d *Dispatcher) call(peer p2p.Peer, msg Message) (Message, error) {
	ch := make(chan Message, 1)

	d.Lock()
	callID := d.nextCallID
	d.nextCallID++ // wraps around to 0 on overflow, but that's fine
	msg.SetCallID(callID)
	d.calls[callID] = &call{
		peerID: peer.ID(),
		ch:     ch,
	}
	d.Unlock()

	if !peer.Send(LiteChannel, cdc.MustMarshalBinaryBare(msg)) {
		return nil, errors.New("failed to send call request, peer may have disconnected")
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(timeout):
		d.Lock()
		delete(d.calls, callID) // no need to close channel, gc handles it
		d.Unlock()
		return nil, errors.New("call timed out")
	}
}

// respond provides a call response.
func (d *Dispatcher) respond(src p2p.Peer, msg Message) error {
	d.Lock()
	defer d.Unlock()
	callID := msg.GetCallID()
	call, ok := d.calls[callID]
	if !ok {
		return errors.Errorf("received call response for unknown call %q", callID)
	}
	if call.peerID != src.ID() {
		return errors.Errorf("received call response from wrong peer %q, expected %q",
			src.ID(), call.peerID)
	}
	delete(d.calls, callID) // no need to close channel, gc handles it
	call.ch <- msg
	return nil
}

// SignedHeader synchronously requests a signed header from a peer. It returns nil if not found.
func (d *Dispatcher) SignedHeader(peer p2p.Peer, height int64) (*types.SignedHeader, error) {
	resp, err := d.call(peer, &signedHeaderRequestMessage{
		Height: height,
	})
	if err != nil {
		return nil, err
	}
	switch msg := resp.(type) {
	case *signedHeaderResponseMessage:
		return msg.SignedHeader, nil
	default:
		return nil, errors.Errorf("received unexpected response %T", msg)
	}
}

// ValidatorSet synchronously requests a signed header from a peer. It returns nil if not found.
func (d *Dispatcher) ValidatorSet(peer p2p.Peer, height int64) (*types.ValidatorSet, error) {
	resp, err := d.call(peer, &validatorSetRequestMessage{
		Height: height,
	})
	if err != nil {
		return nil, err
	}
	switch msg := resp.(type) {
	case *validatorSetResponseMessage:
		return msg.ValidatorSet, nil
	default:
		return nil, errors.Errorf("received unexpected response %T", msg)
	}
}
