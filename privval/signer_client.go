package privval

import (
	"errors"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/crypto"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	privvalproto "github.com/tendermint/tendermint/proto/privval"
	tmproto "github.com/tendermint/tendermint/proto/types"
	"github.com/tendermint/tendermint/types"
)

// SignerClient implements PrivValidator.
// Handles remote validator connections that provide signing services
type SignerClient struct {
	endpoint *SignerListenerEndpoint
}

var _ types.PrivValidator = (*SignerClient)(nil)

// NewSignerClient returns an instance of SignerClient.
// it will start the endpoint (if not already started)
func NewSignerClient(endpoint *SignerListenerEndpoint) (*SignerClient, error) {
	if !endpoint.IsRunning() {
		if err := endpoint.Start(); err != nil {
			return nil, fmt.Errorf("failed to start listener endpoint: %w", err)
		}
	}

	return &SignerClient{endpoint: endpoint}, nil
}

// Close closes the underlying connection
func (sc *SignerClient) Close() error {
	return sc.endpoint.Close()
}

// IsConnected indicates with the signer is connected to a remote signing service
func (sc *SignerClient) IsConnected() bool {
	return sc.endpoint.IsConnected()
}

// WaitForConnection waits maxWait for a connection or returns a timeout error
func (sc *SignerClient) WaitForConnection(maxWait time.Duration) error {
	return sc.endpoint.WaitForConnection(maxWait)
}

//--------------------------------------------------------
// Implement PrivValidator

// Ping sends a ping request to the remote signer
func (sc *SignerClient) Ping() error {
	response, err := sc.endpoint.SendRequest(mustWrapMsg(&privvalproto.PingRequest{}))

	if err != nil {
		sc.endpoint.Logger.Error("SignerClient::Ping", "err", err)
		return nil
	}

	pb := response.GetPingResponse()
	if pb == nil {
		sc.endpoint.Logger.Error("SignerClient::Ping", "err", "response != PingResponse")
		return err
	}

	return nil
}

// GetPubKey retrieves a public key from a remote signer
// returns an error if client is not able to provide the key
func (sc *SignerClient) GetPubKey() (crypto.PubKey, error) {
	response, err := sc.endpoint.SendRequest(mustWrapMsg(&privvalproto.PubKeyRequest{}))
	if err != nil {
		sc.endpoint.Logger.Error("SignerClient::GetPubKey", "err", err)
		return nil, fmt.Errorf("send: %w", err)
	}

	pubKeyResp := response.GetPubKeyResponse()
	if pubKeyResp == nil {
		sc.endpoint.Logger.Error("SignerClient::GetPubKey", "err", "response != PubKeyResponse")
		return nil, fmt.Errorf("unexpected response type %T", response)
	}

	if pubKeyResp.Error != nil {
		sc.endpoint.Logger.Error("failed to get private validator's public key", "err", pubKeyResp.Error)
		return nil, fmt.Errorf("remote error: %w", errors.New(pubKeyResp.Error.Description))
	}

	pk, err := cryptoenc.PubKeyFromProto(*pubKeyResp.PubKey)
	if err != nil {
		return nil, err
	}

	return pk, nil
}

// SignVote requests a remote signer to sign a vote
func (sc *SignerClient) SignVote(chainID string, vote *tmproto.Vote) error {

	response, err := sc.endpoint.SendRequest(mustWrapMsg(&privvalproto.SignVoteRequest{Vote: vote}))
	if err != nil {
		sc.endpoint.Logger.Error("SignerClient::SignVote", "err", err)
		return err
	}

	resp := response.GetSignedVoteResponse()
	if resp == nil {
		sc.endpoint.Logger.Error("SignerClient::GetPubKey", "err", "response != SignedVoteResponse")
		return ErrUnexpectedResponse
	}

	if resp.Error != nil {
		return &RemoteSignerError{Code: int(resp.Error.Code), Description: resp.Error.Description}
	}

	*vote = *resp.Vote

	return nil
}

// SignProposal requests a remote signer to sign a proposal
func (sc *SignerClient) SignProposal(chainID string, proposal *tmproto.Proposal) error {

	response, err := sc.endpoint.SendRequest(mustWrapMsg(&privvalproto.SignProposalRequest{Proposal: *proposal}))
	if err != nil {
		sc.endpoint.Logger.Error("SignerClient::SignProposal", "err", err)
		return err
	}

	resp := response.GetSignedProposalResponse()
	if resp == nil {
		sc.endpoint.Logger.Error("SignerClient::SignProposal", "err", "response != SignedProposalResponse")
		return ErrUnexpectedResponse
	}
	if resp.Error != nil {
		return &RemoteSignerError{Code: int(resp.Error.Code), Description: resp.Error.Description}
	}

	*proposal = *resp.Proposal

	return nil
}
