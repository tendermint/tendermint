package coretypes

import (
	"encoding/json"
	"errors"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/bytes"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

// List of standardized errors used across RPC
var (
	ErrZeroOrNegativePerPage  = errors.New("zero or negative per_page")
	ErrPageOutOfRange         = errors.New("page should be within range")
	ErrZeroOrNegativeHeight   = errors.New("height must be greater than zero")
	ErrHeightExceedsChainHead = errors.New("height must be less than or equal to the head of the node's blockchain")
	ErrHeightNotAvailable     = errors.New("height is not available")
	// ErrInvalidRequest is used as a wrapper to cover more specific cases where the user has
	// made an invalid request
	ErrInvalidRequest = errors.New("invalid request")
)

// List of blocks
type ResultBlockchainInfo struct {
	LastHeight int64              `json:"last_height"`
	BlockMetas []*types.BlockMeta `json:"block_metas"`
}

// Genesis file
type ResultGenesis struct {
	Genesis *types.GenesisDoc `json:"genesis"`
}

// ResultGenesisChunk is the output format for the chunked/paginated
// interface. These chunks are produced by converting the genesis
// document to JSON and then splitting the resulting payload into
// 16 megabyte blocks and then base64 encoding each block.
type ResultGenesisChunk struct {
	ChunkNumber int    `json:"chunk"`
	TotalChunks int    `json:"total"`
	Data        string `json:"data"`
}

// Single block (with meta)
type ResultBlock struct {
	BlockID types.BlockID `json:"block_id"`
	Block   *types.Block  `json:"block"`
}

// Commit and Header
type ResultCommit struct {
	types.SignedHeader `json:"signed_header"`
	CanonicalCommit    bool `json:"canonical"`
}

// ABCI results from a block
type ResultBlockResults struct {
	Height                int64                     `json:"height"`
	TxsResults            []*abci.ResponseDeliverTx `json:"txs_results"`
	TotalGasUsed          int64                     `json:"total_gas_used"`
	BeginBlockEvents      []abci.Event              `json:"begin_block_events"`
	EndBlockEvents        []abci.Event              `json:"end_block_events"`
	ValidatorUpdates      []abci.ValidatorUpdate    `json:"validator_updates"`
	ConsensusParamUpdates *tmproto.ConsensusParams  `json:"consensus_param_updates"`
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

// Info about the node's syncing state
type SyncInfo struct {
	LatestBlockHash   bytes.HexBytes `json:"latest_block_hash"`
	LatestAppHash     bytes.HexBytes `json:"latest_app_hash"`
	LatestBlockHeight int64          `json:"latest_block_height"`
	LatestBlockTime   time.Time      `json:"latest_block_time"`

	EarliestBlockHash   bytes.HexBytes `json:"earliest_block_hash"`
	EarliestAppHash     bytes.HexBytes `json:"earliest_app_hash"`
	EarliestBlockHeight int64          `json:"earliest_block_height"`
	EarliestBlockTime   time.Time      `json:"earliest_block_time"`

	MaxPeerBlockHeight int64 `json:"max_peer_block_height"`

	CatchingUp bool `json:"catching_up"`

	TotalSyncedTime time.Duration `json:"total_synced_time"`
	RemainingTime   time.Duration `json:"remaining_time"`

	TotalSnapshots      int64         `json:"total_snapshots"`
	ChunkProcessAvgTime time.Duration `json:"chunk_process_avg_time"`
	SnapshotHeight      int64         `json:"snapshot_height"`
	SnapshotChunksCount int64         `json:"snapshot_chunks_count"`
	SnapshotChunksTotal int64         `json:"snapshot_chunks_total"`
	BackFilledBlocks    int64         `json:"backfilled_blocks"`
	BackFillBlocksTotal int64         `json:"backfill_blocks_total"`
}

// Info about the node's validator
type ValidatorInfo struct {
	Address     bytes.HexBytes `json:"address"`
	PubKey      crypto.PubKey  `json:"pub_key"`
	VotingPower int64          `json:"voting_power"`
}

// Node Status
type ResultStatus struct {
	NodeInfo      types.NodeInfo `json:"node_info"`
	SyncInfo      SyncInfo       `json:"sync_info"`
	ValidatorInfo ValidatorInfo  `json:"validator_info"`
}

// Is TxIndexing enabled
func (s *ResultStatus) TxIndexEnabled() bool {
	if s == nil {
		return false
	}
	return s.NodeInfo.Other.TxIndex == "on"
}

// Info about peer connections
type ResultNetInfo struct {
	Listening bool     `json:"listening"`
	Listeners []string `json:"listeners"`
	NPeers    int      `json:"n_peers"`
	Peers     []Peer   `json:"peers"`
}

// Log from dialing seeds
type ResultDialSeeds struct {
	Log string `json:"log"`
}

// Log from dialing peers
type ResultDialPeers struct {
	Log string `json:"log"`
}

// A peer
type Peer struct {
	ID  types.NodeID `json:"node_id"`
	URL string       `json:"url"`
}

// Validators for a height.
type ResultValidators struct {
	BlockHeight int64              `json:"block_height"`
	Validators  []*types.Validator `json:"validators"`
	// Count of actual validators in this result
	Count int `json:"count"`
	// Total number of validators
	Total int `json:"total"`
}

// ConsensusParams for given height
type ResultConsensusParams struct {
	BlockHeight     int64                 `json:"block_height"`
	ConsensusParams types.ConsensusParams `json:"consensus_params"`
}

// Info about the consensus state.
// UNSTABLE
type ResultDumpConsensusState struct {
	RoundState json.RawMessage `json:"round_state"`
	Peers      []PeerStateInfo `json:"peers"`
}

// UNSTABLE
type PeerStateInfo struct {
	NodeAddress string          `json:"node_address"`
	PeerState   json.RawMessage `json:"peer_state"`
}

// UNSTABLE
type ResultConsensusState struct {
	RoundState json.RawMessage `json:"round_state"`
}

// CheckTx result
type ResultBroadcastTx struct {
	Code         uint32         `json:"code"`
	Data         bytes.HexBytes `json:"data"`
	Log          string         `json:"log"`
	Codespace    string         `json:"codespace"`
	MempoolError string         `json:"mempool_error"`

	Hash bytes.HexBytes `json:"hash"`
}

// CheckTx and DeliverTx results
type ResultBroadcastTxCommit struct {
	CheckTx   abci.ResponseCheckTx   `json:"check_tx"`
	DeliverTx abci.ResponseDeliverTx `json:"deliver_tx"`
	Hash      bytes.HexBytes         `json:"hash"`
	Height    int64                  `json:"height"`
}

// ResultCheckTx wraps abci.ResponseCheckTx.
type ResultCheckTx struct {
	abci.ResponseCheckTx
}

// Result of querying for a tx
type ResultTx struct {
	Hash     bytes.HexBytes         `json:"hash"`
	Height   int64                  `json:"height"`
	Index    uint32                 `json:"index"`
	TxResult abci.ResponseDeliverTx `json:"tx_result"`
	Tx       types.Tx               `json:"tx"`
	Proof    types.TxProof          `json:"proof,omitempty"`
}

// Result of searching for txs
type ResultTxSearch struct {
	Txs        []*ResultTx `json:"txs"`
	TotalCount int         `json:"total_count"`
}

// ResultBlockSearch defines the RPC response type for a block search by events.
type ResultBlockSearch struct {
	Blocks     []*ResultBlock `json:"blocks"`
	TotalCount int            `json:"total_count"`
}

// List of mempool txs
type ResultUnconfirmedTxs struct {
	Count      int        `json:"n_txs"`
	Total      int        `json:"total"`
	TotalBytes int64      `json:"total_bytes"`
	Txs        []types.Tx `json:"txs"`
}

// Info abci msg
type ResultABCIInfo struct {
	Response abci.ResponseInfo `json:"response"`
}

// Query abci msg
type ResultABCIQuery struct {
	Response abci.ResponseQuery `json:"response"`
}

// Result of broadcasting evidence
type ResultBroadcastEvidence struct {
	Hash []byte `json:"hash"`
}

// empty results
type (
	ResultUnsafeFlushMempool struct{}
	ResultUnsafeProfile      struct{}
	ResultSubscribe          struct{}
	ResultUnsubscribe        struct{}
	ResultHealth             struct{}
)

// Event data from a subscription
type ResultEvent struct {
	SubscriptionID string            `json:"subscription_id"`
	Query          string            `json:"query"`
	Data           types.TMEventData `json:"data"`
	Events         []abci.Event      `json:"events"`
}
