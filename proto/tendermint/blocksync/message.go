package blocksync

import (
	"errors"
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

// Validate validates the message returning an error upon failure.
func (m *Message) Validate() error {
	if m == nil {
		return errors.New("message cannot be nil")
	}

	switch msg := m.Sum.(type) {
	case *Message_BlockRequest:
		if m.GetBlockRequest().Height < 0 {
			return errors.New("negative Height")
		}

	case *Message_BlockResponse:
		// validate basic is called later when converting from proto
		return nil

	case *Message_NoBlockResponse:
		if m.GetNoBlockResponse().Height < 0 {
			return errors.New("negative Height")
		}

	case *Message_StatusResponse:
		if m.GetStatusResponse().Base < 0 {
			return errors.New("negative Base")
		}
		if m.GetStatusResponse().Height < 0 {
			return errors.New("negative Height")
		}
		if m.GetStatusResponse().Base > m.GetStatusResponse().Height {
			return fmt.Errorf(
				"base %v cannot be greater than height %v",
				m.GetStatusResponse().Base, m.GetStatusResponse().Height,
			)
		}

	case *Message_StatusRequest:
		return nil

	default:
		return fmt.Errorf("unknown message type: %T", msg)
	}

	return nil
}
