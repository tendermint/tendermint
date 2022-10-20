package p2p

import (
	"fmt"

	"github.com/cosmos/gogoproto/proto"
)

func (m *PexAddrs) Wrap() (proto.Message, error) {
	pm := &Message{}
	pm.Sum = &Message_PexAddrs{PexAddrs: m}
	return pm, nil
}

func (m *PexRequest) Wrap() (proto.Message, error) {
	pm := &Message{}
	pm.Sum = &Message_PexRequest{PexRequest: m}
	return pm, nil
}

// Unwrap implements the p2p Wrapper interface and unwraps a wrapped PEX
// message.
func (m *Message) Unwrap() (proto.Message, error) {
	switch msg := m.Sum.(type) {
	case *Message_PexRequest:
		return msg.PexRequest, nil
	case *Message_PexAddrs:
		return msg.PexAddrs, nil
	default:
		return nil, fmt.Errorf("unknown pex message: %T", msg)
	}
}
