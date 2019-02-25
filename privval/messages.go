package privval

import (
	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/types"
)

// RemoteSignerMsg is sent between SignerServiceEndpoint and the SignerServiceEndpoint client.
type RemoteSignerMsg interface{}

func RegisterRemoteSignerMsg(cdc *amino.Codec) {
	cdc.RegisterInterface((*RemoteSignerMsg)(nil), nil)
	cdc.RegisterConcrete(&PubKeyRequest{}, "tendermint/remotesigner/PubKeyRequest", nil)
	cdc.RegisterConcrete(&PubKeyResponse{}, "tendermint/remotesigner/PubKeyResponse", nil)
	cdc.RegisterConcrete(&SignVoteRequest{}, "tendermint/remotesigner/SignVoteRequest", nil)
	cdc.RegisterConcrete(&SignedVoteResponse{}, "tendermint/remotesigner/SignedVoteResponse", nil)
	cdc.RegisterConcrete(&SignProposalRequest{}, "tendermint/remotesigner/SignProposalRequest", nil)
	cdc.RegisterConcrete(&SignedProposalResponse{}, "tendermint/remotesigner/SignedProposalResponse", nil)
	cdc.RegisterConcrete(&PingRequest{}, "tendermint/remotesigner/PingRequest", nil)
	cdc.RegisterConcrete(&PingResponse{}, "tendermint/remotesigner/PingResponse", nil)
}

// PubKeyRequest requests the consensus public key from the remote signer.
type PubKeyRequest struct{}

// PubKeyResponse is a PrivValidatorSocket message containing the public key.
type PubKeyResponse struct {
	PubKey crypto.PubKey
	Error  *RemoteSignerError
}

// SignVoteRequest is a PrivValidatorSocket message containing a vote.
type SignVoteRequest struct {
	Vote *types.Vote
}

// SignedVoteResponse is a PrivValidatorSocket message containing a signed vote along with a potenial error message.
type SignedVoteResponse struct {
	Vote  *types.Vote
	Error *RemoteSignerError
}

// SignProposalRequest is a PrivValidatorSocket message containing a Proposal.
type SignProposalRequest struct {
	Proposal *types.Proposal
}

// SignedProposalResponse is a PrivValidatorSocket message containing a proposal response
type SignedProposalResponse struct {
	Proposal *types.Proposal
	Error    *RemoteSignerError
}

// PingRequest is a PrivValidatorSocket message to keep the connection alive.
type PingRequest struct {
}

// PingRequest is a PrivValidatorSocket response to keep the connection alive.
type PingResponse struct {
}
