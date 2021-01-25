package p2p

import (
	fmt "fmt"

	proto "github.com/gogo/protobuf/proto"
)

// Wrap implements the p2p Wrapper interface and wraps a PEX message.
func (m *Message) Wrap(pb proto.Message) error {
	switch msg := pb.(type) {
	case *PexRequest:
		m.Sum = &Message_PexRequest{PexRequest: msg}
	case *PexAddrs:
		m.Sum = &Message_PexAddrs{PexAddrs: msg}
	default:
		return fmt.Errorf("unknown message: %T", msg)
	}
	return nil
}

// Unwrap implements the p2p Wrapper interface and unwraps a wrapped PEX
// message.
func (m *Message) Unwrap() (proto.Message, error) {
	switch msg := m.Sum.(type) {
	case *Message_PexRequest:
		return m.GetPexRequest(), nil
	case *Message_PexAddrs:
		return m.GetPexAddrs(), nil
	default:
		return nil, fmt.Errorf("unknown message: %T", msg)
	}
}
