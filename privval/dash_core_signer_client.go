package privval

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/tendermint/tendermint/crypto/bls12381"

	rpc "github.com/dashevo/dashd-go/rpcclient"
	"github.com/tendermint/tendermint/crypto"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	types "github.com/tendermint/tendermint/types"
)

// DashCoreSignerClient implements PrivValidator.
// Handles remote validator connections that provide signing services
type DashCoreSignerClient struct {
	endpoint          *rpc.Client
	host              string
	cachedProTxHash   crypto.ProTxHash
	rpcUsername       string
	rpcPassword       string
	defaultQuorumType btcjson.LLMQType
}

var _ types.PrivValidator = (*DashCoreSignerClient)(nil)

// NewDashCoreSignerClient returns an instance of SignerClient.
// it will start the endpoint (if not already started)
func NewDashCoreSignerClient(host string, rpcUsername string, rpcPassword string, defaultQuorumType btcjson.LLMQType) (*DashCoreSignerClient, error) {
	// Connect to local dash core RPC server using HTTP POST mode.
	connCfg := &rpc.ConnConfig{
		Host:         host,
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

	return &DashCoreSignerClient{endpoint: client, host: host, rpcUsername: rpcUsername, rpcPassword: rpcPassword, defaultQuorumType: defaultQuorumType}, nil
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

	response, err := sc.endpoint.QuorumInfo(sc.defaultQuorumType, quorumHash.String(), false)
	if err != nil {
		return nil, fmt.Errorf("getPubKey Quorum Info Error for (%d) %s : %w", sc.defaultQuorumType, quorumHash.String(), err)
	}

	proTxHash, err := sc.GetProTxHash()

	if err != nil {
		return nil, fmt.Errorf("getPubKey proTxHash error: %w", err)
	}

	var decodedPublicKeyShare []byte

	found := false

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
			found = true
			if err != nil {
				return nil, fmt.Errorf("error decoding publicKeyShare : %v", err)
			}
			if len(decodedPublicKeyShare) != bls12381.PubKeySize {
				return nil, fmt.Errorf("decoding public key share %d is incorrect size when getting public key : %v", len(decodedMemberProTxHash), err)
			}
			break
		}
	}

	if len(decodedPublicKeyShare) != bls12381.PubKeySize {
		if found == true {
			// We found it, we should have a public key share
			return nil, fmt.Errorf("no public key share found")
		} else {
			// We are not part of the quorum, there is no error
			return nil, nil
		}

	}

	return bls12381.PubKey(decodedPublicKeyShare), nil
}

func (sc *DashCoreSignerClient) GetProTxHash() (crypto.ProTxHash, error) {
	if sc.cachedProTxHash != nil {
		return sc.cachedProTxHash, nil
	}

	masternodeStatus, err := sc.endpoint.MasternodeStatus()
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	decodedProTxHash, err := hex.DecodeString(masternodeStatus.ProTxHash)
	if err != nil {
		return nil, fmt.Errorf("error decoding proTxHash : %v", err)
	}
	if len(decodedProTxHash) != crypto.DefaultHashSize {
		// We are proof of service banned. Get the proTxHash from our IP Address
		networkInfo, err := sc.endpoint.GetNetworkInfo()
		if err == nil && len(networkInfo.LocalAddresses) > 0 {
			localAddress := networkInfo.LocalAddresses[0].Address
			localPort := networkInfo.LocalAddresses[0].Port
			localHost := fmt.Sprintf("%s:%d", localAddress, localPort)
			results, err := sc.endpoint.MasternodeListJSON(localHost)
			if err == nil {
				for _, v := range results {
					decodedProTxHash, err = hex.DecodeString(v.ProTxHash)
				}
			}
		}
		if len(decodedProTxHash) != crypto.DefaultHashSize {
			debug.PrintStack()
			return nil, fmt.Errorf("decoding proTxHash %d is incorrect size when signing proposal : %v", len(decodedProTxHash), err)
		}
	}

	sc.cachedProTxHash = decodedProTxHash

	return decodedProTxHash, nil
}

// SignVote requests a remote signer to sign a vote
func (sc *DashCoreSignerClient) SignVote(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, protoVote *tmproto.Vote) error {
	if len(quorumHash) != crypto.DefaultHashSize {
		return fmt.Errorf("quorum hash is not the right length %s", quorumHash.String())
	}
	blockSignBytes := types.VoteBlockSignBytes(chainID, protoVote)
	stateSignBytes := types.VoteStateSignBytes(chainID, protoVote)

	blockMessageHash := crypto.Sha256(blockSignBytes)

	blockMessageHashString := strings.ToUpper(hex.EncodeToString(blockMessageHash))

	stateMessageHash := crypto.Sha256(stateSignBytes)

	stateMessageHashString := strings.ToUpper(hex.EncodeToString(stateMessageHash))

	blockRequestId := types.VoteBlockRequestIdProto(protoVote)

	blockRequestIdString := strings.ToUpper(hex.EncodeToString(blockRequestId))

	stateRequestId := types.VoteStateRequestIdProto(protoVote)

	stateRequestIdString := strings.ToUpper(hex.EncodeToString(stateRequestId))

	// proTxHash, err := sc.GetProTxHash()

	blockResponse, err := sc.endpoint.QuorumSign(quorumType, blockRequestIdString, blockMessageHashString, quorumHash.String(), false)

	if blockResponse == nil {
		return ErrUnexpectedResponse
	}
	if err != nil {
		return &RemoteSignerError{Code: 500, Description: err.Error()}
	}

	//fmt.Printf("blockResponse %v", blockResponse)
	//
	blockDecodedSignature, err := hex.DecodeString(blockResponse.Signature)
	if err != nil {
		return fmt.Errorf("error decoding signature when signing proposal : %v", err)
	}
	if len(blockDecodedSignature) != bls12381.SignatureSize {
		return fmt.Errorf("decoding signature %d is incorrect size when signing proposal : %v", len(blockDecodedSignature), err)
	}

	/// fmt.Printf("Signed Vote proTxHash %s blockSignBytes %s block signature %s \n", proTxHash, hex.EncodeToString(blockSignBytes),
	//	hex.EncodeToString(blockDecodedSignature))

	// signId := crypto.SignId(sc.defaultQuorumType, bls12381.ReverseBytes(quorumHash), bls12381.ReverseBytes(blockRequestId), bls12381.ReverseBytes(blockMessageHash))

	// fmt.Printf("core returned block requestId %s our block request Id %s\n", blockResponse.ID, blockRequestIdString)
	//
	// fmt.Printf("core block signId %s our block sign Id %s\n", blockResponse.SignHash, hex.EncodeToString(signId))
	//
	//pubKey, err := sc.GetPubKey(quorumHash)
	//verified := pubKey.VerifySignatureDigest(signId, blockDecodedSignature)
	//if verified {
	//	fmt.Printf("Verified core signing with public key %v\n", pubKey)
	//} else {
	//	fmt.Printf("Unable to verify signature %v\n", pubKey)
	//}

	stateResponse, err := sc.endpoint.QuorumSign(sc.defaultQuorumType, stateRequestIdString, stateMessageHashString, quorumHash.String(), false)

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


	// fmt.Printf("Signed Vote proTxHash %s stateSignBytes %s block signature %s \n", proTxHash, hex.EncodeToString(stateSignBytes),
	// 	hex.EncodeToString(stateDecodedSignature))

	// stateSignId := crypto.SignId(sc.defaultQuorumType, bls12381.ReverseBytes(quorumHash), bls12381.ReverseBytes(stateRequestId), bls12381.ReverseBytes(stateMessageHash))

	// fmt.Printf("core returned state requestId %s our state request Id %s\n", stateResponse.ID, stateRequestIdString)
	//
	// fmt.Printf("core state signId %s our state sign Id %s\n", stateResponse.SignHash, hex.EncodeToString(stateSignId))
	//
	//stateVerified := pubKey.VerifySignatureDigest(stateSignId, stateDecodedSignature)
	//if stateVerified {
	//	fmt.Printf("Verified state core signing with public key %v\n", pubKey)
	//} else {
	//	fmt.Printf("Unable to verify state signature %v\n", pubKey)
	//}

	protoVote.BlockSignature = blockDecodedSignature
	protoVote.StateSignature = stateDecodedSignature

	return nil
}

// SignProposal requests a remote signer to sign a proposal
func (sc *DashCoreSignerClient) SignProposal(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, proposalProto *tmproto.Proposal) error {
	messageBytes:= types.ProposalBlockSignBytes(chainID, proposalProto)

	messageHash := crypto.Sha256(messageBytes)

	messageHashString := strings.ToUpper(hex.EncodeToString(messageHash))

	requestIdHash := types.ProposalRequestIdProto(proposalProto)

	requestIdHashString := strings.ToUpper(hex.EncodeToString(requestIdHash))

	if quorumType == 0 {
		return fmt.Errorf("error signing proposal with invalid quorum type")
	}

	response, err := sc.endpoint.QuorumSign(quorumType, requestIdHashString, messageHashString, quorumHash.String(), false)

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

	//fmt.Printf("proposal message that is being signed %v\n", messageBytes)
	//
	//fmt.Printf("proposal response %v\n", response)
	//
	//fmt.Printf("Proposal signBytes %s signature %s \n", hex.EncodeToString(messageBytes),
	//	hex.EncodeToString(decodedSignature))
	//
	//signId := crypto.SignId(sc.defaultQuorumType, bls12381.ReverseBytes(quorumHash), bls12381.ReverseBytes(requestIdHash), bls12381.ReverseBytes(messageHash))
	//
	//fmt.Printf("core returned requestId %s our request Id %s\n", response.ID, requestIdHashString)
	////
	//fmt.Printf("core signId %s our sign Id %s\n", response.SignHash, hex.EncodeToString(signId))
	////
	//pubKey, err := sc.GetPubKey(quorumHash)
	//verified := pubKey.VerifySignatureDigest(signId, decodedSignature)
	//if verified {
	//	fmt.Printf("Verified core signing with public key %v\n", pubKey)
	//} else {
	//	fmt.Printf("Unable to verify signature %v\n", pubKey)
	//}


	proposalProto.Signature = decodedSignature

	return nil
}

func (sc *DashCoreSignerClient) UpdatePrivateKey(privateKey crypto.PrivKey, height int64) error {
	// the private key is dealt with on the abci client
	return nil
}
