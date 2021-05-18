package mockcoreserver

import (
	"github.com/dashevo/dashd-go/btcjson"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/privval"
)

// CoreServer is an interface of a mock core-server
type CoreServer interface {
	Quorum(cmd btcjson.QuorumCmd) btcjson.QuorumInfoResult
	MasternodeStatus(cmd btcjson.MasternodeCmd) btcjson.MasternodeStatusResult
	GetNetworkInfo(cmd btcjson.GetNetworkInfoCmd) btcjson.GetNetworkInfoResult
}

// MockCoreServer is an implementation of a mock core-server
type MockCoreServer struct {
	ChainID    string
	LLMQType   btcjson.LLMQType
	QuorumHash crypto.QuorumHash
	FilePV     *privval.FilePV
}

// Quorum returns a quorum info result
func (c *MockCoreServer) Quorum(cmd btcjson.QuorumCmd) btcjson.QuorumInfoResult {
	// todo needs to use config data for a result
	return btcjson.QuorumInfoResult{
		Height:     1010,
		Type:       "llmq_50_60",
		QuorumHash: c.QuorumHash.String(),
		MinedBlock: "",
		Members: []btcjson.QuorumMember{
			{
				ProTxHash:      "6c91363d97b286e921afb5cf7672c88a2f1614d36d32058c34bef8b44e026007",
				PubKeyOperator: "81749ba8363e5c03e9d6318b0491e38305cf59d9d57cea2295a86ecfa696622571f266c28bacc78666e8b9b0fb2b3121",
				Valid:          true,
				PubKeyShare:    "83349ba8363e5c03e9d6318b0491e38305cf59d9d57cea2295a86ecfa696622571f266c28bacc78666e8b9b0fb2b3123",
			},
		},
		QuorumPublicKey: "0644ff153b9b92c6a59e2adf4ef0b9836f7f6af05fe432ffdcb69bc9e300a2a70af4a8d9fc61323f6b81074d740033d2",
		SecretKeyShare:  "3da0d8f532309660f7f44aa0ed42c1569773b39c70f5771ce5604be77e50759e",
	}
}

// MasternodeStatus returns a masternode-status result
func (c *MockCoreServer) MasternodeStatus(_ btcjson.MasternodeCmd) btcjson.MasternodeStatusResult {
	proTxHash, _ := c.FilePV.GetProTxHash()
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
	MasternodeStatusResult btcjson.MasternodeStatusResult
	GetNetworkInfoResult   btcjson.GetNetworkInfoResult
}

// Quorum returns constant quorum-info result
func (c *StaticCoreServer) Quorum(_ btcjson.QuorumCmd) btcjson.QuorumInfoResult {
	return c.QuorumInfoResult
}

// MasternodeStatus returns constant masternode-status result
func (c *StaticCoreServer) MasternodeStatus(_ btcjson.MasternodeCmd) btcjson.MasternodeStatusResult {
	return c.MasternodeStatusResult
}

// GetNetworkInfo returns constant network-info result
func (c *StaticCoreServer) GetNetworkInfo(_ btcjson.GetNetworkInfoCmd) btcjson.GetNetworkInfoResult {
	return c.GetNetworkInfoResult
}
