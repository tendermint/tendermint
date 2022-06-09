package grpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/dashevo/dashd-go/btcjson"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/encoding"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
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

func (sc *SignerClient) ExtractIntoValidator(ctx context.Context, quorumHash crypto.QuorumHash) *types.Validator {
	pubKey, _ := sc.GetPubKey(ctx, quorumHash)
	proTxHash, _ := sc.GetProTxHash(ctx)
	if len(proTxHash) != crypto.DefaultHashSize {
		panic("proTxHash wrong length")
	}
	return &types.Validator{
		PubKey:      pubKey,
		VotingPower: types.DefaultDashVotingPower,
		ProTxHash:   proTxHash,
	}
}

// GetPubKey retrieves a public key from a remote signer
// returns an error if client is not able to provide the key
func (sc *SignerClient) GetPubKey(ctx context.Context, quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	if len(quorumHash.Bytes()) != crypto.DefaultHashSize {
		return nil, fmt.Errorf("quorum hash must be 32 bytes long if requesting public key from dash core")
	}
	resp, err := sc.client.GetPubKey(ctx, &privvalproto.PubKeyRequest{ChainId: sc.chainID, QuorumHash: quorumHash})
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

func (sc *SignerClient) GetProTxHash(ctx context.Context) (crypto.ProTxHash, error) {
	resp, err := sc.client.GetProTxHash(ctx, &privvalproto.ProTxHashRequest{ChainId: sc.chainID})
	if err != nil {
		errStatus, _ := status.FromError(err)
		sc.logger.Error("SignerClient::GetProTxHash", "err", errStatus.Message())
		return nil, errStatus.Err()
	}
	if len(resp.ProTxHash) != 32 {
		return nil, fmt.Errorf("proTxHash is invalid size")
	}

	return resp.ProTxHash, nil
}

func (sc *SignerClient) GetFirstQuorumHash(ctx context.Context) (crypto.QuorumHash, error) {
	return nil, errors.New("getFirstQuorumHash should not be called on a signer client")
}

func (sc *SignerClient) GetThresholdPublicKey(ctx context.Context, quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	if len(quorumHash.Bytes()) != crypto.DefaultHashSize {
		return nil, fmt.Errorf("quorum hash must be 32 bytes long if requesting public key from dash core")
	}

	resp, err := sc.client.GetThresholdPubKey(ctx, &privvalproto.ThresholdPubKeyRequest{ChainId: sc.chainID, QuorumHash: quorumHash})
	if err != nil {
		errStatus, _ := status.FromError(err)
		sc.logger.Error("SignerClient::GetThresholdPubKey", "err", errStatus.Message())
		return nil, errStatus.Err()
	}
	pk, err := encoding.PubKeyFromProto(resp.PubKey)
	if err != nil {
		return nil, err
	}

	return pk, nil
}
func (sc *SignerClient) GetHeight(ctx context.Context, quorumHash crypto.QuorumHash) (int64, error) {
	return 0, fmt.Errorf("getHeight should not be called on asigner client %s", quorumHash.String())
}

// SignVote requests a remote signer to sign a vote
func (sc *SignerClient) SignVote(
	ctx context.Context, chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash,
	vote *tmproto.Vote, stateID types.StateID, logger log.Logger) error {
	if len(quorumHash.Bytes()) != crypto.DefaultHashSize {
		return fmt.Errorf("quorum hash must be 32 bytes long when signing vote")
	}
	protoStateID := stateID.ToProto()
	resp, err := sc.client.SignVote(ctx, &privvalproto.SignVoteRequest{ChainId: sc.chainID, Vote: vote,
		QuorumType: int32(quorumType), QuorumHash: quorumHash, StateId: &protoStateID})
	if err != nil {
		errStatus, _ := status.FromError(err)
		sc.logger.Error("Client SignVote", "err", errStatus.Message())
		return errStatus.Err()
	}

	*vote = resp.Vote

	return nil
}

// SignProposal requests a remote signer to sign a proposal
func (sc *SignerClient) SignProposal(
	ctx context.Context, chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, proposal *tmproto.Proposal,
) (tmbytes.HexBytes, error) {
	resp, err := sc.client.SignProposal(
		ctx, &privvalproto.SignProposalRequest{ChainId: chainID, Proposal: proposal,
			QuorumType: int32(quorumType), QuorumHash: quorumHash})

	if err != nil {
		errStatus, _ := status.FromError(err)
		sc.logger.Error("SignerClient::SignProposal", "err", errStatus.Message())
		return nil, errStatus.Err()
	}

	*proposal = resp.Proposal

	// We can assume that the signer client calculated the signID correctly
	blockSignID := types.ProposalBlockSignID(chainID, proposal, quorumType, quorumHash)

	return blockSignID, nil
}

func (sc *SignerClient) UpdatePrivateKey(
	ctx context.Context, privateKey crypto.PrivKey, quorumHash crypto.QuorumHash, thresholdPublicKey crypto.PubKey, height int64,
) {

}

func (sc *SignerClient) GetPrivateKey(ctx context.Context, quorumHash crypto.QuorumHash) (crypto.PrivKey, error) {
	return nil, nil
}
