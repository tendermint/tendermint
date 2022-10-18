package p2p

import (
	"github.com/cosmos/gogoproto/proto"
	"github.com/tendermint/tendermint/p2p/conn"
)

type ChannelDescriptor = conn.ChannelDescriptor
type ConnectionStatus = conn.ConnectionStatus

// Envelope contains a message with sender routing info.
type Envelope struct {
	Src       Peer          // sender (empty if outbound)
	Message   proto.Message // message payload
	ChannelID byte
}
