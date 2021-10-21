package p2p

import (
	"fmt"

	"github.com/gogo/protobuf/proto"
)

// Wrap implements the p2p Wrapper interface and wraps a PEX message.
func (m *PexMessage) Wrap(pb proto.Message) error {
	switch msg := pb.(type) {
	case *PexRequest:
		m.Sum = &PexMessage_PexRequest{PexRequest: msg}
	case *PexResponse:
		m.Sum = &PexMessage_PexResponse{PexResponse: msg}
	default:
		return fmt.Errorf("unknown pex message: %T", msg)
	}
	return nil
}

// Unwrap implements the p2p Wrapper interface and unwraps a wrapped PEX
// message.
func (m *PexMessage) Unwrap() (proto.Message, error) {
	switch msg := m.Sum.(type) {
	case *PexMessage_PexRequest:
		return msg.PexRequest, nil
	case *PexMessage_PexResponse:
		return msg.PexResponse, nil
	default:
		return nil, fmt.Errorf("unknown pex message: %T", msg)
	}
}
