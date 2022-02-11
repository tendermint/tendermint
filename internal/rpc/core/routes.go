package core

import (
	"context"
	"time"

	"github.com/tendermint/tendermint/internal/eventlog/cursor"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/rpc/coretypes"
	rpc "github.com/tendermint/tendermint/rpc/jsonrpc/server"
	"github.com/tendermint/tendermint/types"
)

// TODO: better system than "unsafe" prefix

type RoutesMap map[string]*rpc.RPCFunc

// RouteOptions provide optional settings to NewRoutesMap.  A nil *RouteOptions
// is ready for use and provides defaults as specified.
type RouteOptions struct {
	Unsafe bool // include "unsafe" methods (default false)
}

// NewRoutesMap constructs an RPC routing map for the given service
// implementation. If svc implements RPCUnsafe and opts.Unsafe is true, the
// "unsafe" methods will also be added to the map. The caller may also edit the
// map after construction; each call to NewRoutesMap returns a fresh map.
func NewRoutesMap(svc RPCService, opts *RouteOptions) RoutesMap {
	if opts == nil {
		opts = new(RouteOptions)
	}
	out := RoutesMap{
		// Event subscription. Note that subscribe, unsubscribe, and
		// unsubscribe_all are only available via the websocket endpoint.
		"events":          rpc.NewRPCFunc(svc.Events, "filter", "maxItems", "before", "after", "waitTime"),
		"subscribe":       rpc.NewWSRPCFunc(svc.Subscribe, "query"),
		"unsubscribe":     rpc.NewWSRPCFunc(svc.Unsubscribe, "query"),
		"unsubscribe_all": rpc.NewWSRPCFunc(svc.UnsubscribeAll),

		// info API
		"health":               rpc.NewRPCFunc(svc.Health),
		"status":               rpc.NewRPCFunc(svc.Status),
		"net_info":             rpc.NewRPCFunc(svc.NetInfo),
		"blockchain":           rpc.NewRPCFunc(svc.BlockchainInfo, "minHeight", "maxHeight"),
		"genesis":              rpc.NewRPCFunc(svc.Genesis),
		"genesis_chunked":      rpc.NewRPCFunc(svc.GenesisChunked, "chunk"),
		"header":               rpc.NewRPCFunc(svc.Header, "height"),
		"header_by_hash":       rpc.NewRPCFunc(svc.HeaderByHash, "hash"),
		"block":                rpc.NewRPCFunc(svc.Block, "height"),
		"block_by_hash":        rpc.NewRPCFunc(svc.BlockByHash, "hash"),
		"block_results":        rpc.NewRPCFunc(svc.BlockResults, "height"),
		"commit":               rpc.NewRPCFunc(svc.Commit, "height"),
		"check_tx":             rpc.NewRPCFunc(svc.CheckTx, "tx"),
		"remove_tx":            rpc.NewRPCFunc(svc.RemoveTx, "txkey"),
		"tx":                   rpc.NewRPCFunc(svc.Tx, "hash", "prove"),
		"tx_search":            rpc.NewRPCFunc(svc.TxSearch, "query", "prove", "page", "per_page", "order_by"),
		"block_search":         rpc.NewRPCFunc(svc.BlockSearch, "query", "page", "per_page", "order_by"),
		"validators":           rpc.NewRPCFunc(svc.Validators, "height", "page", "per_page"),
		"dump_consensus_state": rpc.NewRPCFunc(svc.DumpConsensusState),
		"consensus_state":      rpc.NewRPCFunc(svc.GetConsensusState),
		"consensus_params":     rpc.NewRPCFunc(svc.ConsensusParams, "height"),
		"unconfirmed_txs":      rpc.NewRPCFunc(svc.UnconfirmedTxs, "page", "per_page"),
		"num_unconfirmed_txs":  rpc.NewRPCFunc(svc.NumUnconfirmedTxs),

		// tx broadcast API
		"broadcast_tx_commit": rpc.NewRPCFunc(svc.BroadcastTxCommit, "tx"),
		"broadcast_tx_sync":   rpc.NewRPCFunc(svc.BroadcastTxSync, "tx"),
		"broadcast_tx_async":  rpc.NewRPCFunc(svc.BroadcastTxAsync, "tx"),

		// abci API
		"abci_query": rpc.NewRPCFunc(svc.ABCIQuery, "path", "data", "height", "prove"),
		"abci_info":  rpc.NewRPCFunc(svc.ABCIInfo),

		// evidence API
		"broadcast_evidence": rpc.NewRPCFunc(svc.BroadcastEvidence, "evidence"),
	}
	if u, ok := svc.(RPCUnsafe); ok && opts.Unsafe {
		out["unsafe_flush_mempool"] = rpc.NewRPCFunc(u.UnsafeFlushMempool)
	}
	return out
}

// RPCService defines the set of methods exported by the RPC service
// implementation, for use in constructing a routing table.
type RPCService interface {
	ABCIInfo(ctx context.Context) (*coretypes.ResultABCIInfo, error)
	ABCIQuery(ctx context.Context, path string, data bytes.HexBytes, height int64, prove bool) (*coretypes.ResultABCIQuery, error)
	Block(ctx context.Context, heightPtr *int64) (*coretypes.ResultBlock, error)
	BlockByHash(ctx context.Context, hash bytes.HexBytes) (*coretypes.ResultBlock, error)
	BlockResults(ctx context.Context, heightPtr *int64) (*coretypes.ResultBlockResults, error)
	BlockSearch(ctx context.Context, query string, pagePtr, perPagePtr *int, orderBy string) (*coretypes.ResultBlockSearch, error)
	BlockchainInfo(ctx context.Context, minHeight, maxHeight int64) (*coretypes.ResultBlockchainInfo, error)
	BroadcastEvidence(ctx context.Context, ev coretypes.Evidence) (*coretypes.ResultBroadcastEvidence, error)
	BroadcastTxAsync(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error)
	BroadcastTxCommit(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTxCommit, error)
	BroadcastTxSync(ctx context.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error)
	CheckTx(ctx context.Context, tx types.Tx) (*coretypes.ResultCheckTx, error)
	Commit(ctx context.Context, heightPtr *int64) (*coretypes.ResultCommit, error)
	ConsensusParams(ctx context.Context, heightPtr *int64) (*coretypes.ResultConsensusParams, error)
	DumpConsensusState(ctx context.Context) (*coretypes.ResultDumpConsensusState, error)
	Events(ctx context.Context, filter *coretypes.EventFilter, maxItems int, before, after cursor.Cursor, waitTime time.Duration) (*coretypes.ResultEvents, error)
	Genesis(ctx context.Context) (*coretypes.ResultGenesis, error)
	GenesisChunked(ctx context.Context, chunk uint) (*coretypes.ResultGenesisChunk, error)
	GetConsensusState(ctx context.Context) (*coretypes.ResultConsensusState, error)
	Header(ctx context.Context, heightPtr *int64) (*coretypes.ResultHeader, error)
	HeaderByHash(ctx context.Context, hash bytes.HexBytes) (*coretypes.ResultHeader, error)
	Health(ctx context.Context) (*coretypes.ResultHealth, error)
	NetInfo(ctx context.Context) (*coretypes.ResultNetInfo, error)
	NumUnconfirmedTxs(ctx context.Context) (*coretypes.ResultUnconfirmedTxs, error)
	RemoveTx(ctx context.Context, txkey types.TxKey) error
	Status(ctx context.Context) (*coretypes.ResultStatus, error)
	Subscribe(ctx context.Context, query string) (*coretypes.ResultSubscribe, error)
	Tx(ctx context.Context, hash bytes.HexBytes, prove bool) (*coretypes.ResultTx, error)
	TxSearch(ctx context.Context, query string, prove bool, pagePtr, perPagePtr *int, orderBy string) (*coretypes.ResultTxSearch, error)
	UnconfirmedTxs(ctx context.Context, page, perPage *int) (*coretypes.ResultUnconfirmedTxs, error)
	Unsubscribe(ctx context.Context, query string) (*coretypes.ResultUnsubscribe, error)
	UnsubscribeAll(ctx context.Context) (*coretypes.ResultUnsubscribe, error)
	Validators(ctx context.Context, heightPtr *int64, pagePtr, perPagePtr *int) (*coretypes.ResultValidators, error)
}

// RPCUnsafe defines the set of "unsafe" methods that may optionally be
// exported by the RPC service.
type RPCUnsafe interface {
	UnsafeFlushMempool(ctx context.Context) (*coretypes.ResultUnsafeFlushMempool, error)
}
