package privval

import (
	"fmt"

	"github.com/gogo/protobuf/proto"

	privvalproto "github.com/tendermint/tendermint/proto/tendermint/privval"
)

// TODO: Add ChainIDRequest

func mustWrapMsg(pb proto.Message) privvalproto.Message {
	msg := privvalproto.Message{}

	switch pb := pb.(type) {
	case *privvalproto.Message:
		msg = *pb
	case *privvalproto.PubKeyRequest:
		msg.Sum = &privvalproto.Message_PubKeyRequest{PubKeyRequest: pb}
	case *privvalproto.PubKeyResponse:
		msg.Sum = &privvalproto.Message_PubKeyResponse{PubKeyResponse: pb}
	case *privvalproto.SignVoteRequest:
		msg.Sum = &privvalproto.Message_SignVoteRequest{SignVoteRequest: pb}
	case *privvalproto.SignedVoteResponse:
		msg.Sum = &privvalproto.Message_SignedVoteResponse{SignedVoteResponse: pb}
	case *privvalproto.SignedProposalResponse:
		msg.Sum = &privvalproto.Message_SignedProposalResponse{SignedProposalResponse: pb}
	case *privvalproto.SignProposalRequest:
		msg.Sum = &privvalproto.Message_SignProposalRequest{SignProposalRequest: pb}
	case *privvalproto.PingRequest:
		msg.Sum = &privvalproto.Message_PingRequest{}
	case *privvalproto.PingResponse:
		msg.Sum = &privvalproto.Message_PingResponse{}
	default:
		panic(fmt.Errorf("unknown message type %T", msg))
	}

	return msg
}
