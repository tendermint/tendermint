package privval

import (
	"context"
	"fmt"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/encoding"
	cryptoproto "github.com/tendermint/tendermint/proto/tendermint/crypto"
	privvalproto "github.com/tendermint/tendermint/proto/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

func DefaultValidationRequestHandler(
	ctx context.Context,
	privVal types.PrivValidator,
	req privvalproto.Message,
	chainID string,
) (privvalproto.Message, error) {
	var (
		res privvalproto.Message
		err error
	)

	switch r := req.Sum.(type) {
	case *privvalproto.Message_PubKeyRequest:
		if r.PubKeyRequest.GetChainId() != chainID {
			res = mustWrapMsg(&privvalproto.PubKeyResponse{
				PubKey: cryptoproto.PublicKey{}, Error: &privvalproto.RemoteSignerError{
					Code: 0, Description: "unable to provide pubkey"}})
			return res, fmt.Errorf("want chainID: %s, got chainID: %s", r.PubKeyRequest.GetChainId(), chainID)
		}

		var pubKey crypto.PubKey
		pubKey, err = privVal.GetPubKey(ctx)
		if err != nil {
			return res, err
		}
		pk, err := encoding.PubKeyToProto(pubKey)
		if err != nil {
			return res, err
		}

		if err != nil {
			res = mustWrapMsg(&privvalproto.PubKeyResponse{
				PubKey: cryptoproto.PublicKey{}, Error: &privvalproto.RemoteSignerError{Code: 0, Description: err.Error()}})
		} else {
			res = mustWrapMsg(&privvalproto.PubKeyResponse{PubKey: pk, Error: nil})
		}

	case *privvalproto.Message_SignVoteRequest:
		if r.SignVoteRequest.ChainId != chainID {
			res = mustWrapMsg(&privvalproto.SignedVoteResponse{
				Vote: tmproto.Vote{}, Error: &privvalproto.RemoteSignerError{
					Code: 0, Description: "unable to sign vote"}})
			return res, fmt.Errorf("want chainID: %s, got chainID: %s", r.SignVoteRequest.GetChainId(), chainID)
		}

		vote := r.SignVoteRequest.Vote

		err = privVal.SignVote(ctx, chainID, vote)
		if err != nil {
			res = mustWrapMsg(&privvalproto.SignedVoteResponse{
				Vote: tmproto.Vote{}, Error: &privvalproto.RemoteSignerError{Code: 0, Description: err.Error()}})
		} else {
			res = mustWrapMsg(&privvalproto.SignedVoteResponse{Vote: *vote, Error: nil})
		}

	case *privvalproto.Message_SignProposalRequest:
		if r.SignProposalRequest.GetChainId() != chainID {
			res = mustWrapMsg(&privvalproto.SignedProposalResponse{
				Proposal: tmproto.Proposal{}, Error: &privvalproto.RemoteSignerError{
					Code:        0,
					Description: "unable to sign proposal"}})
			return res, fmt.Errorf("want chainID: %s, got chainID: %s", r.SignProposalRequest.GetChainId(), chainID)
		}

		proposal := r.SignProposalRequest.Proposal

		err = privVal.SignProposal(ctx, chainID, proposal)
		if err != nil {
			res = mustWrapMsg(&privvalproto.SignedProposalResponse{
				Proposal: tmproto.Proposal{}, Error: &privvalproto.RemoteSignerError{Code: 0, Description: err.Error()}})
		} else {
			res = mustWrapMsg(&privvalproto.SignedProposalResponse{Proposal: *proposal, Error: nil})
		}
	case *privvalproto.Message_PingRequest:
		err, res = nil, mustWrapMsg(&privvalproto.PingResponse{})

	default:
		err = fmt.Errorf("unknown msg: %v", r)
	}

	return res, err
}
