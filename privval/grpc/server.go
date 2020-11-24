package privval

import (
	context "context"
	"net"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/tendermint/tendermint/crypto"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/libs/service"
	privvalproto "github.com/tendermint/tendermint/proto/privval"
	"github.com/tendermint/tendermint/types"
)

type SignerServer struct {
	service.BaseService
	Logger log.Logger

	target  string
	ChainID string
	PrivVal types.PrivValidator
	Opts    []grpc.ServerOption
	Srv     *grpc.Server
}

func NewSignerServer(target string, chainID string, privVal types.PrivValidator, log log.Logger, opts []grpc.ServerOption) *SignerServer {
	return &SignerServer{
		Logger: log,

		target:  target,
		ChainID: chainID,
		Opts:    opts,
		PrivVal: privVal,
	}
}

// OnStart implements service.Service.
func (ss *SignerServer) OnStart() error {
	protocol, address := tmnet.ProtocolAndAddress(ss.target)
	lis, err := net.Listen(protocol, address)
	if err != nil {
		ss.Logger.Error("failed to listen: ", "err", err)
	}
	s := grpc.NewServer(ss.Opts...)
	ss.Srv = s

	privvalproto.RegisterPrivValidatorAPIServer(ss.Srv, &SignerServer{})

	if err := ss.Srv.Serve(lis); err != nil {
		ss.Logger.Error("failed to serve:", "err", err)
	}

	return nil
}

// OnStop implements service.Service.
func (ss *SignerServer) OnStop() {
	ss.Logger.Debug("SignerServer: OnStop calling Close")
	ss.Srv.GracefulStop()
}

var _ privvalproto.PrivValidatorAPIServer = (*SignerServer)(nil)

// PubKey receives a request for the pubkey
// returns the pubkey on success and error on failure
func (ss *SignerServer) GetPubKey(ctx context.Context, req *privvalproto.PubKeyRequest) (
	*privvalproto.PubKeyResponse, error) {
	var pubKey crypto.PubKey

	pubKey, err := ss.PrivVal.GetPubKey()
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "error getting pubkey: %v", err)
	}

	pk, err := cryptoenc.PubKeyToProto(pubKey)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "error transistioning pubkey to proto: %v", err)
	}

	return &privvalproto.PubKeyResponse{PubKey: &pk}, nil
}

// SignVote receives a vote sign requests, attempts to sign it
// returns SignedVoteResponse on success and error on failure
func (ss *SignerServer) SignVote(ctx context.Context, req *privvalproto.SignVoteRequest) (
	*privvalproto.SignedVoteResponse, error) {
	vote := req.Vote

	err := ss.PrivVal.SignVote(req.ChainId, vote)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "error signing vote: %v", err)
	}

	return &privvalproto.SignedVoteResponse{Vote: vote}, nil
}

// SignProposal receives a proposal sign requests, attempts to sign it
// returns SignedProposalResponse on success and error on failure
func (ss *SignerServer) SignProposal(ctx context.Context, req *privvalproto.SignProposalRequest) (
	*privvalproto.SignedProposalResponse, error) {
	proposal := req.Proposal

	err := ss.PrivVal.SignProposal(req.ChainId, proposal)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "error signing proposal: %v", err)
	}

	return &privvalproto.SignedProposalResponse{Proposal: proposal}, nil
}
