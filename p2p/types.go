package p2p

import (
	"github.com/cosmos/gogoproto/proto"
	"github.com/tendermint/tendermint/p2p/conn"
)

type ChannelDescriptor = conn.ChannelDescriptor
type ConnectionStatus = conn.ConnectionStatus

type Envelope struct {
	// Src is set when the message was sent by a remote peer.
	Src       Peer
	ChannelID byte
	Message   proto.Message
}
