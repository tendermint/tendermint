package p2p

import (
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/p2p/conn"
)

// Reactor is responsible for handling incoming messages on one or more
// Channel. Switch calls GetChannels when reactor is added to it. When a new
// peer joins our node, InitPeer and AddPeer are called. RemovePeer is called
// when the peer is stopped. Receive is called when a message is received on a
// channel associated with this reactor.
//
// Peer#Send or Peer#TrySend should be used to send the message to a peer.
type Reactor interface {
	service.Service // Start, Stop

	// SetSwitch allows setting a switch.
	SetSwitch(*Switch)

	// GetChannels returns the list of MConnection.ChannelDescriptor. Make sure
	// that each ID is unique across all the reactors added to the switch.
	GetChannels() []*conn.ChannelDescriptor

	// InitPeer is called by the switch before the peer is started. Use it to
	// initialize data for the peer (e.g. peer state).
	//
	// NOTE: The switch won't call AddPeer nor RemovePeer if it fails to start
	// the peer. Do not store any data associated with the peer in the reactor
	// itself unless you don't want to have a state, which is never cleaned up.
	InitPeer(peer Peer) Peer

	// AddPeer is called by the switch after the peer is added and successfully
	// started. Use it to start goroutines communicating with the peer.
	AddPeer(peer Peer)

	// RemovePeer is called by the switch when the peer is stopped (due to error
	// or other reason).
	RemovePeer(peer Peer, reason interface{})

	// Receive is called by the switch when msgBytes is received from the peer.
	//
	// NOTE reactor can not keep msgBytes around after Receive completes without
	// copying.
	//
	// CONTRACT: msgBytes are not nil.
	//
	// Only one of Receive or ReceiveEnvelope are called per message. If ReceiveEnvelope
	// is implemented, it will be used, otherwise the switch will fallback to
	// using Receive.
	// Deprecated: Reactors looking to receive data from a peer should implement ReceiveEnvelope.
	// Receive will be deprecated in favor of ReceiveEnvelope in v0.37.
	Receive(chID byte, peer Peer, msgBytes []byte)
}

type EnvelopeReceiver interface {
	// ReceiveEnvelope is called by the switch when an envelope is received from any connected
	// peer on any of the channels registered by the reactor.
	//
	// Only one of Receive or ReceiveEnvelope are called per message. If ReceiveEnvelope
	// is implemented, it will be used, otherwise the switch will fallback to
	// using Receive. Receive will be replaced by ReceiveEnvelope in a future version
	ReceiveEnvelope(Envelope)
}

//--------------------------------------

type BaseReactor struct {
	service.BaseService // Provides Start, Stop, .Quit
	Switch              *Switch
}

func NewBaseReactor(name string, impl Reactor) *BaseReactor {
	return &BaseReactor{
		BaseService: *service.NewBaseService(nil, name, impl),
		Switch:      nil,
	}
}

func (br *BaseReactor) SetSwitch(sw *Switch) {
	br.Switch = sw
}
func (*BaseReactor) GetChannels() []*conn.ChannelDescriptor        { return nil }
func (*BaseReactor) AddPeer(peer Peer)                             {}
func (*BaseReactor) RemovePeer(peer Peer, reason interface{})      {}
func (*BaseReactor) ReceiveEnvelope(e Envelope)                    {}
func (*BaseReactor) Receive(chID byte, peer Peer, msgBytes []byte) {}
func (*BaseReactor) InitPeer(peer Peer) Peer                       { return peer }
