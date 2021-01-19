package statesync

import (
	"errors"
	fmt "fmt"

	proto "github.com/gogo/protobuf/proto"
)

// Wrap implements the p2p Wrapper interface and wraps a state sync messages.
func (m *Message) Wrap(pb proto.Message) error {
	switch msg := pb.(type) {
	case *ChunkRequest:
		m.Sum = &Message_ChunkRequest{ChunkRequest: msg}

	case *ChunkResponse:
		m.Sum = &Message_ChunkResponse{ChunkResponse: msg}

	case *SnapshotsRequest:
		m.Sum = &Message_SnapshotsRequest{SnapshotsRequest: msg}

	case *SnapshotsResponse:
		m.Sum = &Message_SnapshotsResponse{SnapshotsResponse: msg}

	default:
		return fmt.Errorf("unknown message: %T", msg)
	}

	return nil
}

// Unwrap implements the p2p Wrapper interface and unwraps a wrapped state sync
// message.
func (m *Message) Unwrap() (proto.Message, error) {
	switch msg := m.Sum.(type) {
	case *Message_ChunkRequest:
		return m.GetChunkRequest(), nil

	case *Message_ChunkResponse:
		return m.GetChunkResponse(), nil

	case *Message_SnapshotsRequest:
		return m.GetSnapshotsRequest(), nil

	case *Message_SnapshotsResponse:
		return m.GetSnapshotsResponse(), nil

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
	case *Message_ChunkRequest:
		if m.GetChunkRequest().Height == 0 {
			return errors.New("height cannot be 0")
		}

	case *Message_ChunkResponse:
		if m.GetChunkResponse().Height == 0 {
			return errors.New("height cannot be 0")
		}
		if m.GetChunkResponse().Missing && len(m.GetChunkResponse().Chunk) > 0 {
			return errors.New("missing chunk cannot have contents")
		}
		if !m.GetChunkResponse().Missing && m.GetChunkResponse().Chunk == nil {
			return errors.New("chunk cannot be nil")
		}

	case *Message_SnapshotsRequest:

	case *Message_SnapshotsResponse:
		if m.GetSnapshotsResponse().Height == 0 {
			return errors.New("height cannot be 0")
		}
		if len(m.GetSnapshotsResponse().Hash) == 0 {
			return errors.New("snapshot has no hash")
		}
		if m.GetSnapshotsResponse().Chunks == 0 {
			return errors.New("snapshot has no chunks")
		}

	default:
		return fmt.Errorf("unknown message type: %T", msg)
	}

	return nil
}
