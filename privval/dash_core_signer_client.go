package privval

import (
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	"strconv"
	"time"

	rpc "github.com/dashevo/dashd-go/rpcclient"
	"github.com/tendermint/tendermint/crypto"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	privvalproto "github.com/tendermint/tendermint/proto/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

// SignerClient implements PrivValidator.
// Handles remote validator connections that provide signing services
type DashCoreSignerClient struct {
	endpoint *rpc.Client
	port uint16
	rpcUsername string
	rpcPassword string
	chainID  string
}

var _ types.PrivValidator = (*DashCoreSignerClient)(nil)

// NewSignerClient returns an instance of SignerClient.
// it will start the endpoint (if not already started)
func NewDashCoreSignerClient(port uint16, rpcUsername string, rpcPassword string, chainID string) (*DashCoreSignerClient, error) {
	portString := strconv.FormatUint(uint64(port), 10)
	// Connect to local bitcoin core RPC server using HTTP POST mode.
	connCfg := &rpc.ConnConfig{
		Host:         "localhost:" + portString,
		User:         rpcUsername,
		Pass:         rpcPassword,
		HTTPPostMode: true, // Dash core only supports HTTP POST mode
		DisableTLS:   true, // Dash core does not provide TLS by default
	}
	// Notice the notification parameter is nil since notifications are
	// not supported in HTTP POST mode.
	client, err := rpc.New(connCfg, nil)
	if err != nil {
		return nil, err
	}
	defer client.Shutdown()

	return &DashCoreSignerClient{endpoint:client, port: port, rpcUsername: rpcUsername, rpcPassword: rpcPassword, chainID: chainID}, nil
}

// Close closes the underlying connection
func (sc *DashCoreSignerClient) Close() error {
	sc.endpoint.Disconnect()
	return nil
}

// IsConnected indicates with the signer is connected to a remote signing service
func (sc *DashCoreSignerClient) IsConnected() bool {
	return sc.endpoint.connec()
}

// WaitForConnection waits maxWait for a connection or returns a timeout error
func (sc *DashCoreSignerClient) WaitForConnection(maxWait time.Duration) error {
	return sc.endpoint.WaitForConnection(maxWait)
}

//--------------------------------------------------------
// Implement PrivValidator

// Ping sends a ping request to the remote signer
func (sc *DashCoreSignerClient) Ping() error {
	err := sc.endpoint.Ping()
	if err != nil {
		return err
	}

	pb, err := sc.endpoint.GetPeerInfo()
	if pb == nil {
		return err
	}

	return nil
}

func (sc *DashCoreSignerClient) ExtractIntoValidator(height int64, quorumHash crypto.QuorumHash) *types.Validator {
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
func (sc *DashCoreSignerClient) GetPubKey(quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	if len(quorumHash.Bytes()) != crypto.DefaultHashSize {
		return nil, fmt.Errorf("quorum hash must be 32 bytes long if requesting public key from dash core")
	}
	response, err := sc.endpoint.QuorumInfo(btcjson.LLMQType_100_67, quorumHash, false)
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	quorumMembers := response.Members


	pk, err := cryptoenc.PubKeyFromProto(resp.PubKey)
	if err != nil {
		return nil, err
	}

	return pk, nil
}

func (sc *DashCoreSignerClient) GetProTxHash() (crypto.ProTxHash, error) {
	masternodeStatus, err := sc.endpoint.MasternodeStatus()
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	if len(masternodeStatus.ProTxHash) != 32 {
		return nil, fmt.Errorf("proTxHash is invalid size")
	}

	return masternodeStatus.ProTxHash, nil
}

// SignVote requests a remote signer to sign a vote
func (sc *DashCoreSignerClient) SignVote(chainID string, quorumHash crypto.QuorumHash, vote *tmproto.Vote) error {
	// fmt.Printf("--> sending request to sign vote (%d/%d) %v - %v", vote.Height, vote.Round, vote.BlockID, vote)
	response, err := sc.endpoint.QuorumSign()
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
func (sc *DashCoreSignerClient) SignProposal(chainID string, quorumHash crypto.QuorumHash, proposal *tmproto.Proposal) error {
	message := mustWrapMsg(
		&privvalproto.SignProposalRequest{Proposal: proposal, ChainId: chainID},
	)

	response, err := sc.endpoint.QuorumSign(btcjson.LLMQType_100_67, requestId, message, quorumHash, false)
	if err != nil {
		return err
	}

	if response == nil {
		return ErrUnexpectedResponse
	}
	if err != nil {
		return &RemoteSignerError{Code: int(resp.Error.Code), Description: err.Error()}
	}

	proposal.Signature = response.Signature

	*proposal = resp.Proposal

	return nil
}

func (sc *DashCoreSignerClient) UpdatePrivateKey(privateKey crypto.PrivKey, height int64) error {
	// the private key is dealt with on the abci client
	return nil
}
