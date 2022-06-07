package coretypes

import (
	"encoding/json"
	"errors"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/jsontypes"
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
	LastHeight int64              `json:"last_height,string"`
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
	ChunkNumber int    `json:"chunk,string"`
	TotalChunks int    `json:"total,string"`
	Data        string `json:"data"`
}

// Single block (with meta)
type ResultBlock struct {
	BlockID types.BlockID `json:"block_id"`
	Block   *types.Block  `json:"block"`
}

// ResultHeader represents the response for a Header RPC Client query
type ResultHeader struct {
	Header *types.Header `json:"header"`
}

// Commit and Header
type ResultCommit struct {
	types.SignedHeader `json:"signed_header"`
	CanonicalCommit    bool `json:"canonical"`
}

// ABCI results from a block
type ResultBlockResults struct {
	Height                int64                    `json:"height,string"`
	TxsResults            []*abci.ExecTxResult     `json:"txs_results"`
	TotalGasUsed          int64                    `json:"total_gas_used,string"`
	FinalizeBlockEvents   []abci.Event             `json:"finalize_block_events"`
	ValidatorUpdates      []abci.ValidatorUpdate   `json:"validator_updates"`
	ConsensusParamUpdates *tmproto.ConsensusParams `json:"consensus_param_updates"`
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
	LatestBlockHeight int64          `json:"latest_block_height,string"`
	LatestBlockTime   time.Time      `json:"latest_block_time"`

	EarliestBlockHash   bytes.HexBytes `json:"earliest_block_hash"`
	EarliestAppHash     bytes.HexBytes `json:"earliest_app_hash"`
	EarliestBlockHeight int64          `json:"earliest_block_height,string"`
	EarliestBlockTime   time.Time      `json:"earliest_block_time"`

	MaxPeerBlockHeight int64 `json:"max_peer_block_height,string"`

	CatchingUp bool `json:"catching_up"`

	TotalSyncedTime time.Duration `json:"total_synced_time,string"`
	RemainingTime   time.Duration `json:"remaining_time,string"`

	TotalSnapshots      int64         `json:"total_snapshots,string"`
	ChunkProcessAvgTime time.Duration `json:"chunk_process_avg_time,string"`
	SnapshotHeight      int64         `json:"snapshot_height,string"`
	SnapshotChunksCount int64         `json:"snapshot_chunks_count,string"`
	SnapshotChunksTotal int64         `json:"snapshot_chunks_total,string"`
	BackFilledBlocks    int64         `json:"backfilled_blocks,string"`
	BackFillBlocksTotal int64         `json:"backfill_blocks_total,string"`
}

type ApplicationInfo struct {
	Version string `json:"version"`
}

// Info about the node's validator
type ValidatorInfo struct {
	Address     bytes.HexBytes
	PubKey      crypto.PubKey
	VotingPower int64
}

type validatorInfoJSON struct {
	Address     bytes.HexBytes  `json:"address"`
	PubKey      json.RawMessage `json:"pub_key"`
	VotingPower int64           `json:"voting_power,string"`
}

func (v ValidatorInfo) MarshalJSON() ([]byte, error) {
	pk, err := jsontypes.Marshal(v.PubKey)
	if err != nil {
		return nil, err
	}
	return json.Marshal(validatorInfoJSON{
		Address: v.Address, PubKey: pk, VotingPower: v.VotingPower,
	})
}

func (v *ValidatorInfo) UnmarshalJSON(data []byte) error {
	var val validatorInfoJSON
	if err := json.Unmarshal(data, &val); err != nil {
		return err
	}
	if err := jsontypes.Unmarshal(val.PubKey, &v.PubKey); err != nil {
		return err
	}
	v.Address = val.Address
	v.VotingPower = val.VotingPower
	return nil
}

// Node Status
type ResultStatus struct {
	NodeInfo        types.NodeInfo        `json:"node_info"`
	ApplicationInfo ApplicationInfo       `json:"application_info,omitempty"`
	SyncInfo        SyncInfo              `json:"sync_info"`
	ValidatorInfo   ValidatorInfo         `json:"validator_info"`
	LightClientInfo types.LightClientInfo `json:"light_client_info,omitempty"`
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
	NPeers    int      `json:"n_peers,string"`
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
	BlockHeight int64              `json:"block_height,string"`
	Validators  []*types.Validator `json:"validators"`

	Count int `json:"count,string"` // Count of actual validators in this result
	Total int `json:"total,string"` // Total number of validators
}

// ConsensusParams for given height
type ResultConsensusParams struct {
	BlockHeight     int64                 `json:"block_height,string"`
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
	Code      uint32         `json:"code"`
	Data      bytes.HexBytes `json:"data"`
	Codespace string         `json:"codespace"`
	Hash      bytes.HexBytes `json:"hash"`
}

// CheckTx and DeliverTx results
type ResultBroadcastTxCommit struct {
	CheckTx  abci.ResponseCheckTx `json:"check_tx"`
	TxResult abci.ExecTxResult    `json:"tx_result"`
	Hash     bytes.HexBytes       `json:"hash"`
	Height   int64                `json:"height,string"`
}

// ResultCheckTx wraps abci.ResponseCheckTx.
type ResultCheckTx struct {
	abci.ResponseCheckTx
}

// Result of querying for a tx
type ResultTx struct {
	Hash     bytes.HexBytes    `json:"hash"`
	Height   int64             `json:"height,string"`
	Index    uint32            `json:"index"`
	TxResult abci.ExecTxResult `json:"tx_result"`
	Tx       types.Tx          `json:"tx"`
	Proof    types.TxProof     `json:"proof,omitempty"`
}

// Result of searching for txs
type ResultTxSearch struct {
	Txs        []*ResultTx `json:"txs"`
	TotalCount int         `json:"total_count,string"`
}

// ResultBlockSearch defines the RPC response type for a block search by events.
type ResultBlockSearch struct {
	Blocks     []*ResultBlock `json:"blocks"`
	TotalCount int            `json:"total_count,string"`
}

// List of mempool txs
type ResultUnconfirmedTxs struct {
	Count      int        `json:"n_txs,string"`
	Total      int        `json:"total,string"`
	TotalBytes int64      `json:"total_bytes,string"`
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
	SubscriptionID string
	Query          string
	Data           types.EventData
	Events         []abci.Event
}

type resultEventJSON struct {
	SubscriptionID string          `json:"subscription_id"`
	Query          string          `json:"query"`
	Data           json.RawMessage `json:"data"`
	Events         []abci.Event    `json:"events"`
}

func (r ResultEvent) MarshalJSON() ([]byte, error) {
	evt, err := jsontypes.Marshal(r.Data)
	if err != nil {
		return nil, err
	}
	return json.Marshal(resultEventJSON{
		SubscriptionID: r.SubscriptionID,
		Query:          r.Query,
		Data:           evt,
		Events:         r.Events,
	})
}

func (r *ResultEvent) UnmarshalJSON(data []byte) error {
	var res resultEventJSON
	if err := json.Unmarshal(data, &res); err != nil {
		return err
	}
	if err := jsontypes.Unmarshal(res.Data, &r.Data); err != nil {
		return err
	}
	r.SubscriptionID = res.SubscriptionID
	r.Query = res.Query
	r.Events = res.Events
	return nil
}

// Evidence is an argument wrapper for a types.Evidence value, that handles
// encoding and decoding through JSON.
type Evidence struct {
	Value types.Evidence
}

func (e Evidence) MarshalJSON() ([]byte, error)     { return jsontypes.Marshal(e.Value) }
func (e *Evidence) UnmarshalJSON(data []byte) error { return jsontypes.Unmarshal(data, &e.Value) }

// ResultEvents is the response from the "/events" RPC endpoint.
type ResultEvents struct {
	// The items matching the request parameters, from newest
	// to oldest, if any were available within the timeout.
	Items []*EventItem `json:"items"`

	// This is true if there is at least one older matching item
	// available in the log that was not returned.
	More bool `json:"more"`

	// The cursor of the oldest item in the log at the time of this reply,
	// or "" if the log is empty.
	Oldest string `json:"oldest"`

	// The cursor of the newest item in the log at the time of this reply,
	// or "" if the log is empty.
	Newest string `json:"newest"`
}

type EventItem struct {
	// The cursor of this item.
	Cursor string `json:"cursor"`

	// The event label of this item (for example, "Vote").
	Event string `json:"event,omitempty"`

	// The encoded event data for this item. The content is a JSON object with
	// the following structure:
	//
	//   {
	//      "type":  "type-tag",
	//      "value": <json-encoded-value>
	//   }
	//
	// The known type tags are defined by the tendermint/types package.
	Data json.RawMessage `json:"data"`
}
