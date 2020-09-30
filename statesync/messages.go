package statesync

import (
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"

	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
)

const (
	// snapshotMsgSize is the maximum size of a snapshotResponseMessage
	snapshotMsgSize = int(4e6)
	// chunkMsgSize is the maximum size of a chunkResponseMessage
	chunkMsgSize = int(16e6)
)

// mustEncodeMsg encodes a Protobuf message, panicing on error.
func mustEncodeMsg(pb proto.Message) []byte {
	msg := ssproto.Message{}
	switch pb := pb.(type) {
	case *ssproto.ChunkRequest:
		msg.Sum = &ssproto.Message_ChunkRequest{ChunkRequest: pb}
	case *ssproto.ChunkResponse:
		msg.Sum = &ssproto.Message_ChunkResponse{ChunkResponse: pb}
	case *ssproto.SnapshotsRequest:
		msg.Sum = &ssproto.Message_SnapshotsRequest{SnapshotsRequest: pb}
	case *ssproto.SnapshotsResponse:
		msg.Sum = &ssproto.Message_SnapshotsResponse{SnapshotsResponse: pb}
	default:
		panic(fmt.Errorf("unknown message type %T", pb))
	}
	bz, err := msg.Marshal()
	if err != nil {
		panic(fmt.Errorf("unable to marshal %T: %w", pb, err))
	}
	return bz
}

// decodeMsg decodes a Protobuf message.
func decodeMsg(bz []byte) (proto.Message, error) {
	pb := &ssproto.Message{}
	err := proto.Unmarshal(bz, pb)
	if err != nil {
		return nil, err
	}
	switch msg := pb.Sum.(type) {
	case *ssproto.Message_ChunkRequest:
		return msg.ChunkRequest, nil
	case *ssproto.Message_ChunkResponse:
		return msg.ChunkResponse, nil
	case *ssproto.Message_SnapshotsRequest:
		return msg.SnapshotsRequest, nil
	case *ssproto.Message_SnapshotsResponse:
		return msg.SnapshotsResponse, nil
	default:
		return nil, fmt.Errorf("unknown message type %T", msg)
	}
}

// validateMsg validates a message.
func validateMsg(pb proto.Message) error {
	if pb == nil {
		return errors.New("message cannot be nil")
	}
	switch msg := pb.(type) {
	case *ssproto.ChunkRequest:
		if msg.Height == 0 {
			return errors.New("height cannot be 0")
		}
	case *ssproto.ChunkResponse:
		if msg.Height == 0 {
			return errors.New("height cannot be 0")
		}
		if msg.Missing && len(msg.Chunk) > 0 {
			return errors.New("missing chunk cannot have contents")
		}
		if !msg.Missing && msg.Chunk == nil {
			return errors.New("chunk cannot be nil")
		}
	case *ssproto.SnapshotsRequest:
	case *ssproto.SnapshotsResponse:
		if msg.Height == 0 {
			return errors.New("height cannot be 0")
		}
		if len(msg.Hash) == 0 {
			return errors.New("snapshot has no hash")
		}
		if msg.Chunks == 0 {
			return errors.New("snapshot has no chunks")
		}
	default:
		return fmt.Errorf("unknown message type %T", msg)
	}
	return nil
}
