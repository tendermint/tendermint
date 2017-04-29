package core_types

import (
	"strings"

	abci "github.com/tendermint/abci/types"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-wire/data"

	"github.com/tendermint/tendermint/p2p"
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

type ResultUnsafeProfile struct{}

type ResultSubscribe struct{}

type ResultUnsubscribe struct{}

type ResultEvent struct {
	Name string            `json:"name"`
	Data types.TMEventData `json:"data"`
}
