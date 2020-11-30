package statesync

import (
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"

	"github.com/tendermint/tendermint/p2p"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
)

const (
	// snapshotMsgSize is the maximum size of a snapshotResponseMessage
	snapshotMsgSize = int(4e6)
	// chunkMsgSize is the maximum size of a chunkResponseMessage
	chunkMsgSize = int(16e6)
)

// assert Wrapper interface implementation of the state sync proto message type.
var _ p2p.Wrapper = (*ssproto.Message)(nil)

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
