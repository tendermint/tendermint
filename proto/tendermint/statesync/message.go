package statesync

import (
	"fmt"

	"github.com/gogo/protobuf/proto"
)

// Wrap implements the p2p Wrapper interface and wraps a state sync proto message.
func (m *Message) Wrap() (proto.Message, error) {
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
// proto message.
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
