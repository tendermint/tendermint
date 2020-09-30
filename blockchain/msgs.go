package blockchain

import (
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"

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

// EncodeMsg encodes a Protobuf message
func EncodeMsg(pb proto.Message) ([]byte, error) {
	msg := bcproto.Message{}

	switch pb := pb.(type) {
	case *bcproto.BlockRequest:
		msg.Sum = &bcproto.Message_BlockRequest{BlockRequest: pb}
	case *bcproto.BlockResponse:
		msg.Sum = &bcproto.Message_BlockResponse{BlockResponse: pb}
	case *bcproto.NoBlockResponse:
		msg.Sum = &bcproto.Message_NoBlockResponse{NoBlockResponse: pb}
	case *bcproto.StatusRequest:
		msg.Sum = &bcproto.Message_StatusRequest{StatusRequest: pb}
	case *bcproto.StatusResponse:
		msg.Sum = &bcproto.Message_StatusResponse{StatusResponse: pb}
	default:
		return nil, fmt.Errorf("unknown message type %T", pb)
	}

	bz, err := proto.Marshal(&msg)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal %T: %w", pb, err)
	}

	return bz, nil
}

// DecodeMsg decodes a Protobuf message.
func DecodeMsg(bz []byte) (proto.Message, error) {
	pb := &bcproto.Message{}

	err := proto.Unmarshal(bz, pb)
	if err != nil {
		return nil, err
	}

	switch msg := pb.Sum.(type) {
	case *bcproto.Message_BlockRequest:
		return msg.BlockRequest, nil
	case *bcproto.Message_BlockResponse:
		return msg.BlockResponse, nil
	case *bcproto.Message_NoBlockResponse:
		return msg.NoBlockResponse, nil
	case *bcproto.Message_StatusRequest:
		return msg.StatusRequest, nil
	case *bcproto.Message_StatusResponse:
		return msg.StatusResponse, nil
	default:
		return nil, fmt.Errorf("unknown message type %T", msg)
	}
}

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
		// validate basic is called later when converting from proto
		return nil
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
