package grpc

import (
	"context"

	"github.com/dashevo/dashd-go/btcjson"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/libs/log"
	privvalproto "github.com/tendermint/tendermint/proto/tendermint/privval"
	"github.com/tendermint/tendermint/types"
)

// SignerServer implements PrivValidatorAPIServer 9generated via protobuf services)
// Handles remote validator connections that provide signing services
type SignerServer struct {
	logger  log.Logger
	chainID string
	privVal types.PrivValidator
}

func NewSignerServer(logger log.Logger, chainID string, privVal types.PrivValidator) *SignerServer {
	return &SignerServer{
		logger:  logger,
		chainID: chainID,
		privVal: privVal,
	}
}

var _ privvalproto.PrivValidatorAPIServer = (*SignerServer)(nil)

// GetPubKey receives a request for the pubkey
// returns the pubkey on success and error on failure
func (ss *SignerServer) GetPubKey(ctx context.Context, req *privvalproto.PubKeyRequest) (
	*privvalproto.PubKeyResponse, error) {
	var pubKey crypto.PubKey

	pubKey, err := ss.privVal.GetPubKey(ctx, req.QuorumHash)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "error getting pubkey: %v", err)
	}

	pk, err := encoding.PubKeyToProto(pubKey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error transitioning pubkey to proto: %v", err)
	}

	ss.logger.Info("SignerServer: GetPubKey Success")

	return &privvalproto.PubKeyResponse{PubKey: pk}, nil
}

// GetThresholdPubKey receives a request for the threshold pubkey
// returns the pubkey on success and error on failure
func (ss *SignerServer) GetThresholdPubKey(ctx context.Context, req *privvalproto.ThresholdPubKeyRequest) (
	*privvalproto.ThresholdPubKeyResponse, error) {
	var pubKey crypto.PubKey

	pubKey, err := ss.privVal.GetThresholdPublicKey(ctx, req.QuorumHash)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "error getting pubkey: %v", err)
	}

	pk, err := encoding.PubKeyToProto(pubKey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error transitioning pubkey to proto: %v", err)
	}

	ss.logger.Info("SignerServer: GetPubKey Success")

	return &privvalproto.ThresholdPubKeyResponse{PubKey: pk}, nil
}

// GetProTxHash receives a request for the proTxHash
// returns the proTxHash on success and error on failure
func (ss *SignerServer) GetProTxHash(ctx context.Context, req *privvalproto.ProTxHashRequest) (
	*privvalproto.ProTxHashResponse, error) {
	var proTxHash crypto.ProTxHash

	proTxHash, err := ss.privVal.GetProTxHash(ctx)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "error getting proTxHash: %v", err)
	}

	ss.logger.Info("SignerServer: GetProTxHash Success")

	return &privvalproto.ProTxHashResponse{ProTxHash: proTxHash}, nil
}

// SignVote receives a vote sign requests, attempts to sign it
// returns SignedVoteResponse on success and error on failure
func (ss *SignerServer) SignVote(ctx context.Context, req *privvalproto.SignVoteRequest) (*privvalproto.SignedVoteResponse, error) {
	vote := req.Vote

	stateID, err := types.StateIDFromProto(req.StateId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "error converting stateId when signing vote: %v", err)
	}
	err = ss.privVal.SignVote(ctx, req.ChainId, btcjson.LLMQType(req.QuorumType), req.QuorumHash, vote, *stateID, nil)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "error signing vote: %v", err)
	}

	ss.logger.Info("SignerServer: SignVote Success", "height", req.Vote.Height)

	return &privvalproto.SignedVoteResponse{Vote: *vote}, nil
}

// SignProposal receives a proposal sign requests, attempts to sign it
// returns SignedProposalResponse on success and error on failure
func (ss *SignerServer) SignProposal(ctx context.Context, req *privvalproto.SignProposalRequest) (*privvalproto.SignedProposalResponse, error) {
	proposal := req.Proposal

	_, err := ss.privVal.SignProposal(ctx, req.ChainId, btcjson.LLMQType(req.QuorumType), req.QuorumHash, proposal)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "error signing proposal: %v", err)
	}

	ss.logger.Info("SignerServer: SignProposal Success", "height", req.Proposal.Height)

	return &privvalproto.SignedProposalResponse{Proposal: *proposal}, nil
}
