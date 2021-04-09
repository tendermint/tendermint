package privval

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/dashevo/dashd-go/btcjson"
	"github.com/tendermint/tendermint/crypto/bls12381"

	rpc "github.com/dashevo/dashd-go/rpcclient"
	"github.com/tendermint/tendermint/crypto"
	privvalproto "github.com/tendermint/tendermint/proto/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	types "github.com/tendermint/tendermint/types"
)

// SignerClient implements PrivValidator.
// Handles remote validator connections that provide signing services
type DashCoreSignerClient struct {
	endpoint    *rpc.Client
	port        uint16
	rpcUsername string
	rpcPassword string
	chainID     string
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

	return &DashCoreSignerClient{endpoint: client, port: port, rpcUsername: rpcUsername, rpcPassword: rpcPassword, chainID: chainID}, nil
}

// Close closes the underlying connection
func (sc *DashCoreSignerClient) Close() error {
	sc.endpoint.Shutdown()
	return nil
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

	response, err := sc.endpoint.QuorumInfo(btcjson.LLMQType_100_67, quorumHash.String(), false)
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	proTxHash, err := sc.GetProTxHash()

	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	var decodedPublicKeyShare []byte

	for _, quorumMember := range response.Members {
		decodedMemberProTxHash, err := hex.DecodeString(quorumMember.ProTxHash)
		if err != nil {
			return nil, fmt.Errorf("error decoding proTxHash : %v", err)
		}
		if len(decodedMemberProTxHash) != crypto.DefaultHashSize {
			return nil, fmt.Errorf("decoding proTxHash %d is incorrect size when getting public key : %v", len(decodedMemberProTxHash), err)
		}
		if bytes.Equal(proTxHash, decodedMemberProTxHash) {
			decodedPublicKeyShare, err = hex.DecodeString(quorumMember.PubKeyShare)
			if err != nil {
				return nil, fmt.Errorf("error decoding publicKeyShare : %v", err)
			}
			if len(decodedPublicKeyShare) != bls12381.PubKeySize {
				return nil, fmt.Errorf("decoding public key share %d is incorrect size when getting public key : %v", len(decodedMemberProTxHash), err)
			}
			break
		}
	}

	return bls12381.PubKey(decodedPublicKeyShare), nil
}

func (sc *DashCoreSignerClient) GetProTxHash() (crypto.ProTxHash, error) {
	masternodeStatus, err := sc.endpoint.MasternodeStatus()
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	decodedProTxHash, err := hex.DecodeString(masternodeStatus.ProTxHash)
	if err != nil {
		return nil, fmt.Errorf("error decoding proTxHash : %v", err)
	}
	if len(decodedProTxHash) != crypto.DefaultHashSize {
		return nil, fmt.Errorf("decoding proTxHash %d is incorrect size when signing proposal : %v", len(decodedProTxHash), err)
	}

	return decodedProTxHash, nil
}

func VoteBlockRequestId(vote *tmproto.Vote) []byte {
	requestIdMessage := []byte("dpbvote")
	heightByteArray := make([]byte, 8)
	binary.LittleEndian.PutUint64(heightByteArray, uint64(vote.Height))
	roundByteArray := make([]byte, 4)
	binary.LittleEndian.PutUint32(roundByteArray, uint32(vote.Round))

	requestIdMessage = append(requestIdMessage, heightByteArray...)
	requestIdMessage = append(requestIdMessage, roundByteArray...)

	return crypto.Sha256(requestIdMessage)
}

func VoteStateRequestId(vote *tmproto.Vote) []byte {
	requestIdMessage := []byte("dpsvote")
	heightByteArray := make([]byte, 8)
	binary.LittleEndian.PutUint64(heightByteArray, uint64(vote.Height))
	roundByteArray := make([]byte, 4)
	binary.LittleEndian.PutUint32(roundByteArray, uint32(vote.Round))

	requestIdMessage = append(requestIdMessage, heightByteArray...)
	requestIdMessage = append(requestIdMessage, roundByteArray...)

	return crypto.Sha256(requestIdMessage)
}

// SignVote requests a remote signer to sign a vote
func (sc *DashCoreSignerClient) SignVote(chainID string, quorumHash crypto.QuorumHash, vote *tmproto.Vote) error {
	blockSignBytes := types.VoteBlockSignBytes(chainID, vote)
	stateSignBytes := types.VoteStateSignBytes(chainID, vote)

	blockMessageHash := crypto.Sha256(blockSignBytes)

	blockMessageHashString := strings.ToUpper(hex.EncodeToString(blockMessageHash))

	stateMessageHash := crypto.Sha256(stateSignBytes)

	stateMessageHashString := strings.ToUpper(hex.EncodeToString(stateMessageHash))

	blockRequestIdHash := VoteBlockRequestId(vote)

	blockRequestIdHashString := strings.ToUpper(hex.EncodeToString(blockRequestIdHash))

	stateRequestIdHash := VoteStateRequestId(vote)

	stateRequestIdHashString := strings.ToUpper(hex.EncodeToString(stateRequestIdHash))

	var quorumHashFixed *chainhash.Hash

	copy(quorumHashFixed[:], quorumHash)

	blockResponse, err := sc.endpoint.QuorumSign(btcjson.LLMQType_100_67, blockRequestIdHashString, blockMessageHashString, quorumHash.String(), false)

	if blockResponse == nil {
		return ErrUnexpectedResponse
	}
	if err != nil {
		return &RemoteSignerError{Code: 500, Description: err.Error()}
	}

	blockDecodedSignature, err := hex.DecodeString(blockResponse.Signature)
	if err != nil {
		return fmt.Errorf("error decoding signature when signing proposal : %v", err)
	}
	if len(blockDecodedSignature) != bls12381.SignatureSize {
		return fmt.Errorf("decoding signature %d is incorrect size when signing proposal : %v", len(blockDecodedSignature), err)
	}

	stateResponse, err := sc.endpoint.QuorumSign(btcjson.LLMQType_100_67, stateRequestIdHashString, stateMessageHashString, quorumHash.String(), false)

	if stateResponse == nil {
		return ErrUnexpectedResponse
	}
	if err != nil {
		return &RemoteSignerError{Code: 500, Description: err.Error()}
	}

	stateDecodedSignature, err := hex.DecodeString(stateResponse.Signature)
	if err != nil {
		return fmt.Errorf("error decoding signature when signing proposal : %v", err)
	}
	if len(stateDecodedSignature) != bls12381.SignatureSize {
		return fmt.Errorf("decoding signature %d is incorrect size when signing proposal : %v", len(stateDecodedSignature), err)
	}

	vote.BlockSignature = blockDecodedSignature
	vote.StateSignature = stateDecodedSignature

	return nil
}

func ProposalRequestId(proposal *tmproto.Proposal) []byte {
	requestIdMessage := []byte("dpprop")
	heightByteArray := make([]byte, 8)
	binary.LittleEndian.PutUint64(heightByteArray, uint64(proposal.Height))
	roundByteArray := make([]byte, 4)
	binary.LittleEndian.PutUint32(roundByteArray, uint32(proposal.Round))

	requestIdMessage = append(requestIdMessage, heightByteArray...)
	requestIdMessage = append(requestIdMessage, roundByteArray...)

	return crypto.Sha256(requestIdMessage)
}

// SignProposal requests a remote signer to sign a proposal
func (sc *DashCoreSignerClient) SignProposal(chainID string, quorumHash crypto.QuorumHash, proposal *tmproto.Proposal) error {
	message := mustWrapMsg(
		&privvalproto.SignProposalRequest{Proposal: proposal, ChainId: chainID},
	)

	messageBytes, err := message.Marshal()

	if err != nil {
		return fmt.Errorf("error marshalling message when signing proposal : %v", err)
	}

	messageHash := crypto.Sha256(messageBytes)

	messageHashString := strings.ToUpper(hex.EncodeToString(messageHash))

	requestIdHash := ProposalRequestId(proposal)

	requestIdHashString := strings.ToUpper(hex.EncodeToString(requestIdHash))

	response, err := sc.endpoint.QuorumSign(btcjson.LLMQType_100_67, requestIdHashString, messageHashString, quorumHash.String(), false)

	if response == nil {
		return ErrUnexpectedResponse
	}
	if err != nil {
		return &RemoteSignerError{Code: 500, Description: err.Error()}
	}

	decodedSignature, err := hex.DecodeString(response.Signature)
	if err != nil {
		return fmt.Errorf("error decoding signature when signing proposal : %v", err)
	}
	if len(decodedSignature) != bls12381.SignatureSize {
		return fmt.Errorf("decoding signature %d is incorrect size when signing proposal : %v", len(decodedSignature), err)
	}

	proposal.Signature = decodedSignature

	return nil
}

func (sc *DashCoreSignerClient) UpdatePrivateKey(privateKey crypto.PrivKey, height int64) error {
	// the private key is dealt with on the abci client
	return nil
}
