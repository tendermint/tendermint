package privval

import (
	"fmt"

	"github.com/gogo/protobuf/proto"

	"github.com/tendermint/tendermint/crypto"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	privvalproto "github.com/tendermint/tendermint/proto/privval"
	"github.com/tendermint/tendermint/types"
)

func DefaultValidationRequestHandler(
	privVal types.PrivValidator,
	req proto.Message,
	chainID string,
) (proto.Message, error) {
	var (
		res proto.Message
		err error
	)

	fmt.Println("here", req.String(), chainID)

	switch r := req.(type) {
	case *privvalproto.PubKeyRequest:
		var pubKey crypto.PubKey
		pubKey, err = privVal.GetPubKey()
		pk, err := cryptoenc.PubKeyToProto(pubKey)
		if err != nil {
			return nil, err
		}

		if err != nil {
			res = &privvalproto.PubKeyResponse{PubKey: nil, Error: &privvalproto.RemoteSignerError{Code: 0, Description: err.Error()}}
		} else {
			res = &privvalproto.PubKeyResponse{PubKey: &pk, Error: nil}
		}

	case *privvalproto.SignVoteRequest:
		v, err := types.VoteFromProto(r.Vote)
		if err != nil {
			return nil, err
		}

		err = privVal.SignVote(chainID, v)
		if err != nil {
			res = &privvalproto.SignedVoteResponse{Vote: nil, Error: &privvalproto.RemoteSignerError{Code: 0, Description: err.Error()}}
		} else {
			vpb := v.ToProto()
			res = &privvalproto.SignedVoteResponse{Vote: vpb, Error: nil}
		}

	case *privvalproto.SignProposalRequest:
		p, err := types.ProposalFromProto(&r.Proposal)
		if err != nil {
			return nil, err
		}

		err = privVal.SignProposal(chainID, p)
		if err != nil {
			res = &privvalproto.SignedProposalResponse{Proposal: nil, Error: &privvalproto.RemoteSignerError{Code: 0, Description: err.Error()}}
		} else {
			ppb := p.ToProto()
			res = &privvalproto.SignedProposalResponse{Proposal: ppb, Error: nil}
		}

	case *privvalproto.PingRequest:
		err, res = nil, &privvalproto.PingResponse{}

	default:
		err = fmt.Errorf("unknown msg: %v", r)
	}

	return res, err
}
