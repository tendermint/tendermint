package grpc

import (
	context "context"

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

// PubKey receives a request for the pubkey
// returns the pubkey on success and error on failure
func (ss *SignerServer) GetPubKey(ctx context.Context, req *privvalproto.PubKeyRequest) (
	*privvalproto.PubKeyResponse, error) {
	var pubKey crypto.PubKey

	pubKey, err := ss.privVal.GetPubKey(ctx)
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

// SignVote receives a vote sign requests, attempts to sign it
// returns SignedVoteResponse on success and error on failure
func (ss *SignerServer) SignVote(ctx context.Context, req *privvalproto.SignVoteRequest) (*privvalproto.SignedVoteResponse, error) {
	vote := req.Vote

	err := ss.privVal.SignVote(ctx, req.ChainId, vote)
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

	err := ss.privVal.SignProposal(ctx, req.ChainId, proposal)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "error signing proposal: %v", err)
	}

	ss.logger.Info("SignerServer: SignProposal Success", "height", req.Proposal.Height)

	return &privvalproto.SignedProposalResponse{Proposal: *proposal}, nil
}
