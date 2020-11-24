package privval

import (
	"context"
	"fmt"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/status"

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
	privValidator privvalproto.PrivValidatorAPIClient
	conn          *grpc.ClientConn
	logger        log.Logger
}

var _ types.PrivValidator = (*SignerClient)(nil)

// NewSignerClient returns an instance of SignerClient.
// it will start the endpoint (if not already started)
func NewSignerClient(target string,
	opts []grpc.DialOption, log log.Logger) (*SignerClient, error) {
	if target == "" {
		return nil, fmt.Errorf("target connection parameter missing. endpoint %s", target)
	}

	ctx := context.Background()

	conn, err := grpc.DialContext(ctx, target, opts...)
	if err != nil {
		log.Error("unable to connect to client.", "target", target, "err", err)
	}

	sc := &SignerClient{
		ctx:           ctx,
		privValidator: privvalproto.NewPrivValidatorAPIClient(conn), // Create the Private Validator Client
		logger:        log,
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
func (sc *SignerClient) GetPubKey() (crypto.PubKey, error) {
	resp, err := sc.privValidator.GetPubKey(sc.ctx, &privvalproto.PubKeyRequest{})
	if err != nil {
		errStatus, _ := status.FromError(err)
		sc.logger.Error("SignerClient::GetPubKey", "err", errStatus.Message())
		return nil, fmt.Errorf("send GetPubKey request: %w", errStatus.Err())
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
		errStatus, _ := status.FromError(err)
		sc.logger.Error("Client SignVote", "err", errStatus.Message())
		return fmt.Errorf("send SignVote request: %w", errStatus.Err())
	}

	*vote = *resp.Vote

	return nil
}

// SignProposal requests a remote signer to sign a proposal
func (sc *SignerClient) SignProposal(chainID string, proposal *tmproto.Proposal) error {
	resp, err := sc.privValidator.SignProposal(
		sc.ctx, &privvalproto.SignProposalRequest{ChainId: chainID, Proposal: proposal})

	if err != nil {
		errStatus, _ := status.FromError(err)
		sc.logger.Error("SignerClient::SignProposal", "err", errStatus.Message())
		return fmt.Errorf("send SignProposal request: %w", errStatus.Err())
	}

	*proposal = *resp.Proposal

	return nil
}
