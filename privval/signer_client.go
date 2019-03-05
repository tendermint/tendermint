package privval

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/types"
)

// SignerClient implements PrivValidator.
// It uses a validator endpoint to request signatures from an external process.
type SignerClient struct {
	endpoint *SignerListenerEndpoint

	// memoized
	consensusPubKey crypto.PubKey
}

// Check that SignerClient implements PrivValidator.
var _ types.PrivValidator = (*SignerClient)(nil)

// NewSignerClient returns an instance of SignerClient.
func NewSignerClient(endpoint *SignerListenerEndpoint) (*SignerClient, error) {
	if !endpoint.IsRunning() {
		if err := endpoint.Start(); err != nil {
			return nil, errors.Wrap(err, "failed to start private validator")
		}
	}

	// TODO: Fix this
	//// retrieve and memoize the consensus public key once.
	//pubKey, err := getPubKey(conn)
	//if err != nil {
	//	return nil, cmn.ErrorWrap(err, "error while retrieving public key for remote signer")
	//}
	// TODO: Fix this

	//return &SignerClient{endpoint: endpoint, consensusPubKey: pubKey,}, nil
	return &SignerClient{endpoint: endpoint}, nil
}

// Close calls Close on the underlying net.Conn.
func (sc *SignerClient) Close() error {
	return sc.endpoint.Close()
}

// Close calls Close on the underlying net.Conn.
func (sc *SignerClient) IsConnected() bool {
	return sc.endpoint.IsConnected()
}

// Close calls Close on the underlying net.Conn.
func (sc *SignerClient) WaitForConnection(maxWait time.Duration) error {
	if sc.endpoint == nil {
		return fmt.Errorf("endpoint has not been defined")
	}
	return sc.endpoint.WaitForConnection(maxWait)
}

//--------------------------------------------------------
// Implement PrivValidator

// GetPubKey implements PrivValidator.
func (sc *SignerClient) GetPubKey() crypto.PubKey {
	response, err := sc.endpoint.SendRequest(&PubKeyRequest{})
	if err != nil {
		return nil
	}

	pubKeyResp, ok := response.(*PubKeyResponse)
	if !ok {
		sc.endpoint.Logger.Error("response is not PubKeyResponse")
		return nil
	}

	if pubKeyResp.Error != nil {
		sc.endpoint.Logger.Error("failed to get private validator's public key", "err", pubKeyResp.Error)
		return nil
	}

	return pubKeyResp.PubKey
}

// SignVote implements PrivValidator.
func (sc *SignerClient) SignVote(chainID string, vote *types.Vote) error {
	response, err := sc.endpoint.SendRequest(&SignVoteRequest{Vote: vote})
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
func (sc *SignerClient) SignProposal(chainID string, proposal *types.Proposal) error {
	response, err := sc.endpoint.SendRequest(&SignProposalRequest{Proposal: proposal})
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

func handleRequest(req RemoteSignerMsg, chainID string, privVal types.PrivValidator) (RemoteSignerMsg, error) {
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
