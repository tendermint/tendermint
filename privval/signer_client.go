package privval

import (
	"context"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/libs/log"
	privvalproto "github.com/tendermint/tendermint/proto/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

// SignerClient implements PrivValidator.
// Handles remote validator connections that provide signing services
type SignerClient struct {
	logger   log.Logger
	endpoint *SignerListenerEndpoint
	chainID  string
}

var _ types.PrivValidator = (*SignerClient)(nil)

// NewSignerClient returns an instance of SignerClient.
// it will start the endpoint (if not already started)
func NewSignerClient(ctx context.Context, endpoint *SignerListenerEndpoint, chainID string) (*SignerClient, error) {
	if !endpoint.IsRunning() {
		if err := endpoint.Start(ctx); err != nil {
			return nil, fmt.Errorf("failed to start listener endpoint: %w", err)
		}
	}

	return &SignerClient{
		logger:   endpoint.logger,
		endpoint: endpoint,
		chainID:  chainID,
	}, nil
}

// Close closes the underlying connection
func (sc *SignerClient) Close() error {
	sc.endpoint.Stop()
	err := sc.endpoint.Close()
	if err != nil {
		return err
	}
	return nil
}

// IsConnected indicates with the signer is connected to a remote signing service
func (sc *SignerClient) IsConnected() bool {
	return sc.endpoint.IsConnected()
}

// WaitForConnection waits maxWait for a connection or returns a timeout error
func (sc *SignerClient) WaitForConnection(ctx context.Context, maxWait time.Duration) error {
	return sc.endpoint.WaitForConnection(ctx, maxWait)
}

//--------------------------------------------------------
// Implement PrivValidator

// Ping sends a ping request to the remote signer
func (sc *SignerClient) Ping(ctx context.Context) error {
	response, err := sc.endpoint.SendRequest(ctx, mustWrapMsg(&privvalproto.PingRequest{}))
	if err != nil {
		sc.logger.Error("SignerClient::Ping", "err", err)
		return nil
	}

	pb := response.GetPingResponse()
	if pb == nil {
		return err
	}

	return nil
}

// GetPubKey retrieves a public key from a remote signer
// returns an error if client is not able to provide the key
func (sc *SignerClient) GetPubKey(ctx context.Context) (crypto.PubKey, error) {
	response, err := sc.endpoint.SendRequest(ctx, mustWrapMsg(&privvalproto.PubKeyRequest{ChainId: sc.chainID}))
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	resp := response.GetPubKeyResponse()
	if resp == nil {
		return nil, ErrUnexpectedResponse
	}
	if resp.Error != nil {
		return nil, &RemoteSignerError{Code: int(resp.Error.Code), Description: resp.Error.Description}
	}

	pk, err := encoding.PubKeyFromProto(resp.PubKey)
	if err != nil {
		return nil, err
	}

	return pk, nil
}

// SignVote requests a remote signer to sign a vote
func (sc *SignerClient) SignVote(ctx context.Context, chainID string, vote *tmproto.Vote) error {
	response, err := sc.endpoint.SendRequest(ctx, mustWrapMsg(&privvalproto.SignVoteRequest{Vote: vote, ChainId: chainID}))
	if err != nil {
		return err
	}

	resp := response.GetSignedVoteResponse()
	if resp == nil {
		return ErrUnexpectedResponse
	}
	if resp.Error != nil {
		return &RemoteSignerError{Code: int(resp.Error.Code), Description: resp.Error.Description}
	}

	*vote = resp.Vote

	return nil
}

// SignProposal requests a remote signer to sign a proposal
func (sc *SignerClient) SignProposal(ctx context.Context, chainID string, proposal *tmproto.Proposal) error {
	response, err := sc.endpoint.SendRequest(ctx, mustWrapMsg(
		&privvalproto.SignProposalRequest{Proposal: proposal, ChainId: chainID},
	))
	if err != nil {
		return err
	}

	resp := response.GetSignedProposalResponse()
	if resp == nil {
		return ErrUnexpectedResponse
	}
	if resp.Error != nil {
		return &RemoteSignerError{Code: int(resp.Error.Code), Description: resp.Error.Description}
	}

	*proposal = resp.Proposal

	return nil
}
