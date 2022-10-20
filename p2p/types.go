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

// Wrapper is a Protobuf message that can contain a variety of inner messages
// (e.g. via oneof fields). If a Channel's message type implements Wrapper, the
// Router will automatically wrap outbound messages and unwrap inbound messages,
// such that reactors do not have to do this themselves.
type Unwrapper interface {
	proto.Message

	// Unwrap will unwrap the inner message contained in this message.
	Unwrap() (proto.Message, error)
}

type Wrapper interface {
	// Wrap will take the underlying message and wrap it in its wrapper type.
	Wrap() (proto.Message, error)
}
