package privval

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dashevo/dashd-go/btcjson"

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
	logger   log.Logger
	endpoint *SignerListenerEndpoint
	chainID  string
}

var _ types.PrivValidator = (*SignerClient)(nil)

// NewSignerClient returns an instance of SignerClient.
// it will start the endpoint (if not already started)
func NewSignerClient(ctx context.Context, endpoint *SignerListenerEndpoint, chainID string) (*SignerClient, error) {
	if !endpoint.IsRunning() {
		if err := endpoint.Start(ctx); err != nil {
			return nil, fmt.Errorf("failed to start listener endpoint: %w", err)
		}
	}

	return &SignerClient{
		logger:   endpoint.logger,
		endpoint: endpoint,
		chainID:  chainID,
	}, nil
}

// Close closes the underlying connection
func (sc *SignerClient) Close() error {
	sc.endpoint.Stop()
	err := sc.endpoint.Close()
	if err != nil {
		return err
	}
	return nil
}

// IsConnected indicates with the signer is connected to a remote signing service
func (sc *SignerClient) IsConnected() bool {
	return sc.endpoint.IsConnected()
}

// WaitForConnection waits maxWait for a connection or returns a timeout error
func (sc *SignerClient) WaitForConnection(ctx context.Context, maxWait time.Duration) error {
	return sc.endpoint.WaitForConnection(ctx, maxWait)
}

//--------------------------------------------------------
// Implement PrivValidator

// Ping sends a ping request to the remote signer
func (sc *SignerClient) Ping(ctx context.Context) error {
	response, err := sc.endpoint.SendRequest(ctx, mustWrapMsg(&privvalproto.PingRequest{}))
	if err != nil {
		sc.logger.Error("SignerClient::Ping", "err", err)
		return nil
	}

	pb := response.GetPingResponse()
	if pb == nil {
		return err
	}

	return nil
}

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
	response, err := sc.endpoint.SendRequest(ctx,
		mustWrapMsg(&privvalproto.PubKeyRequest{ChainId: sc.chainID, QuorumHash: quorumHash}),
	)
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

	pk, err := encoding.PubKeyFromProto(resp.PubKey)
	if err != nil {
		return nil, err
	}

	return pk, nil
}

func (sc *SignerClient) GetProTxHash(ctx context.Context) (crypto.ProTxHash, error) {
	response, err := sc.endpoint.SendRequest(ctx, mustWrapMsg(&privvalproto.ProTxHashRequest{ChainId: sc.chainID}))
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

func (sc *SignerClient) GetFirstQuorumHash(ctx context.Context) (crypto.QuorumHash, error) {
	return nil, errors.New("getFirstQuorumHash should not be called on a signer client")
}

func (sc *SignerClient) GetThresholdPublicKey(ctx context.Context, quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	if len(quorumHash.Bytes()) != crypto.DefaultHashSize {
		return nil, fmt.Errorf("quorum hash must be 32 bytes long if requesting public key from dash core")
	}

	response, err := sc.endpoint.SendRequest(ctx,
		mustWrapMsg(&privvalproto.ThresholdPubKeyRequest{ChainId: sc.chainID, QuorumHash: quorumHash}),
	)
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
	ctx context.Context,
	chainID string,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	vote *tmproto.Vote,
	stateID types.StateID,
	logger log.Logger,
) error {
	// fmt.Printf("--> sending request to sign vote (%d/%d) %v - %v", vote.Height, vote.Round, vote.BlockID, vote)
	stateIDProto := stateID.ToProto()

	voteRequest := privvalproto.SignVoteRequest{
		Vote:       vote,
		ChainId:    chainID,
		QuorumType: int32(quorumType),
		QuorumHash: quorumHash,
		StateId:    &stateIDProto,
	}

	response, err := sc.endpoint.SendRequest(ctx, mustWrapMsg(&voteRequest))
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
func (sc *SignerClient) SignProposal(
	ctx context.Context,
	chainID string,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	proposal *tmproto.Proposal,
) (tmbytes.HexBytes, error) {
	response, err := sc.endpoint.SendRequest(ctx, mustWrapMsg(
		&privvalproto.SignProposalRequest{Proposal: proposal, ChainId: chainID,
			QuorumType: int32(quorumType), QuorumHash: quorumHash},
	))
	if err != nil {
		return nil, err
	}

	resp := response.GetSignedProposalResponse()
	if resp == nil {
		return nil, ErrUnexpectedResponse
	}
	if resp.Error != nil {
		return nil, &RemoteSignerError{Code: int(resp.Error.Code), Description: resp.Error.Description}
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
