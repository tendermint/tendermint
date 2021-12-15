package grpc

import (
	"context"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/status"

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
	logger log.Logger

	client  privvalproto.PrivValidatorAPIClient
	conn    *grpc.ClientConn
	chainID string
}

var _ types.PrivValidator = (*SignerClient)(nil)

// NewSignerClient returns an instance of SignerClient.
// it will start the endpoint (if not already started)
func NewSignerClient(conn *grpc.ClientConn,
	chainID string, log log.Logger) (*SignerClient, error) {

	sc := &SignerClient{
		logger:  log,
		chainID: chainID,
		client:  privvalproto.NewPrivValidatorAPIClient(conn), // Create the Private Validator Client
	}

	return sc, nil
}

// Close closes the underlying connection
func (sc *SignerClient) Close() error {
	sc.logger.Info("Stopping service")
	if sc.conn != nil {
		return sc.conn.Close()
	}
	return nil
}

//--------------------------------------------------------
// Implement PrivValidator

// GetPubKey retrieves a public key from a remote signer
// returns an error if client is not able to provide the key
func (sc *SignerClient) GetPubKey(ctx context.Context) (crypto.PubKey, error) {
	resp, err := sc.client.GetPubKey(ctx, &privvalproto.PubKeyRequest{ChainId: sc.chainID})
	if err != nil {
		errStatus, _ := status.FromError(err)
		sc.logger.Error("SignerClient::GetPubKey", "err", errStatus.Message())
		return nil, errStatus.Err()
	}

	pk, err := encoding.PubKeyFromProto(resp.PubKey)
	if err != nil {
		return nil, err
	}

	return pk, nil
}

// SignVote requests a remote signer to sign a vote
func (sc *SignerClient) SignVote(ctx context.Context, chainID string, vote *tmproto.Vote) error {
	resp, err := sc.client.SignVote(ctx, &privvalproto.SignVoteRequest{ChainId: sc.chainID, Vote: vote})
	if err != nil {
		errStatus, _ := status.FromError(err)
		sc.logger.Error("Client SignVote", "err", errStatus.Message())
		return errStatus.Err()
	}

	*vote = resp.Vote

	return nil
}

// SignProposal requests a remote signer to sign a proposal
func (sc *SignerClient) SignProposal(ctx context.Context, chainID string, proposal *tmproto.Proposal) error {
	resp, err := sc.client.SignProposal(
		ctx, &privvalproto.SignProposalRequest{ChainId: chainID, Proposal: proposal})

	if err != nil {
		errStatus, _ := status.FromError(err)
		sc.logger.Error("SignerClient::SignProposal", "err", errStatus.Message())
		return errStatus.Err()
	}

	*proposal = resp.Proposal

	return nil
}
