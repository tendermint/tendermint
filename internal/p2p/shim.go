package p2p

import (
	"github.com/gogo/protobuf/proto"
)

// ChannelDescriptorShim defines a shim wrapper around a legacy p2p channel
// and the proto.Message the new p2p Channel is responsible for handling.
// A ChannelDescriptorShim is not contained in ReactorShim, but is rather
// used to construct a ReactorShim.
type ChannelDescriptorShim struct {
	MsgType    proto.Message
	Descriptor *ChannelDescriptor
}
