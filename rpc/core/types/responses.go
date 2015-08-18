package core_types

import (
	acm "github.com/tendermint/tendermint/account"
	stypes "github.com/tendermint/tendermint/state/types"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/wire"
)

type ResultGetStorage struct {
	Key   []byte `json:"key"`
	Value []byte `json:"value"`
}

type ResultCall struct {
	Return  []byte `json:"return"`
	GasUsed int64  `json:"gas_used"`
	// TODO ...
}

type ResultListAccounts struct {
	BlockHeight int            `json:"block_height"`
	Accounts    []*acm.Account `json:"accounts"`
}

type ResultDumpStorage struct {
	StorageRoot  []byte        `json:"storage_root"`
	StorageItems []StorageItem `json:"storage_items"`
}

type StorageItem struct {
	Key   []byte `json:"key"`
	Value []byte `json:"value"`
}

type ResultBlockchainInfo struct {
	LastHeight int                `json:"last_height"`
	BlockMetas []*types.BlockMeta `json:"block_metas"`
}

type ResultGetBlock struct {
	BlockMeta *types.BlockMeta `json:"block_meta"`
	Block     *types.Block     `json:"block"`
}

type ResultStatus struct {
	NodeInfo          *types.NodeInfo `json:"node_info"`
	GenesisHash       []byte          `json:"genesis_hash"`
	PubKey            acm.PubKey      `json:"pub_key"`
	LatestBlockHash   []byte          `json:"latest_block_hash"`
	LatestBlockHeight int             `json:"latest_block_height"`
	LatestBlockTime   int64           `json:"latest_block_time"` // nano
}

type ResultNetInfo struct {
	Listening bool     `json:"listening"`
	Listeners []string `json:"listeners"`
	Peers     []Peer   `json:"peers"`
}

type Peer struct {
	types.NodeInfo `json:"node_info"`
	IsOutbound     bool `json:"is_outbound"`
}

type ResultListValidators struct {
	BlockHeight         int                `json:"block_height"`
	BondedValidators    []*types.Validator `json:"bonded_validators"`
	UnbondingValidators []*types.Validator `json:"unbonding_validators"`
}

type ResultDumpConsensusState struct {
	RoundState      string   `json:"round_state"`
	PeerRoundStates []string `json:"peer_round_states"`
}

type ResultListNames struct {
	BlockHeight int                   `json:"block_height"`
	Names       []*types.NameRegEntry `json:"names"`
}

type ResultGenPrivAccount struct {
	PrivAccount *acm.PrivAccount `json:"priv_account"`
}

type ResultGetAccount struct {
	Account *acm.Account `json:"account"`
}

type ResultBroadcastTx struct {
	Receipt Receipt `json:"receipt"`
}

type Receipt struct {
	TxHash          []byte `json:"tx_hash"`
	CreatesContract uint8  `json:"creates_contract"`
	ContractAddr    []byte `json:"contract_addr"`
}

type ResultListUnconfirmedTxs struct {
	Txs []types.Tx `json:"txs"`
}

type ResultGetName struct {
	Entry *types.NameRegEntry `json:"entry"`
}

type ResultGenesis struct {
	Genesis *stypes.GenesisDoc `json:"genesis"`
}

type ResultSignTx struct {
	Tx types.Tx `json:"tx"`
}

type ResultEvent struct {
	Event string          `json:"event"`
	Data  types.EventData `json:"data"`
}

//----------------------------------------
// response & result types

type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Result  Result `json:"result"`
	Error   string `json:"error"`
}

const (
	ResultTypeGetStorage         = byte(0x01)
	ResultTypeCall               = byte(0x02)
	ResultTypeListAccounts       = byte(0x03)
	ResultTypeDumpStorage        = byte(0x04)
	ResultTypeBlockchainInfo     = byte(0x05)
	ResultTypeGetBlock           = byte(0x06)
	ResultTypeStatus             = byte(0x07)
	ResultTypeNetInfo            = byte(0x08)
	ResultTypeListValidators     = byte(0x09)
	ResultTypeDumpConsensusState = byte(0x0A)
	ResultTypeListNames          = byte(0x0B)
	ResultTypeGenPrivAccount     = byte(0x0C)
	ResultTypeGetAccount         = byte(0x0D)
	ResultTypeBroadcastTx        = byte(0x0E)
	ResultTypeListUnconfirmedTxs = byte(0x0F)
	ResultTypeGetName            = byte(0x10)
	ResultTypeGenesis            = byte(0x11)
	ResultTypeSignTx             = byte(0x12)
	ResultTypeEvent              = byte(0x13) // so websockets can respond to rpc functions
)

type Result interface{}

// for wire.readReflect
var _ = wire.RegisterInterface(
	struct{ Result }{},
	wire.ConcreteType{&ResultGetStorage{}, ResultTypeGetStorage},
	wire.ConcreteType{&ResultCall{}, ResultTypeCall},
	wire.ConcreteType{&ResultListAccounts{}, ResultTypeListAccounts},
	wire.ConcreteType{&ResultDumpStorage{}, ResultTypeDumpStorage},
	wire.ConcreteType{&ResultBlockchainInfo{}, ResultTypeBlockchainInfo},
	wire.ConcreteType{&ResultGetBlock{}, ResultTypeGetBlock},
	wire.ConcreteType{&ResultStatus{}, ResultTypeStatus},
	wire.ConcreteType{&ResultNetInfo{}, ResultTypeNetInfo},
	wire.ConcreteType{&ResultListValidators{}, ResultTypeListValidators},
	wire.ConcreteType{&ResultDumpConsensusState{}, ResultTypeDumpConsensusState},
	wire.ConcreteType{&ResultListNames{}, ResultTypeListNames},
	wire.ConcreteType{&ResultGenPrivAccount{}, ResultTypeGenPrivAccount},
	wire.ConcreteType{&ResultGetAccount{}, ResultTypeGetAccount},
	wire.ConcreteType{&ResultBroadcastTx{}, ResultTypeBroadcastTx},
	wire.ConcreteType{&ResultListUnconfirmedTxs{}, ResultTypeListUnconfirmedTxs},
	wire.ConcreteType{&ResultGetName{}, ResultTypeGetName},
	wire.ConcreteType{&ResultGenesis{}, ResultTypeGenesis},
	wire.ConcreteType{&ResultSignTx{}, ResultTypeSignTx},
	wire.ConcreteType{&ResultEvent{}, ResultTypeEvent},
)
