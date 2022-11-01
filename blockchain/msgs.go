package blockchain

import (
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"

	"github.com/tendermint/tendermint/p2p"
	bcproto "github.com/tendermint/tendermint/proto/tendermint/blockchain"
	"github.com/tendermint/tendermint/types"
)

const (
	// NOTE: keep up to date with bcproto.BlockResponse
	BlockResponseMessagePrefixSize   = 4
	BlockResponseMessageFieldKeySize = 1
	MaxMsgSize                       = types.MaxBlockSizeBytes +
		BlockResponseMessagePrefixSize +
		BlockResponseMessageFieldKeySize
)

// ValidateMsg validates a message.
func ValidateMsg(pb proto.Message) error {
	if pb == nil {
		return errors.New("message cannot be nil")
	}

	switch msg := pb.(type) {
	case *bcproto.BlockRequest:
		if msg.Height < 0 {
			return errors.New("negative Height")
		}
	case *bcproto.BlockResponse:
		_, err := types.BlockFromProto(msg.Block)
		if err != nil {
			return err
		}
	case *bcproto.NoBlockResponse:
		if msg.Height < 0 {
			return errors.New("negative Height")
		}
	case *bcproto.StatusResponse:
		if msg.Base < 0 {
			return errors.New("negative Base")
		}
		if msg.Height < 0 {
			return errors.New("negative Height")
		}
		if msg.Base > msg.Height {
			return fmt.Errorf("base %v cannot be greater than height %v", msg.Base, msg.Height)
		}
	case *bcproto.StatusRequest:
		return nil
	default:
		return fmt.Errorf("unknown message type %T", msg)
	}
	return nil
}

// EncodeMsg encodes a Protobuf message
//
// Deprecated: Will be removed in v0.37.
func EncodeMsg(pb proto.Message) ([]byte, error) {
	if um, ok := pb.(p2p.Wrapper); ok {
		pb = um.Wrap()
	}
	bz, err := proto.Marshal(pb)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal %T: %w", pb, err)
	}

	return bz, nil
}

// DecodeMsg decodes a Protobuf message.
//
// Deprecated: Will be removed in v0.37.
func DecodeMsg(bz []byte) (proto.Message, error) {
	pb := &bcproto.Message{}

	err := proto.Unmarshal(bz, pb)
	if err != nil {
		return nil, err
	}
	return pb.Unwrap()
}
