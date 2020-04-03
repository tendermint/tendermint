package blockchain

import (
	"errors"

	amino "github.com/tendermint/go-amino"

	bcproto "github.com/tendermint/tendermint/proto/blockchain"
	"github.com/tendermint/tendermint/types"
)

var Cdc = amino.NewCodec()

func init() {
	RegisterBlockchainMessages(Cdc)
	types.RegisterBlockAmino(Cdc)
}

func MsgToProto(msg Message) (*bcproto.Message, error) {
	switch msg := msg.(type) {
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

		block, err := msg.Block.ToProto()
		if err != nil {
			return nil, err
		}
		bm := bcproto.Message{
			Sum: &bcproto.Message_BlockResponse{
				BlockResponse: &bcproto.BlockResponse{
					Block: block,
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

	case *StatusRequestMessage:
		bm := bcproto.Message{
			Sum: &bcproto.Message_StatusRequest{
				StatusRequest: &bcproto.StatusRequest{
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
				},
			},
		}
		return &bm, nil

	default:
		return nil, errors.New("blockchain message not recognized")
	}
}

func MsgFromProto(bcm bcproto.Message) (*Message, error) {

	return nil, err
}
