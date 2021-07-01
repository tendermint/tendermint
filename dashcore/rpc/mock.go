package dashcore

import (
	"encoding/hex"
	"errors"
	"github.com/tendermint/tendermint/types"
	"strconv"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/libs/bytes"
)

// DashCoreMockClient is an implementation of a mock core-server
type DashCoreMockClient struct {
	chainID  string
	llmqType btcjson.LLMQType
	localPV  types.PrivValidator
	canSign  bool
}

func NewDashCoreMockClient(chainId string, llmqType btcjson.LLMQType, localPV types.PrivValidator, canSign bool) *DashCoreMockClient {
	if localPV == nil {
		panic("localPV must be set")
	}
	return &DashCoreMockClient{
		chainID: chainId,
		llmqType: llmqType,
		localPV: localPV,
		canSign: canSign,
	}
}

// Close closes the underlying connection
func (mc *DashCoreMockClient) Close() error {
	return nil
}

// Ping sends a ping request to the remote signer
func (mc *DashCoreMockClient) Ping() error {
	return nil
}

func (mc *DashCoreMockClient) QuorumInfo(quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash) (*btcjson.QuorumInfoResult, error) {
	var members []btcjson.QuorumMember
	proTxHash, err := mc.localPV.GetProTxHash()
	if err != nil {
		panic(err)
	}
	pk, err := mc.localPV.GetPubKey(quorumHash)
	if err != nil {
		panic(err)
	}
	if pk != nil {
		members = append(members, btcjson.QuorumMember{
			ProTxHash:      proTxHash.String(),
			PubKeyOperator: crypto.CRandHex(96),
			Valid:          true,
			PubKeyShare:    pk.HexString(),
		})
	}
	tpk, err := mc.localPV.GetThresholdPublicKey(quorumHash)
	if err != nil {
		panic(err)
	}
	height, err := mc.localPV.GetHeight(quorumHash)
	if err != nil {
		panic(err)
	}
	return &btcjson.QuorumInfoResult{
		Height:          uint32(height),
		Type:            strconv.Itoa(int(quorumType)),
		QuorumHash:      quorumHash.String(),
		Members:         members,
		QuorumPublicKey: tpk.String(),
	}, nil
}

func (mc *DashCoreMockClient) MasternodeStatus() (*btcjson.MasternodeStatusResult, error) {
	proTxHash, err := mc.localPV.GetProTxHash()
	if err != nil {
		panic(err)
	}
	return &btcjson.MasternodeStatusResult{
		Outpoint:        "",
		Service:         "",
		ProTxHash:       proTxHash.String(),
		CollateralHash:  "",
		CollateralIndex: 0,
		DMNState:        btcjson.DMNState{},
		State:           "",
		Status:          "",
	}, nil
}

func (mc *DashCoreMockClient) GetNetworkInfo() (*btcjson.GetNetworkInfoResult, error) {
	return &btcjson.GetNetworkInfoResult{
		Version:         0,
		SubVersion:      "",
		ProtocolVersion: 0,
		LocalServices:   "",
		LocalRelay:      false,
		TimeOffset:      0,
		Connections:     0,
		NetworkActive:   false,
		Networks:        nil,
		RelayFee:        0,
		IncrementalFee:  0,
		LocalAddresses:  nil,
		Warnings:        "",
	}, nil
}

func (mc *DashCoreMockClient) MasternodeListJSON(filter string) (map[string]btcjson.MasternodelistResultJSON, error) {
	proTxHash, err := mc.localPV.GetProTxHash()
	if err != nil {
		panic(err)
	}
	m := make(map[string]btcjson.MasternodelistResultJSON)
	m[""] = btcjson.MasternodelistResultJSON{
		Address:           "",
		Collateraladdress: "",
		Lastpaidblock:     0,
		Lastpaidtime:      0,
		Owneraddress:      "",
		Payee:             "",
		ProTxHash:         proTxHash.String(),
		Pubkeyoperator:    "",
		Status:            "",
		Votingaddress:     "",
	}

	return m, nil
}

func (mc *DashCoreMockClient) QuorumSign(quorumType btcjson.LLMQType, requestID bytes.HexBytes, messageHash bytes.HexBytes, quorumHash crypto.QuorumHash) (*btcjson.QuorumSignResult, error) {
	if mc.canSign == false {
		return nil, errors.New("dash core mock client not set up for signing")
	}

	signID := crypto.SignId(
		quorumType,
		bls12381.ReverseBytes(quorumHash),
		bls12381.ReverseBytes(requestID),
		bls12381.ReverseBytes(messageHash),
	)
	privateKey, err := mc.localPV.GetPrivateKey(quorumHash)
	if err != nil {
		panic(err)
	}

	sign, err := privateKey.SignDigest(signID)
	if err != nil {
		panic(err)
	}

	res := btcjson.QuorumSignResult{
		LLMQType:   int(quorumType),
		QuorumHash: quorumHash.String(),
		ID:         hex.EncodeToString(requestID),
		MsgHash:    hex.EncodeToString(messageHash),
		SignHash:   hex.EncodeToString(signID),
		Signature:  hex.EncodeToString(sign),
	}
	return &res, nil
}

func (mc *DashCoreMockClient) QuorumVerify(quorumType btcjson.LLMQType, requestID bytes.HexBytes, messageHash bytes.HexBytes, signature bytes.HexBytes, quorumHash crypto.QuorumHash) (bool, error) {
	signID := crypto.SignId(
		quorumType,
		bls12381.ReverseBytes(quorumHash),
		bls12381.ReverseBytes(requestID),
		bls12381.ReverseBytes(messageHash),
	)
	thresholdPublicKey, err := mc.localPV.GetThresholdPublicKey(quorumHash)
	if err != nil {
		panic(err)
	}

	signatureVerified := thresholdPublicKey.VerifySignatureDigest(signID, signature)

	return signatureVerified, nil
}
