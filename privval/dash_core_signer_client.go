package privval

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/dashevo/dashd-go/btcjson"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	dashcore "github.com/tendermint/tendermint/dash/core"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

// DashPrivValidator is a PrivValidator that uses Dash-specific logic
type DashPrivValidator interface {
	types.PrivValidator
	dashcore.QuorumVerifier
	DashRPCClient() dashcore.Client
	// QuorumSign executes quorum signature process and returns signature and signHash
	QuorumSign(
		ctx context.Context,
		msgHash []byte,
		requestIDHash []byte,
		quorumType btcjson.LLMQType,
		quorumHash crypto.QuorumHash,
	) (signature []byte, signHash []byte, err error)
}

// DashCoreSignerClient implements DashPrivValidator.
// Handles remote validator connections that provide signing services
type DashCoreSignerClient struct {
	dashCoreRPCClient dashcore.Client
	cachedProTxHash   crypto.ProTxHash
	defaultQuorumType btcjson.LLMQType
}

var _ DashPrivValidator = (*DashCoreSignerClient)(nil)

// NewDashCoreSignerClient returns an instance of SignerClient.
// it will start the endpoint (if not already started)
func NewDashCoreSignerClient(
	client dashcore.Client, defaultQuorumType btcjson.LLMQType,
) (*DashCoreSignerClient, error) {
	return &DashCoreSignerClient{dashCoreRPCClient: client, defaultQuorumType: defaultQuorumType}, nil
}

// Close closes the underlying connection
func (sc *DashCoreSignerClient) Close() error {
	err := sc.dashCoreRPCClient.Close()
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
	for i := 0; i < 3; i++ {
		if err = sc.ping(); err == nil {
			return nil
		}
	}

	return err
}

// ping sends a ping request to the remote signer
func (sc *DashCoreSignerClient) ping() error {
	err := sc.dashCoreRPCClient.Ping()
	if err != nil {
		return err
	}

	return nil
}

func (sc *DashCoreSignerClient) ExtractIntoValidator(ctx context.Context, quorumHash crypto.QuorumHash) *types.Validator {
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
func (sc *DashCoreSignerClient) GetPubKey(ctx context.Context, quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	if len(quorumHash.Bytes()) != crypto.DefaultHashSize {
		return nil, fmt.Errorf("quorum hash must be 32 bytes long if requesting public key from dash core")
	}

	response, err := sc.dashCoreRPCClient.QuorumInfo(sc.defaultQuorumType, quorumHash)
	if err != nil {
		return nil, fmt.Errorf("getPubKey Quorum Info Error for (%d) %s : %w", sc.defaultQuorumType, quorumHash.String(), err)
	}

	proTxHash, err := sc.GetProTxHash(ctx)

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
			return nil, fmt.Errorf(
				"decoding proTxHash %d is incorrect size when getting public key : %v",
				len(decodedMemberProTxHash),
				err,
			)
		}
		if bytes.Equal(proTxHash, decodedMemberProTxHash) {
			decodedPublicKeyShare, err = hex.DecodeString(quorumMember.PubKeyShare)
			found = true
			if err != nil {
				return nil, fmt.Errorf("error decoding publicKeyShare : %v", err)
			}
			if len(decodedPublicKeyShare) != bls12381.PubKeySize {
				return nil, fmt.Errorf(
					"decoding public key share %d is incorrect size when getting public key : %v",
					len(decodedMemberProTxHash),
					err,
				)
			}
			break
		}
	}

	if len(decodedPublicKeyShare) != bls12381.PubKeySize {
		if found {
			// We found it, we should have a public key share
			return nil, fmt.Errorf("no public key share found")
		}
		// We are not part of the quorum, there is no error
		return nil, nil
	}

	return bls12381.PubKey(decodedPublicKeyShare), nil
}

func (sc *DashCoreSignerClient) GetFirstQuorumHash(ctx context.Context) (crypto.QuorumHash, error) {
	return nil, errors.New("getFirstQuorumHash should not be called on a dash core signer client")
}

func (sc *DashCoreSignerClient) GetThresholdPublicKey(ctx context.Context, quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	if len(quorumHash.Bytes()) != crypto.DefaultHashSize {
		return nil, fmt.Errorf("quorum hash must be 32 bytes long if requesting public key from dash core")
	}

	response, err := sc.dashCoreRPCClient.QuorumInfo(sc.defaultQuorumType, quorumHash)
	if err != nil {
		return nil, fmt.Errorf(
			"getThresholdPublicKey Quorum Info Error for (%d) %s : %w",
			sc.defaultQuorumType,
			quorumHash.String(),
			err,
		)
	}
	decodedThresholdPublicKey, err := hex.DecodeString(response.QuorumPublicKey)
	if len(decodedThresholdPublicKey) != bls12381.PubKeySize {
		return nil, fmt.Errorf(
			"decoding thresholdPublicKey %d is incorrect size when getting public key : %v",
			len(decodedThresholdPublicKey),
			err,
		)
	}
	return bls12381.PubKey(decodedThresholdPublicKey), nil
}

func (sc *DashCoreSignerClient) GetHeight(ctx context.Context, quorumHash crypto.QuorumHash) (int64, error) {
	return 0, fmt.Errorf("getHeight should not be called on a dash core signer client %s", quorumHash.String())
}

func (sc *DashCoreSignerClient) GetProTxHash(ctx context.Context) (crypto.ProTxHash, error) {
	if sc.cachedProTxHash != nil {
		return sc.cachedProTxHash, nil
	}

	masternodeStatus, err := sc.dashCoreRPCClient.MasternodeStatus()
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	decodedProTxHash, err := hex.DecodeString(masternodeStatus.ProTxHash)
	if err != nil {
		return nil, fmt.Errorf("error decoding proTxHash : %v", err)
	}
	if len(decodedProTxHash) != crypto.DefaultHashSize {
		// We are proof of service banned. Get the proTxHash from our IP Address
		networkInfo, err := sc.dashCoreRPCClient.GetNetworkInfo()
		if err == nil && len(networkInfo.LocalAddresses) > 0 {
			localAddress := networkInfo.LocalAddresses[0].Address
			localPort := networkInfo.LocalAddresses[0].Port
			localHost := fmt.Sprintf("%s:%d", localAddress, localPort)
			results, err := sc.dashCoreRPCClient.MasternodeListJSON(localHost)
			if err == nil {
				for _, v := range results {
					decodedProTxHash, err = hex.DecodeString(v.ProTxHash)
					if err != nil {
						return nil, fmt.Errorf("error decoding proTxHash: %v", err)
					}
				}
			}
		}
		if len(decodedProTxHash) != crypto.DefaultHashSize {
			return nil, fmt.Errorf(
				"decoding proTxHash %d is incorrect size when signing proposal : %v",
				len(decodedProTxHash),
				err,
			)
		}
	}

	sc.cachedProTxHash = decodedProTxHash

	return decodedProTxHash, nil
}

// SignVote requests a remote signer to sign a vote
func (sc *DashCoreSignerClient) SignVote(
	ctx context.Context, chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash,
	protoVote *tmproto.Vote, stateID types.StateID, logger log.Logger) error {
	if len(quorumHash) != crypto.DefaultHashSize {
		return fmt.Errorf("quorum hash is not the right length %s", quorumHash.String())
	}

	quorumSigns, err := types.MakeQuorumSigns(chainID, quorumType, quorumHash, protoVote, stateID)
	if err != nil {
		return err
	}
	blockQS := quorumSigns.Block

	qs, err := sc.quorumSignAndVerify(ctx, quorumType, quorumHash, blockQS.ReqID, blockQS.Hash())
	if err != nil {
		return err
	}

	// No need to check the error as this is only used for logging
	proTxHash, _ := sc.GetProTxHash(ctx)

	logger.Debug("signed vote", "height", protoVote.Height, "round", protoVote.Round, "voteType", protoVote.Type,
		"quorumType", quorumType, "quorumHash", quorumHash, "signature", qs.sign, "signBytes", blockQS.Raw,
		"proTxHash", proTxHash, "coreBlockRequestId", qs.ID, "blockRequestId",
		hex.EncodeToString(blockQS.ReqID), "coreSignId", tmbytes.Reverse(qs.signHash),
		"signId", hex.EncodeToString(blockQS.ID))

	protoVote.BlockSignature = qs.sign

	// Only sign the state when voting for the block
	if protoVote.BlockID.Hash != nil {
		signItem := quorumSigns.State
		resp, err := sc.quorumSignAndVerify(ctx, quorumType, quorumHash, signItem.ReqID, signItem.Hash())
		if err != nil {
			return err
		}
		protoVote.StateSignature = resp.sign
	}

	return sc.signVoteExtensions(ctx, quorumType, quorumHash, protoVote, quorumSigns)
}

// SignProposal requests a remote signer to sign a proposal
func (sc *DashCoreSignerClient) SignProposal(
	ctx context.Context, chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, proposalProto *tmproto.Proposal,
) (tmbytes.HexBytes, error) {
	if quorumType == 0 {
		return nil, fmt.Errorf("error signing proposal with invalid quorum type")
	}
	signItem := types.SignItem{
		ReqID: types.ProposalRequestIDProto(proposalProto),
		Raw:   types.ProposalBlockSignBytes(chainID, proposalProto),
	}
	resp, err := sc.quorumSignAndVerify(ctx, quorumType, quorumHash, signItem.ReqID, signItem.Hash())
	if err != nil {
		return nil, err
	}
	proposalProto.Signature = resp.sign
	return nil, nil
}

// QuorumSign implements DashPrivValidator
func (sc *DashCoreSignerClient) QuorumSign(
	ctx context.Context,
	msgHash []byte,
	requestIDHash []byte,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
) ([]byte, []byte, error) {
	qs, err := sc.quorumSignAndVerify(ctx, quorumType, quorumHash, requestIDHash, msgHash)
	if err != nil {
		return nil, nil, err
	}
	return qs.sign, qs.signHash, nil
}

func (sc *DashCoreSignerClient) UpdatePrivateKey(
	ctx context.Context,
	privateKey crypto.PrivKey,
	quorumHash crypto.QuorumHash,
	thresholdPublicKey crypto.PubKey,
	height int64,
) {

}

func (sc *DashCoreSignerClient) GetPrivateKey(ctx context.Context, quorumHash crypto.QuorumHash) (crypto.PrivKey, error) {
	key := &dashConsensusPrivateKey{
		quorumHash: quorumHash,
		quorumType: sc.defaultQuorumType,
		privval:    sc,
	}

	return key, nil
}

// QuorumVerify implements dashcore.QuorumVerifier
func (sc *DashCoreSignerClient) QuorumVerify(
	quorumType btcjson.LLMQType,
	requestID tmbytes.HexBytes,
	messageHash tmbytes.HexBytes,
	signature tmbytes.HexBytes,
	quorumHash tmbytes.HexBytes,
) (bool, error) {
	return sc.dashCoreRPCClient.QuorumVerify(quorumType, requestID, messageHash, signature, quorumHash)
}

// DashRPCClient implements DashPrivValidator
func (sc *DashCoreSignerClient) DashRPCClient() dashcore.Client {
	if sc == nil {
		return nil
	}
	return sc.dashCoreRPCClient
}

func (sc *DashCoreSignerClient) signVoteExtensions(
	ctx context.Context,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	protoVote *tmproto.Vote,
	quorumSignData types.QuorumSignData,
) error {
	if protoVote.Type != tmproto.PrecommitType {
		if len(protoVote.VoteExtensions) > 0 {
			return errors.New("unexpected vote extension - extensions are only allowed in precommits")
		}
		return nil
	}
	for et, extensions := range protoVote.VoteExtensionsToMap() {
		for i, ext := range extensions {
			signItem := quorumSignData.Extensions[et][i]
			resp, err := sc.quorumSignAndVerify(ctx, quorumType, quorumHash, signItem.ReqID, signItem.Hash())
			if err != nil {
				return err
			}
			ext.Signature = resp.sign
		}
	}
	return nil
}

func (sc *DashCoreSignerClient) quorumSignAndVerify(
	ctx context.Context,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	reqID, msgHash []byte,
) (*quorumSignResult, error) {
	qs, err := sc.quorumSign(quorumType, quorumHash, reqID, msgHash)
	if err != nil {
		return nil, err
	}
	pubKey, err := sc.GetPubKey(ctx, quorumHash)
	if err != nil {
		return nil, &RemoteSignerError{Code: 500, Description: err.Error()}
	}
	verified := pubKey.VerifySignatureDigest(reqID, qs.sign)
	if !verified {
		return nil, fmt.Errorf("unable to verify signature with pubkey %s", pubKey.String())
	}
	return qs, nil
}

func (sc *DashCoreSignerClient) quorumSign(
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	reqID, msgHash []byte,
) (*quorumSignResult, error) {
	resp, err := sc.dashCoreRPCClient.QuorumSign(quorumType, reqID, msgHash, quorumHash)
	if err != nil {
		return nil, &RemoteSignerError{Code: 500, Description: "cannot sign vote: " + err.Error()}
	}
	if resp == nil {
		return nil, ErrUnexpectedResponse
	}
	sign, err := hex.DecodeString(resp.Signature)
	if err != nil {
		return nil, fmt.Errorf("error decoding signature when signing vote : %v", err)
	}
	if len(sign) != bls12381.SignatureSize {
		return nil, fmt.Errorf("decoding signature %d is incorrect size when signing vote : %v", len(sign), err)
	}
	signHash, err := hex.DecodeString(resp.SignHash)
	if err != nil {
		return nil, fmt.Errorf("error decoding coreSignID when signing vote : %v", err)
	}
	return &quorumSignResult{resp, sign, signHash}, nil
}

type quorumSignResult struct {
	*btcjson.QuorumSignResult
	sign     []byte
	signHash []byte
}
