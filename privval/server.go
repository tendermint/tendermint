package privval

import (
	context "context"

	"github.com/tendermint/tendermint/crypto"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	privvalproto "github.com/tendermint/tendermint/proto/privval"
	"github.com/tendermint/tendermint/types"
)

type SignerServer struct {
	chainID string
	privVal types.PrivValidator
}

var _ privvalproto.PrivValidatorAPIServer = (*SignerServer)(nil)

func NewSignerServer(chainID string, privVal types.PrivValidator) *SignerServer {
	return &SignerServer{
		chainID: chainID,
		privVal: privVal,
	}
}

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
func (ss *SignerServer) SignVote(_ context.Context, req *privvalproto.SignVoteRequest) (
	*privvalproto.SignedVoteResponse, error) {
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
func (ss *SignerServer) SignProposal(_ context.Context, req *privvalproto.SignProposalRequest) (
	*privvalproto.SignedProposalResponse, error) {
	proposal := req.Proposal

	err := ss.privVal.SignProposal(req.ChainId, proposal)
	if err != nil {
		return &privvalproto.SignedProposalResponse{
			Proposal: nil, Error: &privvalproto.RemoteSignerError{Code: 0, Description: err.Error()}}, err
	}

	return &privvalproto.SignedProposalResponse{Proposal: proposal, Error: nil}, nil
}
