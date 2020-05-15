package privval

import (
	"fmt"

	"github.com/tendermint/tendermint/crypto"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	privvalproto "github.com/tendermint/tendermint/proto/privval"
	"github.com/tendermint/tendermint/types"
)

func DefaultValidationRequestHandler(
	privVal types.PrivValidator,
	req privvalproto.Message,
	chainID string,
) (*privvalproto.Message, error) {
	var (
		res *privvalproto.Message
		err error
	)

	switch r := req.Sum.(type) {
	case *privvalproto.Message_PubKeyRequest:
		var pubKey crypto.PubKey
		pubKey, err = privVal.GetPubKey()
		pk, err := cryptoenc.PubKeyToProto(pubKey)
		if err != nil {
			return nil, err
		}

		if err != nil {
			res = mustWrapMsg(&privvalproto.PubKeyResponse{PubKey: nil, Error: &privvalproto.RemoteSignerError{Code: 0, Description: err.Error()}})
		} else {
			res = mustWrapMsg(&privvalproto.PubKeyResponse{PubKey: &pk, Error: nil})
		}

	case *privvalproto.Message_SignVoteRequest:
		v, err := types.VoteFromProto(r.SignVoteRequest.Vote)
		if err != nil {
			return nil, err
		}

		err = privVal.SignVote(chainID, v)
		if err != nil {
			res = mustWrapMsg(&privvalproto.SignedVoteResponse{Vote: nil, Error: &privvalproto.RemoteSignerError{Code: 0, Description: err.Error()}})
		} else {
			vpb := v.ToProto()
			res = mustWrapMsg(&privvalproto.SignedVoteResponse{Vote: vpb, Error: nil})
		}

	case *privvalproto.Message_SignProposalRequest:
		p, err := types.ProposalFromProto(&r.SignProposalRequest.Proposal)
		if err != nil {
			return nil, err
		}

		err = privVal.SignProposal(chainID, p)
		if err != nil {
			res = mustWrapMsg(&privvalproto.SignedProposalResponse{Proposal: nil, Error: &privvalproto.RemoteSignerError{Code: 0, Description: err.Error()}})
		} else {
			ppb := p.ToProto()
			res = mustWrapMsg(&privvalproto.SignedProposalResponse{Proposal: ppb, Error: nil})
		}

	case *privvalproto.Message_PingRequest:
		err, res = nil, mustWrapMsg(&privvalproto.PingResponse{})

	default:
		err = fmt.Errorf("unknown msg: %v", r)
	}

	return res, err
}
