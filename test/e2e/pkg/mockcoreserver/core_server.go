package mockcoreserver

import (
	"encoding/hex"
	"strconv"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/privval"
)

// CoreServer is an interface of a mock core-server
type CoreServer interface {
	QuorumInfo(cmd btcjson.QuorumCmd) btcjson.QuorumInfoResult
	QuorumSign(cmd btcjson.QuorumCmd) btcjson.QuorumSignResult
	MasternodeStatus(cmd btcjson.MasternodeCmd) btcjson.MasternodeStatusResult
	GetNetworkInfo(cmd btcjson.GetNetworkInfoCmd) btcjson.GetNetworkInfoResult
}

// MockCoreServer is an implementation of a mock core-server
type MockCoreServer struct {
	ChainID  string
	LLMQType btcjson.LLMQType
	FilePV   *privval.FilePV
}

// QuorumInfo returns a quorum-info result
func (c *MockCoreServer) QuorumInfo(cmd btcjson.QuorumCmd) btcjson.QuorumInfoResult {
	var members []btcjson.QuorumMember
	proTxHash, err := c.FilePV.GetProTxHash()
	if err != nil {
		panic(err)
	}
	quorumHash := strVal(cmd.QuorumHash)
	qq := bytes.HexBytes(quorumHash)
	pk, err := c.FilePV.GetPubKey(qq)
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
	tpk, err := c.FilePV.GetThresholdPublicKey(qq)
	if err != nil {
		panic(err)
	}
	height, err := c.FilePV.GetHeight(qq)
	if err != nil {
		panic(err)
	}
	return btcjson.QuorumInfoResult{
		Height:          uint32(height),
		Type:            strconv.Itoa(int(c.LLMQType)),
		QuorumHash:      quorumHash,
		Members:         members,
		QuorumPublicKey: tpk.String(),
	}
}

// QuorumSign returns a quorum-sign result
func (c *MockCoreServer) QuorumSign(cmd btcjson.QuorumCmd) btcjson.QuorumSignResult {
	reqID, err := hex.DecodeString(strVal(cmd.RequestID))
	if err != nil {
		panic(err)
	}
	msgHash, err := hex.DecodeString(strVal(cmd.MessageHash))
	if err != nil {
		panic(err)
	}

	quorumHashBytes, err := hex.DecodeString(*cmd.QuorumHash)
	if err != nil {
		panic(err)
	}
	quorumHash := crypto.QuorumHash(quorumHashBytes)

	signID := crypto.SignId(
		*cmd.LLMQType,
		bls12381.ReverseBytes(quorumHash),
		bls12381.ReverseBytes(reqID),
		bls12381.ReverseBytes(msgHash),
	)
	privateKey, err := c.FilePV.Key.PrivateKeyForQuorumHash(quorumHash)
	if err != nil {
		panic(err)
	}

	sign, err := privateKey.SignDigest(signID)
	if err != nil {
		panic(err)
	}

	res := btcjson.QuorumSignResult{
		LLMQType:   int(c.LLMQType),
		QuorumHash: quorumHash.String(),
		ID:         hex.EncodeToString(reqID),
		MsgHash:    hex.EncodeToString(msgHash),
		SignHash:   hex.EncodeToString(signID),
		Signature:  hex.EncodeToString(sign),
	}
	return res
}

// MasternodeStatus returns a masternode-status result
func (c *MockCoreServer) MasternodeStatus(_ btcjson.MasternodeCmd) btcjson.MasternodeStatusResult {
	proTxHash, err := c.FilePV.GetProTxHash()
	if err != nil {
		panic(err)
	}
	return btcjson.MasternodeStatusResult{
		ProTxHash: proTxHash.String(),
	}
}

// GetNetworkInfo returns network-info result
func (c *MockCoreServer) GetNetworkInfo(_ btcjson.GetNetworkInfoCmd) btcjson.GetNetworkInfoResult {
	return btcjson.GetNetworkInfoResult{}
}

// StaticCoreServer is a mock of core-server with static result data
type StaticCoreServer struct {
	QuorumInfoResult       btcjson.QuorumInfoResult
	QuorumSignResult       btcjson.QuorumSignResult
	MasternodeStatusResult btcjson.MasternodeStatusResult
	GetNetworkInfoResult   btcjson.GetNetworkInfoResult
}

// Quorum returns constant quorum-info result
func (c *StaticCoreServer) QuorumInfo(_ btcjson.QuorumCmd) btcjson.QuorumInfoResult {
	return c.QuorumInfoResult
}

// Quorum returns constant quorum-sign result
func (c *StaticCoreServer) QuorumSign(_ btcjson.QuorumCmd) btcjson.QuorumSignResult {
	return c.QuorumSignResult
}

// MasternodeStatus returns constant masternode-status result
func (c *StaticCoreServer) MasternodeStatus(_ btcjson.MasternodeCmd) btcjson.MasternodeStatusResult {
	return c.MasternodeStatusResult
}

// GetNetworkInfo returns constant network-info result
func (c *StaticCoreServer) GetNetworkInfo(_ btcjson.GetNetworkInfoCmd) btcjson.GetNetworkInfoResult {
	return c.GetNetworkInfoResult
}

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
