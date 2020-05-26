package privval

import (
	context "context"
	"net"

	grpc "google.golang.org/grpc"

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

	ListenAddr string
	ChainID    string
	PrivVal    types.PrivValidator
	Srv        *grpc.Server
}

func NewSignerServer(listenAddr string, chainID string, privVal types.PrivValidator,
	srv *grpc.Server, log log.Logger) *SignerServer {
	return &SignerServer{
		Logger: log,

		ListenAddr: listenAddr,
		ChainID:    chainID,
		PrivVal:    privVal,
		Srv:        srv,
	}
}

// OnStart implements service.Service.
func (ss *SignerServer) OnStart() error {
	protocol, address := tmnet.ProtocolAndAddress(ss.ListenAddr)
	lis, err := net.Listen(protocol, address) //todo:
	if err != nil {
		ss.Logger.Error("failed to listen: ", "err", err)
	}

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

// Ping receives a ping request
// returns the ping response
func (ss *SignerServer) Ping(_ context.Context, req *privvalproto.PingRequest) (*privvalproto.PingResponse, error) {
	return &privvalproto.PingResponse{}, nil
}

// PubKey receives a request for the pubkey
// returns the pubkey on success and error on failure
func (ss *SignerServer) GetPubKey(_ context.Context, req *privvalproto.PubKeyRequest) (
	*privvalproto.PubKeyResponse, error) {
	var pubKey crypto.PubKey

	pubKey, err := ss.PrivVal.GetPubKey()
	if err != nil {
		return &privvalproto.PubKeyResponse{
			PubKey: nil, Error: &privvalproto.RemoteSignerError{Code: 0, Description: err.Error()}}, err
	}

	pk, err := cryptoenc.PubKeyToProto(pubKey)
	if err != nil {
		return &privvalproto.PubKeyResponse{
			PubKey: nil,
			Error:  &privvalproto.RemoteSignerError{Code: 0, Description: err.Error()}}, err
	}

	return &privvalproto.PubKeyResponse{PubKey: &pk, Error: nil}, nil
}

// SignVote receives a vote sign requests, attempts to sign it
// returns SignedVoteResponse on success and error on failure
func (ss *SignerServer) SignVote(_ context.Context, req *privvalproto.SignVoteRequest) (
	*privvalproto.SignedVoteResponse, error) {
	vote := req.Vote

	err := ss.PrivVal.SignVote(req.ChainId, vote)
	if err != nil {
		return &privvalproto.SignedVoteResponse{
			Vote: nil, Error: &privvalproto.RemoteSignerError{Code: 0, Description: err.Error()}}, err
	}

	return &privvalproto.SignedVoteResponse{Vote: vote, Error: nil}, nil
}

// SignProposal receives a proposal sign requests, attempts to sign it
// returns SignedProposalResponse on success and error on failure
func (ss *SignerServer) SignProposal(_ context.Context, req *privvalproto.SignProposalRequest) (
	*privvalproto.SignedProposalResponse, error) {
	proposal := req.Proposal

	err := ss.PrivVal.SignProposal(req.ChainId, proposal)
	if err != nil {
		return &privvalproto.SignedProposalResponse{
			Proposal: nil, Error: &privvalproto.RemoteSignerError{Code: 0, Description: err.Error()}}, err
	}

	return &privvalproto.SignedProposalResponse{Proposal: proposal, Error: nil}, nil
}
