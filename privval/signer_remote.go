package privval

import (
	"fmt"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/types"
)

// SignerRemote implements PrivValidator.
// It uses a net.Conn to request signatures from an external process.
type SignerRemote struct {
	endpoint *SignerValidatorEndpoint

	// memoized
	consensusPubKey crypto.PubKey
}

// Check that SignerRemote implements PrivValidator.
var _ types.PrivValidator = (*SignerRemote)(nil)

// NewSignerRemote returns an instance of SignerRemote.
func NewSignerRemote(endpoint *SignerValidatorEndpoint) (*SignerRemote, error) {

	// TODO: Fix this
	//// retrieve and memoize the consensus public key once.
	//pubKey, err := getPubKey(conn)
	//if err != nil {
	//	return nil, cmn.ErrorWrap(err, "error while retrieving public key for remote signer")
	//}
	// TODO: Fix this

	//return &SignerRemote{endpoint: endpoint, consensusPubKey: pubKey,}, nil
	return &SignerRemote{endpoint: endpoint}, nil
}

// Close calls Close on the underlying net.Conn.
func (sr *SignerRemote) Close() error {
	return sr.endpoint.Close()
}

//--------------------------------------------------------
// Implement PrivValidator

// GetPubKey implements PrivValidator.
func (sr *SignerRemote) GetPubKey() crypto.PubKey {
	response, err := sr.endpoint.SendRequest(&PubKeyRequest{})
	if err != nil {
		return nil
	}

	pubKeyResp, ok := response.(*PubKeyResponse)
	if !ok {
		sr.endpoint.Logger.Error("response is not PubKeyResponse")
		return nil
	}

	if pubKeyResp.Error != nil {
		sr.endpoint.Logger.Error("failed to get private validator's public key", "err", pubKeyResp.Error)
		return nil
	}

	return pubKeyResp.PubKey
}

// SignVote implements PrivValidator.
func (sr *SignerRemote) SignVote(chainID string, vote *types.Vote) error {
	response, err := sr.endpoint.SendRequest(&SignVoteRequest{Vote: vote})
	if err != nil {
		return err
	}

	resp, ok := response.(*SignedVoteResponse)
	if !ok {
		return ErrUnexpectedResponse
	}

	if resp.Error != nil {
		return resp.Error
	}
	*vote = *resp.Vote

	return nil
}

// SignProposal implements PrivValidator.
func (sr *SignerRemote) SignProposal(chainID string, proposal *types.Proposal) error {
	response, err := sr.endpoint.SendRequest(&SignProposalRequest{Proposal: proposal})
	if err != nil {
		return err
	}

	resp, ok := response.(*SignedProposalResponse)
	if !ok {
		return ErrUnexpectedResponse
	}
	if resp.Error != nil {
		return resp.Error
	}
	*proposal = *resp.Proposal

	return nil
}

func handleRequest(
	req RemoteSignerMsg,
	chainID string,
	privVal types.PrivValidator) (RemoteSignerMsg, error) {

	var res RemoteSignerMsg
	var err error

	switch r := req.(type) {
	case *PubKeyRequest:
		var p crypto.PubKey
		p = privVal.GetPubKey()
		res = &PubKeyResponse{p, nil}

	case *SignVoteRequest:
		err = privVal.SignVote(chainID, r.Vote)
		if err != nil {
			res = &SignedVoteResponse{nil, &RemoteSignerError{0, err.Error()}}
		} else {
			res = &SignedVoteResponse{r.Vote, nil}
		}

	case *SignProposalRequest:
		err = privVal.SignProposal(chainID, r.Proposal)
		if err != nil {
			res = &SignedProposalResponse{nil, &RemoteSignerError{0, err.Error()}}
		} else {
			res = &SignedProposalResponse{r.Proposal, nil}
		}

	case *PingRequest:
		res = &PingResponse{}

	default:
		err = fmt.Errorf("unknown msg: %v", r)
	}

	return res, err
}
