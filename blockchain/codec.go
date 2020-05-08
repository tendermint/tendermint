package blockchain

import (
	"errors"

	bcproto "github.com/tendermint/tendermint/proto/blockchain"
	"github.com/tendermint/tendermint/types"
)

func MsgToProto(bcm Message) (*bcproto.Message, error) {
	if bcm == nil {
		return nil, errors.New("message is nil")
	}

	switch msg := bcm.(type) {
	case *BlockRequestMessage:
		bm := bcproto.Message{
			Sum: &bcproto.Message_BlockRequest{
				BlockRequest: &bcproto.BlockRequest{
					Height: msg.Height,
				},
			},
		}
		return &bm, nil
	case *BlockResponseMessage:
		b, err := msg.Block.ToProto()
		if err != nil {
			return nil, err
		}

		bm := bcproto.Message{
			Sum: &bcproto.Message_BlockResponse{
				BlockResponse: &bcproto.BlockResponse{
					Block: *b,
				},
			},
		}
		return &bm, nil
	case *NoBlockResponseMessage:
		bm := bcproto.Message{
			Sum: &bcproto.Message_NoBlockResponse{
				NoBlockResponse: &bcproto.NoBlockResponse{
					Height: msg.Height,
				},
			},
		}
		return &bm, nil
	case *StatusResponseMessage:
		bm := bcproto.Message{
			Sum: &bcproto.Message_StatusResponse{
				StatusResponse: &bcproto.StatusResponse{
					Height: msg.Height,
					Base:   msg.Base,
				},
			},
		}
		return &bm, nil
	case *StatusRequestMessage:
		bm := bcproto.Message{
			Sum: &bcproto.Message_StatusRequest{
				StatusRequest: &bcproto.StatusRequest{
					Height: msg.Height,
					Base:   msg.Base,
				},
			},
		}
		return &bm, nil
	default:
		return nil, errors.New("message is not recognized")
	}
}

func MsgFromProto(bcm *bcproto.Message) (Message, error) {
	if bcm == nil {
		return nil, errors.New("message is nil")
	}

	var bm Message
	switch msg := bcm.Sum.(type) {
	case *bcproto.Message_BlockRequest:
		bm = &BlockRequestMessage{Height: msg.BlockRequest.Height}
	case *bcproto.Message_NoBlockResponse:
		bm = &NoBlockResponseMessage{Height: msg.NoBlockResponse.Height}
	case *bcproto.Message_BlockResponse:
		b, err := types.BlockFromProto(&msg.BlockResponse.Block)
		if err != nil {
			return nil, err
		}
		bm = &BlockResponseMessage{Block: b}
	case *bcproto.Message_StatusRequest:
		bm = &StatusRequestMessage{
			Height: msg.StatusRequest.Height,
			Base:   msg.StatusRequest.Base,
		}
	case *bcproto.Message_StatusResponse:
		bm = &StatusResponseMessage{
			Height: msg.StatusResponse.Height,
			Base:   msg.StatusResponse.Base,
		}
	default:
		return nil, errors.New("message is not recognized")
	}

	if err := bm.ValidateBasic(); err != nil {
		return nil, err
	}

	return bm, nil
}
