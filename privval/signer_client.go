package privval

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/crypto"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	privvalproto "github.com/tendermint/tendermint/proto/privval"
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
			return nil, errors.Wrap(err, "failed to start listener endpoint")
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
	response, err := sc.endpoint.SendRequest(&privvalproto.PingRequest{})

	if err != nil {
		sc.endpoint.Logger.Error("SignerClient::Ping", "err", err)
		return nil
	}

	_, ok := response.(*privvalproto.PingResponse)
	if !ok {
		sc.endpoint.Logger.Error("SignerClient::Ping", "err", "response != PingResponse")
		return err
	}

	return nil
}

// GetPubKey retrieves a public key from a remote signer
// returns an error if client is not able to provide the key
func (sc *SignerClient) GetPubKey() (crypto.PubKey, error) {
	response, err := sc.endpoint.SendRequest(&privvalproto.PubKeyRequest{})
	if err != nil {
		sc.endpoint.Logger.Error("SignerClient::GetPubKey", "err", err)
		return nil, errors.Wrap(err, "send")
	}

	pubKeyResp, ok := response.(*privvalproto.PubKeyResponse)
	if !ok {
		sc.endpoint.Logger.Error("SignerClient::GetPubKey", "err", "response != PubKeyResponse")
		return nil, errors.Errorf("unexpected response type %T", response)
	}

	if pubKeyResp.Error != nil || pubKeyResp.PubKey == nil {
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
func (sc *SignerClient) SignVote(chainID string, vote *types.Vote) error {
	pbv := vote.ToProto()

	response, err := sc.endpoint.SendRequest(&privvalproto.SignVoteRequest{Vote: pbv})
	if err != nil {
		sc.endpoint.Logger.Error("SignerClient::SignVote", "err", err)
		return err
	}

	resp, ok := response.(*privvalproto.SignedVoteResponse)
	if !ok {
		sc.endpoint.Logger.Error("SignerClient::GetPubKey", "err", "response != SignedVoteResponse")
		return ErrUnexpectedResponse
	}

	if resp.Error != nil {
		return fmt.Errorf("%s", resp.Error.Description)
	}
	v, err := types.VoteFromProto(resp.Vote)
	if err != nil {
		return err
	}
	*vote = *v

	return nil
}

// SignProposal requests a remote signer to sign a proposal
func (sc *SignerClient) SignProposal(chainID string, proposal *types.Proposal) error {
	pb := proposal.ToProto()
	response, err := sc.endpoint.SendRequest(&privvalproto.SignProposalRequest{Proposal: *pb})
	if err != nil {
		sc.endpoint.Logger.Error("SignerClient::SignProposal", "err", err)
		return err
	}

	resp, ok := response.(*privvalproto.SignedProposalResponse)
	if !ok {
		sc.endpoint.Logger.Error("SignerClient::SignProposal", "err", "response != SignedProposalResponse")
		return ErrUnexpectedResponse
	}
	if resp.Error != nil {
		return fmt.Errorf("%s", resp.Error.Description)
	}
	p, err := types.ProposalFromProto(resp.Proposal)
	if err != nil {
		return err
	}
	*proposal = *p

	return nil
}
