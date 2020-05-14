package privval

import (
	"fmt"

	"github.com/gogo/protobuf/proto"

	privvalproto "github.com/tendermint/tendermint/proto/privval"
)

// TODO: Add ChainIDRequest

func mustWrapMsg(pb proto.Message) *privvalproto.Message {
	msg := privvalproto.Message{}

	switch pb := pb.(type) {
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

	return &msg
}

func mustUnwrapMsg(msg privvalproto.Message) proto.Message {

	switch msg := msg.Sum.(type) {
	case *privvalproto.Message_PubKeyRequest:
		return msg.PubKeyRequest
	case *privvalproto.Message_PubKeyResponse:
		return msg.PubKeyResponse
	case *privvalproto.Message_SignVoteRequest:
		return msg.SignVoteRequest
	case *privvalproto.Message_SignedVoteResponse:
		return msg.SignedVoteResponse
	case *privvalproto.Message_SignProposalRequest:
		return msg.SignProposalRequest
	case *privvalproto.Message_SignedProposalResponse:
		return msg.SignedProposalResponse
	case *privvalproto.Message_PingRequest:
		return msg.PingRequest
	case *privvalproto.Message_PingResponse:
		return msg.PingResponse
	default:
		panic(fmt.Errorf("unknown message type %T", msg))
	}
}
