package core_types

import (
	"strings"
	"time"

	abci "github.com/tendermint/abci/types"
	crypto "github.com/tendermint/go-crypto"
	cmn "github.com/tendermint/tmlibs/common"

	cstypes "github.com/tendermint/tendermint/consensus/types"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

type ResultBlockchainInfo struct {
	LastHeight int64              `json:"last_height"`
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
	// SignedHeader is header and commit, embedded so we only have
	// one level in the json output
	types.SignedHeader
	CanonicalCommit bool `json:"canonical"`
}

type ResultBlockResults struct {
	Height  int64                `json:"height"`
	Results *state.ABCIResponses `json:"results"`
}

// NewResultCommit is a helper to initialize the ResultCommit with
// the embedded struct
func NewResultCommit(header *types.Header, commit *types.Commit,
	canonical bool) *ResultCommit {

	return &ResultCommit{
		SignedHeader: types.SignedHeader{
			Header: header,
			Commit: commit,
		},
		CanonicalCommit: canonical,
	}
}

type ValidatorStatus struct {
	VotingPower int64 `json:"voting_power"`
}

type ResultStatus struct {
	NodeInfo          p2p.NodeInfo    `json:"node_info"`
	PubKey            crypto.PubKey   `json:"pub_key"`
	LatestBlockHash   cmn.HexBytes    `json:"latest_block_hash"`
	LatestAppHash     cmn.HexBytes    `json:"latest_app_hash"`
	LatestBlockHeight int64           `json:"latest_block_height"`
	LatestBlockTime   time.Time       `json:"latest_block_time"`
	Syncing           bool            `json:"syncing"`
	ValidatorStatus   ValidatorStatus `json:"validator_status,omitempty"`
}

func (s *ResultStatus) TxIndexEnabled() bool {
	if s == nil {
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

type ResultDialPeers struct {
	Log string `json:"log"`
}

type Peer struct {
	p2p.NodeInfo     `json:"node_info"`
	p2p.ID           `json:"node_id"`
	IsOutbound       bool                 `json:"is_outbound"`
	ConnectionStatus p2p.ConnectionStatus `json:"connection_status"`
}

type ResultValidators struct {
	BlockHeight int64              `json:"block_height"`
	Validators  []*types.Validator `json:"validators"`
}

type ResultDumpConsensusState struct {
	RoundState      *cstypes.RoundState                `json:"round_state"`
	PeerRoundStates map[p2p.ID]*cstypes.PeerRoundState `json:"peer_round_states"`
}

type ResultBroadcastTx struct {
	Code uint32       `json:"code"`
	Data cmn.HexBytes `json:"data"`
	Log  string       `json:"log"`

	Hash cmn.HexBytes `json:"hash"`
}

type ResultBroadcastTxCommit struct {
	CheckTx   abci.ResponseCheckTx   `json:"check_tx"`
	DeliverTx abci.ResponseDeliverTx `json:"deliver_tx"`
	Hash      cmn.HexBytes           `json:"hash"`
	Height    int64                  `json:"height"`
}

type ResultTx struct {
	Hash     cmn.HexBytes           `json:"hash"`
	Height   int64                  `json:"height"`
	Index    uint32                 `json:"index"`
	TxResult abci.ResponseDeliverTx `json:"tx_result"`
	Tx       types.Tx               `json:"tx"`
	Proof    types.TxProof          `json:"proof,omitempty"`
}

type ResultUnconfirmedTxs struct {
	N   int        `json:"n_txs"`
	Txs []types.Tx `json:"txs"`
}

type ResultABCIInfo struct {
	Response abci.ResponseInfo `json:"response"`
}

type ResultABCIQuery struct {
	Response abci.ResponseQuery `json:"response"`
}

type ResultUnsafeFlushMempool struct{}

type ResultUnsafeProfile struct{}

type ResultSubscribe struct{}

type ResultUnsubscribe struct{}

type ResultEvent struct {
	Query string            `json:"query"`
	Data  types.TMEventData `json:"data"`
}

type ResultHealth struct{}
