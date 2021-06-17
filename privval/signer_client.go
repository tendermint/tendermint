package privval

import (
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	"time"

	"github.com/tendermint/tendermint/crypto"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	privvalproto "github.com/tendermint/tendermint/proto/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

// SignerClient implements PrivValidator.
// Handles remote validator connections that provide signing services
type SignerClient struct {
	endpoint *SignerListenerEndpoint
	chainID  string
}

var _ types.PrivValidator = (*SignerClient)(nil)

// NewSignerClient returns an instance of SignerClient.
// it will start the endpoint (if not already started)
func NewSignerClient(endpoint *SignerListenerEndpoint, chainID string) (*SignerClient, error) {
	if !endpoint.IsRunning() {
		if err := endpoint.Start(); err != nil {
			return nil, fmt.Errorf("failed to start listener endpoint: %w", err)
		}
	}

	return &SignerClient{endpoint: endpoint, chainID: chainID}, nil
}

// Close closes the underlying connection
func (sc *SignerClient) Close() error {
	return sc.endpoint.Close()
}

// IsConnected indicates with the signer is connected to a remote signing service
func (sc *SignerClient) IsConnected() bool {
	return sc.endpoint.IsConnected()
}

// WaitForConnection waits maxWait for a connection or returns a timeout error
func (sc *SignerClient) WaitForConnection(maxWait time.Duration) error {
	return sc.endpoint.WaitForConnection(maxWait)
}

//--------------------------------------------------------
// Implement PrivValidator

// Ping sends a ping request to the remote signer
func (sc *SignerClient) Ping() error {
	response, err := sc.endpoint.SendRequest(mustWrapMsg(&privvalproto.PingRequest{}))
	if err != nil {
		sc.endpoint.Logger.Error("SignerClient::Ping", "err", err)
		return nil
	}

	pb := response.GetPingResponse()
	if pb == nil {
		return err
	}

	return nil
}

func (sc *SignerClient) ExtractIntoValidator(height int64, quorumHash crypto.QuorumHash) *types.Validator {
	pubKey, _ := sc.GetPubKey(quorumHash)
	proTxHash, _ := sc.GetProTxHash()
	if len(proTxHash) != crypto.DefaultHashSize {
		panic("proTxHash wrong length")
	}
	return &types.Validator{
		Address:     pubKey.Address(),
		PubKey:      pubKey,
		VotingPower: types.DefaultDashVotingPower,
		ProTxHash:   proTxHash,
	}
}

// GetPubKey retrieves a public key from a remote signer
// returns an error if client is not able to provide the key
func (sc *SignerClient) GetPubKey(quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	response, err := sc.endpoint.SendRequest(mustWrapMsg(&privvalproto.PubKeyRequest{ChainId: sc.chainID}))
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	resp := response.GetPubKeyResponse()
	if resp == nil {
		return nil, ErrUnexpectedResponse
	}
	if resp.Error != nil {
		return nil, &RemoteSignerError{Code: int(resp.Error.Code), Description: resp.Error.Description}
	}

	pk, err := cryptoenc.PubKeyFromProto(resp.PubKey)
	if err != nil {
		return nil, err
	}

	return pk, nil
}

func (sc *SignerClient) GetProTxHash() (crypto.ProTxHash, error) {
	response, err := sc.endpoint.SendRequest(mustWrapMsg(&privvalproto.ProTxHashRequest{ChainId: sc.chainID}))
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	resp := response.GetProTxHashResponse()
	if resp == nil {
		return nil, ErrUnexpectedResponse
	}
	if resp.Error != nil {
		return nil, &RemoteSignerError{Code: int(resp.Error.Code), Description: resp.Error.Description}
	}

	if len(resp.ProTxHash) != 32 {
		return nil, fmt.Errorf("proTxHash is invalid size")
	}

	return resp.ProTxHash, nil
}

// SignVote requests a remote signer to sign a vote
func (sc *SignerClient) SignVote(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, vote *tmproto.Vote) error {
	// fmt.Printf("--> sending request to sign vote (%d/%d) %v - %v", vote.Height, vote.Round, vote.BlockID, vote)
	response, err := sc.endpoint.SendRequest(mustWrapMsg(&privvalproto.SignVoteRequest{Vote: vote, ChainId: chainID,
		QuorumType: int32(quorumType), QuorumHash: quorumHash}))
	if err != nil {
		return err
	}

	resp := response.GetSignedVoteResponse()
	if resp == nil {
		return ErrUnexpectedResponse
	}
	if resp.Error != nil {
		return &RemoteSignerError{Code: int(resp.Error.Code), Description: resp.Error.Description}
	}

	*vote = resp.Vote

	return nil
}

// SignProposal requests a remote signer to sign a proposal
func (sc *SignerClient) SignProposal(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, proposal *tmproto.Proposal) error {
	response, err := sc.endpoint.SendRequest(mustWrapMsg(
		&privvalproto.SignProposalRequest{Proposal: proposal, ChainId: chainID,
			QuorumType: int32(quorumType), QuorumHash: quorumHash},
	))
	if err != nil {
		return err
	}

	resp := response.GetSignedProposalResponse()
	if resp == nil {
		return ErrUnexpectedResponse
	}
	if resp.Error != nil {
		return &RemoteSignerError{Code: int(resp.Error.Code), Description: resp.Error.Description}
	}

	*proposal = resp.Proposal

	return nil
}

func (sc *SignerClient) UpdatePrivateKey(privateKey crypto.PrivKey, height int64) error {
	// the private key is dealt with on the abci client
	return nil
}
