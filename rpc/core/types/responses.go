package core_types

import (
	"github.com/tendermint/tendermint/account"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

type ResponseGetStorage struct {
	Key   []byte `json:"key"`
	Value []byte `json:"value"`
}

type ResponseCall struct {
	Return  []byte `json:"return"`
	GasUsed uint64 `json:"gas_used"`
	// TODO ...
}

type ResponseListAccounts struct {
	BlockHeight uint               `json:"block_height"`
	Accounts    []*account.Account `json:"accounts"`
}

type StorageItem struct {
	Key   []byte `json:"key"`
	Value []byte `json:"value"`
}

type ResponseDumpStorage struct {
	StorageRoot  []byte        `json:"storage_root"`
	StorageItems []StorageItem `json:"storage_items"`
}

type ResponseBlockchainInfo struct {
	LastHeight uint               `json:"last_height"`
	BlockMetas []*types.BlockMeta `json:"block_metas"`
}

type ResponseGetBlock struct {
	BlockMeta *types.BlockMeta `json:"block_meta"`
	Block     *types.Block     `json:"block"`
}

type Receipt struct {
	TxHash          []byte `json:"tx_hash"`
	CreatesContract uint8  `json:"creates_contract"`
	ContractAddr    []byte `json:"contract_addr"`
}

type ResponseStatus struct {
	Moniker           string         `json:"moniker"`
	ChainID           string         `json:"chain_id"`
	Version           string         `json:"version"`
	GenesisHash       []byte         `json:"genesis_hash"`
	PubKey            account.PubKey `json:"pub_key"`
	LatestBlockHash   []byte         `json:"latest_block_hash"`
	LatestBlockHeight uint           `json:"latest_block_height"`
	LatestBlockTime   int64          `json:"latest_block_time"` // nano
}

type ResponseNetInfo struct {
	Listening bool     `json:"listening"`
	Listeners []string `json:"listeners"`
	Peers     []Peer   `json:"peers"`
}

type Peer struct {
	types.NodeInfo `json:"node_info"`
	IsOutbound     bool `json:"is_outbound"`
}

type ResponseListValidators struct {
	BlockHeight         uint            `json:"block_height"`
	BondedValidators    []*sm.Validator `json:"bonded_validators"`
	UnbondingValidators []*sm.Validator `json:"unbonding_validators"`
}

type ResponseDumpConsensusState struct {
	RoundState      string   `json:"round_state"`
	PeerRoundStates []string `json:"peer_round_states"`
}

type ResponseListNames struct {
	BlockHeight uint                  `json:"block_height"`
	Names       []*types.NameRegEntry `json:"names"`
}
