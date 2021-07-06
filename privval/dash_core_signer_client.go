package privval

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"runtime/debug"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/dashcore/rpc"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	types "github.com/tendermint/tendermint/types"
)

// DashCoreSignerClient implements PrivValidator.
// Handles remote validator connections that provide signing services
type DashCoreSignerClient struct {
	dashCoreRpcClient dashcore.DashCoreClient
	cachedProTxHash   crypto.ProTxHash
	defaultQuorumType btcjson.LLMQType
}

var _ types.PrivValidator = (*DashCoreSignerClient)(nil)

// NewDashCoreSignerClient returns an instance of SignerClient.
// it will start the endpoint (if not already started)
func NewDashCoreSignerClient(client dashcore.DashCoreClient, defaultQuorumType btcjson.LLMQType) (*DashCoreSignerClient, error) {
	return &DashCoreSignerClient{dashCoreRpcClient: client, defaultQuorumType: defaultQuorumType}, nil
}

// Close closes the underlying connection
func (sc *DashCoreSignerClient) Close() error {
	err := sc.dashCoreRpcClient.Close()
	if err != nil {
		return err
	}
	return nil
}

//--------------------------------------------------------
// Implement PrivValidator

// Ping sends a ping request to the remote signer and will retry 2 extra times if failure
func (sc *DashCoreSignerClient) Ping() error {
	var err error
	for i:=0; i<3; i++ {
		if err = sc.ping(); err == nil {
			return nil
		}
	}

	return err
}

// ping sends a ping request to the remote signer
func (sc *DashCoreSignerClient) ping() error {
	err := sc.dashCoreRpcClient.Ping()
	if err != nil {
		return err
	}

	return nil
}


func (sc *DashCoreSignerClient) ExtractIntoValidator(quorumHash crypto.QuorumHash) *types.Validator {
	pubKey, _ := sc.GetPubKey(quorumHash)
	proTxHash, _ := sc.GetProTxHash()
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
func (sc *DashCoreSignerClient) GetPubKey(quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	if len(quorumHash.Bytes()) != crypto.DefaultHashSize {
		return nil, fmt.Errorf("quorum hash must be 32 bytes long if requesting public key from dash core")
	}

	response, err := sc.dashCoreRpcClient.QuorumInfo(sc.defaultQuorumType, quorumHash)
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

func (sc *DashCoreSignerClient) GetFirstQuorumHash() (crypto.QuorumHash, error) {
	return nil, errors.New("getFirstQuorumHash should not be called on a dash core signer client")
}

func (sc *DashCoreSignerClient) GetThresholdPublicKey(quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	if len(quorumHash.Bytes()) != crypto.DefaultHashSize {
		return nil, fmt.Errorf("quorum hash must be 32 bytes long if requesting public key from dash core")
	}

	response, err := sc.dashCoreRpcClient.QuorumInfo(sc.defaultQuorumType, quorumHash)
	if err != nil {
		return nil, fmt.Errorf("getThresholdPublicKey Quorum Info Error for (%d) %s : %w", sc.defaultQuorumType, quorumHash.String(), err)
	}
	decodedThresholdPublicKey, err := hex.DecodeString(response.QuorumPublicKey)
	if len(decodedThresholdPublicKey) != bls12381.PubKeySize {
		return nil, fmt.Errorf("decoding thresholdPublicKey %d is incorrect size when getting public key : %v", len(decodedThresholdPublicKey), err)
	}
	return bls12381.PubKey(decodedThresholdPublicKey), nil
}
func (sc *DashCoreSignerClient) GetHeight(quorumHash crypto.QuorumHash) (int64, error) {
	return 0, fmt.Errorf("getHeight should not be called on a dash core signer client %s", quorumHash.String())
}

func (sc *DashCoreSignerClient) GetProTxHash() (crypto.ProTxHash, error) {
	if sc.cachedProTxHash != nil {
		return sc.cachedProTxHash, nil
	}

	masternodeStatus, err := sc.dashCoreRpcClient.MasternodeStatus()
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	decodedProTxHash, err := hex.DecodeString(masternodeStatus.ProTxHash)
	if err != nil {
		return nil, fmt.Errorf("error decoding proTxHash : %v", err)
	}
	if len(decodedProTxHash) != crypto.DefaultHashSize {
		// We are proof of service banned. Get the proTxHash from our IP Address
		networkInfo, err := sc.dashCoreRpcClient.GetNetworkInfo()
		if err == nil && len(networkInfo.LocalAddresses) > 0 {
			localAddress := networkInfo.LocalAddresses[0].Address
			localPort := networkInfo.LocalAddresses[0].Port
			localHost := fmt.Sprintf("%s:%d", localAddress, localPort)
			results, err := sc.dashCoreRpcClient.MasternodeListJSON(localHost)
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

	stateMessageHash := crypto.Sha256(stateSignBytes)

	blockRequestId := types.VoteBlockRequestIdProto(protoVote)

	stateRequestId := types.VoteStateRequestIdProto(protoVote)

	// proTxHash, err := sc.GetProTxHash()

	blockResponse, err := sc.dashCoreRpcClient.QuorumSign(quorumType, blockRequestId, blockMessageHash, quorumHash)

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

	stateResponse, err := sc.dashCoreRpcClient.QuorumSign(sc.defaultQuorumType, stateRequestId, stateMessageHash, quorumHash)

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
func (sc *DashCoreSignerClient) SignProposal(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, proposalProto *tmproto.Proposal) ([]byte, error) {
	messageBytes := types.ProposalBlockSignBytes(chainID, proposalProto)

	messageHash := crypto.Sha256(messageBytes)

	requestIdHash := types.ProposalRequestIdProto(proposalProto)

	if quorumType == 0 {
		return nil, fmt.Errorf("error signing proposal with invalid quorum type")
	}

	response, err := sc.dashCoreRpcClient.QuorumSign(quorumType, requestIdHash, messageHash, quorumHash)

	if response == nil {
		return nil, ErrUnexpectedResponse
	}
	if err != nil {
		return nil, &RemoteSignerError{Code: 500, Description: err.Error()}
	}

	decodedSignature, err := hex.DecodeString(response.Signature)
	if err != nil {
		return nil, fmt.Errorf("error decoding signature when signing proposal : %v", err)
	}
	if len(decodedSignature) != bls12381.SignatureSize {
		return nil, fmt.Errorf("decoding signature %d is incorrect size when signing proposal : %v", len(decodedSignature), err)
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

	return nil, nil
}

func (sc *DashCoreSignerClient) UpdatePrivateKey(privateKey crypto.PrivKey, quorumHash crypto.QuorumHash, thresholdPublicKey crypto.PubKey, height int64) {

}

func (sc *DashCoreSignerClient) GetPrivateKey(quorumHash crypto.QuorumHash) (crypto.PrivKey, error) {
	return nil, nil
}
