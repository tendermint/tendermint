package privval

import (
	"fmt"
	"reflect"

	"github.com/dashevo/dashd-go/btcjson"

	"github.com/gogo/protobuf/proto"
	"github.com/tendermint/tendermint/crypto"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	cryptoproto "github.com/tendermint/tendermint/proto/tendermint/crypto"
	privvalproto "github.com/tendermint/tendermint/proto/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

func DefaultValidationRequestHandler(
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
		res, err = handleKeyRequest(
			r.PubKeyRequest.QuorumHash, r.PubKeyRequest.GetChainId,
			computePubKeyResponse, chainID, privVal, "unable to provide pubkey")
	case *privvalproto.Message_ThresholdPubKeyRequest:
		res, err = handleKeyRequest(
			r.ThresholdPubKeyRequest.QuorumHash, r.ThresholdPubKeyRequest.GetChainId,
			computeThresholdPubKeyResponse, chainID, privVal, "unable to provide threshold pubkey")
	case *privvalproto.Message_ProTxHashRequest:
		if r.ProTxHashRequest.GetChainId() != chainID {
			res = mustWrapMsg(&privvalproto.ProTxHashResponse{
				ProTxHash: nil, Error: &privvalproto.RemoteSignerError{
					Code: 0, Description: "unable to provide proTxHash"}})
			return res, fmt.Errorf("want chainID: %s, got chainID: %s", r.ProTxHashRequest.GetChainId(), chainID)
		}

		var proTxHash crypto.ProTxHash
		proTxHash, err = privVal.GetProTxHash()
		if err != nil {
			return res, err
		}

		if err != nil {
			res = mustWrapMsg(&privvalproto.ProTxHashResponse{
				ProTxHash: nil, Error: &privvalproto.RemoteSignerError{Code: 0, Description: err.Error()}})
		} else {
			res = mustWrapMsg(&privvalproto.ProTxHashResponse{ProTxHash: proTxHash, Error: nil})
		}

	case *privvalproto.Message_SignVoteRequest:
		if r.SignVoteRequest.ChainId != chainID {
			res = mustWrapMsg(&privvalproto.SignedVoteResponse{
				Vote: tmproto.Vote{}, Error: &privvalproto.RemoteSignerError{
					Code: 0, Description: "unable to sign vote"}})
			return res, fmt.Errorf("want chainID: %s, got chainID: %s", r.SignVoteRequest.GetChainId(), chainID)
		}

		vote := r.SignVoteRequest.Vote
		voteQuorumHash := r.SignVoteRequest.QuorumHash
		voteQuorumType := r.SignVoteRequest.QuorumType
		stateID := r.SignVoteRequest.GetStateId()
		if stateID == nil || reflect.DeepEqual(*stateID, tmproto.StateID{}) {
			res = mustWrapMsg(&privvalproto.SignedVoteResponse{
				Vote: tmproto.Vote{},
				Error: &privvalproto.RemoteSignerError{
					Code:        0,
					Description: "State ID not provided"},
			})
			break
		}

		err = privVal.SignVote(chainID, btcjson.LLMQType(voteQuorumType), voteQuorumHash, vote, *stateID, nil)
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

		proposalQuorumHash := r.SignProposalRequest.QuorumHash
		proposalQuorumType := r.SignProposalRequest.QuorumType
		_, err = privVal.SignProposal(chainID, btcjson.LLMQType(proposalQuorumType), proposalQuorumHash, proposal)
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

// computeKeyResponse is a function type for key response generation
type computeKeyResponse func(pubkey cryptoproto.PublicKey, err *privvalproto.RemoteSignerError) proto.Message

// computePubKeyResponse returns PubKeyResponse type
func computePubKeyResponse(pubKey cryptoproto.PublicKey, err *privvalproto.RemoteSignerError) proto.Message {
	return &privvalproto.PubKeyResponse{
		PubKey: pubKey,
		Error:  err,
	}
}

// computeThresholdPubKeyResponse returns ThresholdPubKeyResponse
func computeThresholdPubKeyResponse(pubKey cryptoproto.PublicKey, err *privvalproto.RemoteSignerError) proto.Message {
	return &privvalproto.ThresholdPubKeyResponse{
		PubKey: pubKey,
		Error:  err,
	}
}

// getChainID is a function type for getting chainID of a request
type getChainID func() string

// handleKeyRequest handles key message requests
func handleKeyRequest(
	quorumHash crypto.QuorumHash, getChainIDFn getChainID, keyResponseFn computeKeyResponse,
	chainID string, privVal types.PrivValidator, description string,
) (res privvalproto.Message, err error) {
	if getChainIDFn() != chainID {
		res = mustWrapMsg(keyResponseFn(
			cryptoproto.PublicKey{},
			&privvalproto.RemoteSignerError{
				Code:        0,
				Description: description,
			},
		))
		return res, fmt.Errorf("want chainID: %s, got chainID: %s", getChainIDFn(), chainID)
	}

	var pubKey crypto.PubKey
	pubKey, err = privVal.GetPubKey(quorumHash)
	if err != nil {
		res = mustWrapMsg(keyResponseFn(
			cryptoproto.PublicKey{},
			&privvalproto.RemoteSignerError{
				Code:        0,
				Description: err.Error(),
			},
		))
	}

	var pk cryptoproto.PublicKey
	pk, err = cryptoenc.PubKeyToProto(pubKey)

	if err != nil {
		res = mustWrapMsg(keyResponseFn(
			cryptoproto.PublicKey{},
			&privvalproto.RemoteSignerError{
				Code:        0,
				Description: err.Error(),
			},
		))
	} else {
		res = mustWrapMsg(keyResponseFn(pk, nil))
	}
	return res, err
}
