package p2p

import (
	"github.com/tendermint/tendermint/p2p/conn"
	cmn "github.com/tendermint/tmlibs/common"
)

type Reactor interface {
	cmn.Service // Start, Stop

	SetSwitch(*Switch)
	GetChannels() []*conn.ChannelDescriptor
	AddPeer(peer Peer)
	RemovePeer(peer Peer, reason interface{})
	Receive(chID byte, peer Peer, msgBytes []byte) // CONTRACT: msgBytes are not nil
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
func (_ *BaseReactor) GetChannels() []*conn.ChannelDescriptor        { return nil }
func (_ *BaseReactor) AddPeer(peer Peer)                             {}
func (_ *BaseReactor) RemovePeer(peer Peer, reason interface{})      {}
func (_ *BaseReactor) Receive(chID byte, peer Peer, msgBytes []byte) {}
