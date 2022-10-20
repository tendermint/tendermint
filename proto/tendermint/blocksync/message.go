package blocksync

import (
	"fmt"

	"github.com/gogo/protobuf/proto"
)

const (
	BlockResponseMessagePrefixSize   = 4
	BlockResponseMessageFieldKeySize = 1
)

// Wrap implements the p2p Wrapper interface and wraps a blockchain message.
func (m *Message) Wrap(pb proto.Message) error {
	switch msg := pb.(type) {
	case *BlockRequest:
		m.Sum = &Message_BlockRequest{BlockRequest: msg}

	case *BlockResponse:
		m.Sum = &Message_BlockResponse{BlockResponse: msg}

	case *NoBlockResponse:
		m.Sum = &Message_NoBlockResponse{NoBlockResponse: msg}

	case *StatusRequest:
		m.Sum = &Message_StatusRequest{StatusRequest: msg}

	case *StatusResponse:
		m.Sum = &Message_StatusResponse{StatusResponse: msg}

	default:
		return fmt.Errorf("unknown message: %T", msg)
	}

	return nil
}

// Unwrap implements the p2p Wrapper interface and unwraps a wrapped blockchain
// message.
func (m *Message) Unwrap() (proto.Message, error) {
	switch msg := m.Sum.(type) {
	case *Message_BlockRequest:
		return m.GetBlockRequest(), nil

	case *Message_BlockResponse:
		return m.GetBlockResponse(), nil

	case *Message_NoBlockResponse:
		return m.GetNoBlockResponse(), nil

	case *Message_StatusRequest:
		return m.GetStatusRequest(), nil

	case *Message_StatusResponse:
		return m.GetStatusResponse(), nil

	default:
		return nil, fmt.Errorf("unknown message: %T", msg)
	}
}
