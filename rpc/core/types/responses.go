package core_types

import (
	"strings"

	abci "github.com/tendermint/abci/types"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-wire/data"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tendermint/types"
)

type ResultBlockchainInfo struct {
	LastHeight int                `json:"last_height"`
	BlockMetas []*types.BlockMeta `json:"block_metas"`
}

type ResultGenesis struct {
	Genesis *types.GenesisDoc `json:"genesis"`
}

type ResultBlock struct {
	BlockMeta *types.BlockMeta `json:"block_meta"`
	Block     *types.Block     `json:"block"`
}

type ResultCommit struct {
	Header          *types.Header `json:"header"`
	Commit          *types.Commit `json:"commit"`
	CanonicalCommit bool          `json:"canonical"`
}

type ResultStatus struct {
	NodeInfo          *p2p.NodeInfo `json:"node_info"`
	PubKey            crypto.PubKey `json:"pub_key"`
	LatestBlockHash   data.Bytes    `json:"latest_block_hash"`
	LatestAppHash     data.Bytes    `json:"latest_app_hash"`
	LatestBlockHeight int           `json:"latest_block_height"`
	LatestBlockTime   int64         `json:"latest_block_time"` // nano
}

func (s *ResultStatus) TxIndexEnabled() bool {
	if s == nil || s.NodeInfo == nil {
		return false
	}
	for _, s := range s.NodeInfo.Other {
		info := strings.Split(s, "=")
		if len(info) == 2 && info[0] == "tx_index" {
			return info[1] == "on"
		}
	}
	return false
}

type ResultNetInfo struct {
	Listening bool     `json:"listening"`
	Listeners []string `json:"listeners"`
	Peers     []Peer   `json:"peers"`
}

type ResultDialSeeds struct {
	Log string `json:"log"`
}

type Peer struct {
	p2p.NodeInfo     `json:"node_info"`
	IsOutbound       bool                 `json:"is_outbound"`
	ConnectionStatus p2p.ConnectionStatus `json:"connection_status"`
}

type ResultValidators struct {
	BlockHeight int                `json:"block_height"`
	Validators  []*types.Validator `json:"validators"`
}

type ResultDumpConsensusState struct {
	RoundState      string   `json:"round_state"`
	PeerRoundStates []string `json:"peer_round_states"`
}

type ResultBroadcastTx struct {
	Code abci.CodeType `json:"code"`
	Data data.Bytes    `json:"data"`
	Log  string        `json:"log"`

	Hash data.Bytes `json:"hash"`
}

type ResultBroadcastTxCommit struct {
	CheckTx   abci.Result `json:"check_tx"`
	DeliverTx abci.Result `json:"deliver_tx"`
	Hash      data.Bytes  `json:"hash"`
	Height    int         `json:"height"`
}

type ResultTx struct {
	Height   int           `json:"height"`
	Index    int           `json:"index"`
	TxResult abci.Result   `json:"tx_result"`
	Tx       types.Tx      `json:"tx"`
	Proof    types.TxProof `json:"proof,omitempty"`
}

type ResultUnconfirmedTxs struct {
	N   int        `json:"n_txs"`
	Txs []types.Tx `json:"txs"`
}

type ResultABCIInfo struct {
	Response abci.ResponseInfo `json:"response"`
}

type ResultABCIQuery struct {
	*abci.ResultQuery `json:"response"`
}

type ResultUnsafeFlushMempool struct{}

type ResultUnsafeSetConfig struct{}

type ResultUnsafeProfile struct{}

type ResultSubscribe struct {
}

type ResultUnsubscribe struct {
}

type ResultEvent struct {
	Name string            `json:"name"`
	Data types.TMEventData `json:"data"`
}

//----------------------------------------
// response & result types

const (
	// 0x0 bytes are for the blockchain
	ResultTypeGenesis        = byte(0x01)
	ResultTypeBlockchainInfo = byte(0x02)
	ResultTypeBlock          = byte(0x03)
	ResultTypeCommit         = byte(0x04)

	// 0x2 bytes are for the network
	ResultTypeStatus    = byte(0x20)
	ResultTypeNetInfo   = byte(0x21)
	ResultTypeDialSeeds = byte(0x22)

	// 0x4 bytes are for the consensus
	ResultTypeValidators         = byte(0x40)
	ResultTypeDumpConsensusState = byte(0x41)

	// 0x6 bytes are for txs / the application
	ResultTypeBroadcastTx       = byte(0x60)
	ResultTypeUnconfirmedTxs    = byte(0x61)
	ResultTypeBroadcastTxCommit = byte(0x62)
	ResultTypeTx                = byte(0x63)

	// 0x7 bytes are for querying the application
	ResultTypeABCIQuery = byte(0x70)
	ResultTypeABCIInfo  = byte(0x71)

	// 0x8 bytes are for events
	ResultTypeSubscribe   = byte(0x80)
	ResultTypeUnsubscribe = byte(0x81)
	ResultTypeEvent       = byte(0x82)

	// 0xa bytes for testing
	ResultTypeUnsafeSetConfig        = byte(0xa0)
	ResultTypeUnsafeStartCPUProfiler = byte(0xa1)
	ResultTypeUnsafeStopCPUProfiler  = byte(0xa2)
	ResultTypeUnsafeWriteHeapProfile = byte(0xa3)
	ResultTypeUnsafeFlushMempool     = byte(0xa4)
)

const (
	// for the blockchain
	ResultNameGenesis        = "genesis"
	ResultNameBlockchainInfo = "info"
	ResultNameBlock          = "block"
	ResultNameCommit         = "commit"

	// for the network
	ResultNameStatus    = "status"
	ResultNameNetInfo   = "netinfo"
	ResultNameDialSeeds = "dialseeds"

	// for the consensus
	ResultNameValidators         = "validators"
	ResultNameDumpConsensusState = "consensus"

	// for txs / the application
	ResultNameBroadcastTx       = "broadcast_tx"
	ResultNameUnconfirmedTxs    = "unconfirmed_tx"
	ResultNameBroadcastTxCommit = "broadcast_tx_commit"
	ResultNameTx                = "tx"

	// for querying the application
	ResultNameABCIQuery = "abci_query"
	ResultNameABCIInfo  = "abci_info"

	// for events
	ResultNameSubscribe   = "subscribe"
	ResultNameUnsubscribe = "unsubscribe"
	ResultNameEvent       = "event"

	// for testing
	ResultNameUnsafeSetConfig        = "unsafe_set_config"
	ResultNameUnsafeStartCPUProfiler = "unsafe_start_profiler"
	ResultNameUnsafeStopCPUProfiler  = "unsafe_stop_profiler"
	ResultNameUnsafeWriteHeapProfile = "unsafe_write_heap"
	ResultNameUnsafeFlushMempool     = "unsafe_flush_mempool"
)

type TMResultInner interface {
	rpctypes.Result
}

type TMResult struct {
	TMResultInner
}

func (tmr TMResult) MarshalJSON() ([]byte, error) {
	return tmResultMapper.ToJSON(tmr.TMResultInner)
}

func (tmr *TMResult) UnmarshalJSON(data []byte) (err error) {
	parsed, err := tmResultMapper.FromJSON(data)
	if err == nil && parsed != nil {
		tmr.TMResultInner = parsed.(TMResultInner)
	}
	return
}

func (tmr TMResult) Unwrap() TMResultInner {
	tmrI := tmr.TMResultInner
	for wrap, ok := tmrI.(TMResult); ok; wrap, ok = tmrI.(TMResult) {
		tmrI = wrap.TMResultInner
	}
	return tmrI
}

func (tmr TMResult) Empty() bool {
	return tmr.TMResultInner == nil
}

var tmResultMapper = data.NewMapper(TMResult{}).
	RegisterImplementation(&ResultGenesis{}, ResultNameGenesis, ResultTypeGenesis).
	RegisterImplementation(&ResultBlockchainInfo{}, ResultNameBlockchainInfo, ResultTypeBlockchainInfo).
	RegisterImplementation(&ResultBlock{}, ResultNameBlock, ResultTypeBlock).
	RegisterImplementation(&ResultCommit{}, ResultNameCommit, ResultTypeCommit).
	RegisterImplementation(&ResultStatus{}, ResultNameStatus, ResultTypeStatus).
	RegisterImplementation(&ResultNetInfo{}, ResultNameNetInfo, ResultTypeNetInfo).
	RegisterImplementation(&ResultDialSeeds{}, ResultNameDialSeeds, ResultTypeDialSeeds).
	RegisterImplementation(&ResultValidators{}, ResultNameValidators, ResultTypeValidators).
	RegisterImplementation(&ResultDumpConsensusState{}, ResultNameDumpConsensusState, ResultTypeDumpConsensusState).
	RegisterImplementation(&ResultBroadcastTx{}, ResultNameBroadcastTx, ResultTypeBroadcastTx).
	RegisterImplementation(&ResultBroadcastTxCommit{}, ResultNameBroadcastTxCommit, ResultTypeBroadcastTxCommit).
	RegisterImplementation(&ResultTx{}, ResultNameTx, ResultTypeTx).
	RegisterImplementation(&ResultUnconfirmedTxs{}, ResultNameUnconfirmedTxs, ResultTypeUnconfirmedTxs).
	RegisterImplementation(&ResultSubscribe{}, ResultNameSubscribe, ResultTypeSubscribe).
	RegisterImplementation(&ResultUnsubscribe{}, ResultNameUnsubscribe, ResultTypeUnsubscribe).
	RegisterImplementation(&ResultEvent{}, ResultNameEvent, ResultTypeEvent).
	RegisterImplementation(&ResultUnsafeSetConfig{}, ResultNameUnsafeSetConfig, ResultTypeUnsafeSetConfig).
	RegisterImplementation(&ResultUnsafeProfile{}, ResultNameUnsafeStartCPUProfiler, ResultTypeUnsafeStartCPUProfiler).
	RegisterImplementation(&ResultUnsafeProfile{}, ResultNameUnsafeStopCPUProfiler, ResultTypeUnsafeStopCPUProfiler).
	RegisterImplementation(&ResultUnsafeProfile{}, ResultNameUnsafeWriteHeapProfile, ResultTypeUnsafeWriteHeapProfile).
	RegisterImplementation(&ResultUnsafeFlushMempool{}, ResultNameUnsafeFlushMempool, ResultTypeUnsafeFlushMempool).
	RegisterImplementation(&ResultABCIQuery{}, ResultNameABCIQuery, ResultTypeABCIQuery).
	RegisterImplementation(&ResultABCIInfo{}, ResultNameABCIInfo, ResultTypeABCIInfo)
