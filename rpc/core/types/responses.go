package core_types

import (
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-p2p"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
)

type ResultBlockchainInfo struct {
	LastHeight int                `json:"last_height"`
	BlockMetas []*types.BlockMeta `json:"block_metas"`
}

type ResultGetBlock struct {
	BlockMeta *types.BlockMeta `json:"block_meta"`
	Block     *types.Block     `json:"block"`
}

type ResultStatus struct {
	NodeInfo          *p2p.NodeInfo `json:"node_info"`
	PubKey            crypto.PubKey `json:"pub_key"`
	LatestBlockHash   []byte        `json:"latest_block_hash"`
	LatestBlockHeight int           `json:"latest_block_height"`
	LatestBlockTime   int64         `json:"latest_block_time"` // nano
}

type ResultNetInfo struct {
	Listening bool     `json:"listening"`
	Listeners []string `json:"listeners"`
	Peers     []Peer   `json:"peers"`
}

type Peer struct {
	p2p.NodeInfo `json:"node_info"`
	IsOutbound   bool `json:"is_outbound"`
}

type ResultListValidators struct {
	BlockHeight int                `json:"block_height"`
	Validators  []*types.Validator `json:"validators"`
}

type ResultDumpConsensusState struct {
	RoundState      string   `json:"round_state"`
	PeerRoundStates []string `json:"peer_round_states"`
}

type ResultBroadcastTx struct {
}

type ResultListUnconfirmedTxs struct {
	N   int        `json:"n_txs"`
	Txs []types.Tx `json:"txs"`
}

type ResultGenesis struct {
	Genesis *types.GenesisDoc `json:"genesis"`
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
	ResultTypeBlockchainInfo     = byte(0x05)
	ResultTypeGetBlock           = byte(0x06)
	ResultTypeStatus             = byte(0x07)
	ResultTypeNetInfo            = byte(0x08)
	ResultTypeListValidators     = byte(0x09)
	ResultTypeDumpConsensusState = byte(0x0A)
	ResultTypeBroadcastTx        = byte(0x0E)
	ResultTypeListUnconfirmedTxs = byte(0x0F)
	ResultTypeGenesis            = byte(0x11)
	ResultTypeEvent              = byte(0x13) // so websockets can respond to rpc functions
)

type Result interface{}

// for wire.readReflect
var _ = wire.RegisterInterface(
	struct{ Result }{},
	wire.ConcreteType{&ResultBlockchainInfo{}, ResultTypeBlockchainInfo},
	wire.ConcreteType{&ResultGetBlock{}, ResultTypeGetBlock},
	wire.ConcreteType{&ResultStatus{}, ResultTypeStatus},
	wire.ConcreteType{&ResultNetInfo{}, ResultTypeNetInfo},
	wire.ConcreteType{&ResultListValidators{}, ResultTypeListValidators},
	wire.ConcreteType{&ResultDumpConsensusState{}, ResultTypeDumpConsensusState},
	wire.ConcreteType{&ResultBroadcastTx{}, ResultTypeBroadcastTx},
	wire.ConcreteType{&ResultListUnconfirmedTxs{}, ResultTypeListUnconfirmedTxs},
	wire.ConcreteType{&ResultGenesis{}, ResultTypeGenesis},
	wire.ConcreteType{&ResultEvent{}, ResultTypeEvent},
)
