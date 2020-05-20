package privval

import (
	context "context"
	"net"

	"github.com/tendermint/tendermint/crypto"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/libs/service"
	privvalproto "github.com/tendermint/tendermint/proto/privval"
	"github.com/tendermint/tendermint/types"
)

type SignerServer struct {
	service.BaseService
	listener net.Listener
	chainID  string
	privVal  types.PrivValidator
}

func NewSignerServer(endpoint net.Listener, chainID string, privVal types.PrivValidator) *SignerServer {
	ss := &SignerServer{
		listener: endpoint,
		chainID:  chainID,
		privVal:  privVal,
	}

	// ss.BaseService = *service.NewBaseService(endpoint.Logger, "SignerServer", ss)

	return ss
}

// // OnStart implements service.Service.
// func (ss *SignerServer) OnStart() error {
// 	go ss.serviceLoop()
// 	return nil
// }

// // OnStop implements service.Service.
// func (ss *SignerServer) OnStop() {
// 	ss.endpoint.Logger.Debug("SignerServer: OnStop calling Close")
// 	_ = ss.endpoint.Close()
// }

// Ping recieves a ping request
// returns the ping response
func (ss *SignerServer) Ping(ctx context.Context) (*privvalproto.PingResponse, error) {
	return &privvalproto.PingResponse{}, nil
}

// PubKey recieves a request for the pubkey
// returns the pubkey on success and error on failure
func (ss *SignerServer) PubKey(ctx context.Context) (*privvalproto.PubKeyResponse, error) {
	var pubKey crypto.PubKey

	pubKey, err := ss.privVal.GetPubKey()
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
func (ss *SignerServer) SignVote(ctx context.Context, req privvalproto.SignVoteRequest) (*privvalproto.SignedVoteResponse, error) {
	vote := req.Vote

	err := ss.privVal.SignVote(req.ChainId, vote)
	if err != nil {
		return &privvalproto.SignedVoteResponse{
			Vote: nil, Error: &privvalproto.RemoteSignerError{Code: 0, Description: err.Error()}}, err
	}

	return &privvalproto.SignedVoteResponse{Vote: vote, Error: nil}, nil
}

// SignProposal receives a proposal sign requests, attempts to sign it
// returns SignedProposalResponse on success and error on failure
func (ss *SignerServer) SignProposal(ctx context.Context, req privvalproto.SignProposalRequest) (*privvalproto.SignedProposalResponse, error) {
	proposal := req.Proposal

	err := ss.privVal.SignProposal(req.ChainId, proposal)
	if err != nil {
		return &privvalproto.SignedProposalResponse{
			Proposal: nil, Error: &privvalproto.RemoteSignerError{Code: 0, Description: err.Error()}}, err
	}

	return &privvalproto.SignedProposalResponse{Proposal: proposal, Error: nil}, nil
}
