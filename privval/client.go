package privval

import (
	"context"
	"errors"
	"fmt"

	grpc "google.golang.org/grpc"

	"github.com/tendermint/tendermint/crypto"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/libs/log"
	privvalproto "github.com/tendermint/tendermint/proto/privval"
	tmproto "github.com/tendermint/tendermint/proto/types"
	"github.com/tendermint/tendermint/types"
)

// SignerClient implements PrivValidator.
// Handles remote validator connections that provide signing services
type SignerClient struct {
	ctx           context.Context
	cancel        context.CancelFunc
	privValidator privvalproto.PrivValidatorAPIClient
	conn          *grpc.ClientConn
	endpoint      string
	withCert      string
	options       []grpc.DialOption
	logger        log.Logger
}

var _ types.PrivValidator = (*SignerClient)(nil)

// NewSignerClient returns an instance of SignerClient.
// it will start the endpoint (if not already started)
func NewSignerClient(ctx context.Context, endpoint string,
	opts []grpc.DialOption, log log.Logger) (*SignerClient, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("target connection parameter missing. endpoint %s", endpoint)
	}

	if ctx == nil {
		ctx = context.Background()
	}

	ctx, cancel := context.WithCancel(ctx)

	//todo: add keepalive and retries

	conn, err := grpc.DialContext(ctx, endpoint, opts...)
	if err != nil {
		log.Error("unable to connect to client.", "target", endpoint, "err", err)
	}

	sc := &SignerClient{
		ctx:           ctx,
		cancel:        cancel,
		privValidator: privvalproto.NewPrivValidatorAPIClient(conn), // Create the Private Validator Client
		endpoint:      endpoint,
		logger:        log,
	}

	return sc, nil
}

// Close closes the underlying connection
func (sc *SignerClient) Close() error {
	sc.cancel()
	sc.logger.Info("Stopping service")
	if sc.conn != nil {
		return sc.conn.Close()
	}
	return nil
}

//--------------------------------------------------------
// Implement PrivValidator

// Ping sends a ping request to the remote signer
// todo: look at deprcating in favor of keepalive
// make sure it is supported in other languages (rust)
func (sc *SignerClient) Ping() error {
	_, err := sc.privValidator.Ping(sc.ctx, &privvalproto.PingRequest{})
	if err != nil {
		sc.logger.Error("SignerClient::Ping", "err", err)
		return nil
	}

	return nil
}

// GetPubKey retrieves a public key from a remote signer
// returns an error if client is not able to provide the key
func (sc *SignerClient) GetPubKey() (crypto.PubKey, error) {
	resp, err := sc.privValidator.GetPubKey(sc.ctx, &privvalproto.PubKeyRequest{})
	if err != nil {
		sc.logger.Error("SignerClient::GetPubKey", "err", err)
		return nil, fmt.Errorf("send: %w", err)
	}

	if resp.Error != nil {
		sc.logger.Error("failed to get private validator's public key", "err", resp.Error)
		return nil, fmt.Errorf("remote error: %w", errors.New(resp.Error.Description))
	}

	pk, err := cryptoenc.PubKeyFromProto(*resp.PubKey)
	if err != nil {
		return nil, err
	}

	return pk, nil
}

// SignVote requests a remote signer to sign a vote
func (sc *SignerClient) SignVote(chainID string, vote *tmproto.Vote) error {
	resp, err := sc.privValidator.SignVote(sc.ctx, &privvalproto.SignVoteRequest{ChainId: chainID, Vote: vote})
	if err != nil {
		sc.logger.Error("SignerClient::SignVote", "err", err)
		return err
	}

	if resp.Error != nil {
		return &RemoteSignerError{Code: int(resp.Error.Code), Description: resp.Error.Description}
	}

	*vote = *resp.Vote

	return nil
}

// SignProposal requests a remote signer to sign a proposal
func (sc *SignerClient) SignProposal(chainID string, proposal *tmproto.Proposal) error {
	resp, err := sc.privValidator.SignProposal(
		sc.ctx, &privvalproto.SignProposalRequest{ChainId: chainID, Proposal: proposal})
	if err != nil {
		sc.logger.Error("SignerClient::SignProposal", "err", err)
		return err
	}

	if resp.Error != nil {
		return &RemoteSignerError{Code: int(resp.Error.Code), Description: resp.Error.Description}
	}

	*proposal = *resp.Proposal

	return nil
}
