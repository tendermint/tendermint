package p2p

import (
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/p2p/conn"
)

type Reactor interface {
	cmn.Service // Start, Stop

	// SetSwitch allows setting a switch.
	SetSwitch(*Switch)

	// GetChannels returns the list of channel descriptors.
	GetChannels() []*conn.ChannelDescriptor

	// AddPeer is called by the switch when a new peer is added.
	AddPeer(peer Peer)

	// RemovePeer is called by the switch when the peer is stopped (due to error
	// or other reason).
	RemovePeer(peer Peer, reason interface{})

	// Receive is called when msgBytes is received from peer.
	//
	// NOTE reactor can not keep msgBytes around after Receive completes without
	// copying.
	//
	// CONTRACT: msgBytes are not nil.
	Receive(chID byte, peer Peer, msgBytes []byte)
}

//--------------------------------------

type BaseReactor struct {
	cmn.BaseService // Provides Start, Stop, .Quit
	Switch          *Switch
}

func NewBaseReactor(name string, impl Reactor) *BaseReactor {
	return &BaseReactor{
		BaseService: *cmn.NewBaseService(nil, name, impl),
		Switch:      nil,
	}
}

func (br *BaseReactor) SetSwitch(sw *Switch) {
	br.Switch = sw
}
func (*BaseReactor) GetChannels() []*conn.ChannelDescriptor        { return nil }
func (*BaseReactor) AddPeer(peer Peer)                             {}
func (*BaseReactor) RemovePeer(peer Peer, reason interface{})      {}
func (*BaseReactor) Receive(chID byte, peer Peer, msgBytes []byte) {}
