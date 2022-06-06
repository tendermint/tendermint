package core

import (
	"context"

	"github.com/tendermint/tendermint/rpc/coretypes"
	rpc "github.com/tendermint/tendermint/rpc/jsonrpc/server"
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
		"events":          rpc.NewRPCFunc(svc.Events).Timeout(0),
		"subscribe":       rpc.NewWSRPCFunc(svc.Subscribe),
		"unsubscribe":     rpc.NewWSRPCFunc(svc.Unsubscribe),
		"unsubscribe_all": rpc.NewWSRPCFunc(svc.UnsubscribeAll),

		// info API
		"health":               rpc.NewRPCFunc(svc.Health),
		"status":               rpc.NewRPCFunc(svc.Status),
		"net_info":             rpc.NewRPCFunc(svc.NetInfo),
		"blockchain":           rpc.NewRPCFunc(svc.BlockchainInfo),
		"genesis":              rpc.NewRPCFunc(svc.Genesis),
		"genesis_chunked":      rpc.NewRPCFunc(svc.GenesisChunked),
		"header":               rpc.NewRPCFunc(svc.Header),
		"header_by_hash":       rpc.NewRPCFunc(svc.HeaderByHash),
		"block":                rpc.NewRPCFunc(svc.Block),
		"block_by_hash":        rpc.NewRPCFunc(svc.BlockByHash),
		"block_results":        rpc.NewRPCFunc(svc.BlockResults),
		"commit":               rpc.NewRPCFunc(svc.Commit),
		"check_tx":             rpc.NewRPCFunc(svc.CheckTx),
		"remove_tx":            rpc.NewRPCFunc(svc.RemoveTx),
		"tx":                   rpc.NewRPCFunc(svc.Tx),
		"tx_search":            rpc.NewRPCFunc(svc.TxSearch),
		"block_search":         rpc.NewRPCFunc(svc.BlockSearch),
		"validators":           rpc.NewRPCFunc(svc.Validators),
		"dump_consensus_state": rpc.NewRPCFunc(svc.DumpConsensusState),
		"consensus_state":      rpc.NewRPCFunc(svc.GetConsensusState),
		"consensus_params":     rpc.NewRPCFunc(svc.ConsensusParams),
		"unconfirmed_txs":      rpc.NewRPCFunc(svc.UnconfirmedTxs),
		"num_unconfirmed_txs":  rpc.NewRPCFunc(svc.NumUnconfirmedTxs),

		// tx broadcast API
		"broadcast_tx": rpc.NewRPCFunc(svc.BroadcastTx),
		// TODO remove after 0.36
		// deprecated broadcast tx methods:
		"broadcast_tx_commit": rpc.NewRPCFunc(svc.BroadcastTxCommit),
		"broadcast_tx_sync":   rpc.NewRPCFunc(svc.BroadcastTx),
		"broadcast_tx_async":  rpc.NewRPCFunc(svc.BroadcastTxAsync),

		// abci API
		"abci_query": rpc.NewRPCFunc(svc.ABCIQuery),
		"abci_info":  rpc.NewRPCFunc(svc.ABCIInfo),

		// evidence API
		"broadcast_evidence": rpc.NewRPCFunc(svc.BroadcastEvidence),
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
	ABCIQuery(ctx context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error)
	Block(ctx context.Context, req *coretypes.RequestBlockInfo) (*coretypes.ResultBlock, error)
	BlockByHash(ctx context.Context, req *coretypes.RequestBlockByHash) (*coretypes.ResultBlock, error)
	BlockResults(ctx context.Context, req *coretypes.RequestBlockInfo) (*coretypes.ResultBlockResults, error)
	BlockSearch(ctx context.Context, req *coretypes.RequestBlockSearch) (*coretypes.ResultBlockSearch, error)
	BlockchainInfo(ctx context.Context, req *coretypes.RequestBlockchainInfo) (*coretypes.ResultBlockchainInfo, error)
	BroadcastEvidence(ctx context.Context, req *coretypes.RequestBroadcastEvidence) (*coretypes.ResultBroadcastEvidence, error)
	BroadcastTx(ctx context.Context, req *coretypes.RequestBroadcastTx) (*coretypes.ResultBroadcastTx, error)
	BroadcastTxAsync(ctx context.Context, req *coretypes.RequestBroadcastTx) (*coretypes.ResultBroadcastTx, error)
	BroadcastTxCommit(ctx context.Context, req *coretypes.RequestBroadcastTx) (*coretypes.ResultBroadcastTxCommit, error)
	BroadcastTxSync(ctx context.Context, req *coretypes.RequestBroadcastTx) (*coretypes.ResultBroadcastTx, error)
	CheckTx(ctx context.Context, req *coretypes.RequestCheckTx) (*coretypes.ResultCheckTx, error)
	Commit(ctx context.Context, req *coretypes.RequestBlockInfo) (*coretypes.ResultCommit, error)
	ConsensusParams(ctx context.Context, req *coretypes.RequestConsensusParams) (*coretypes.ResultConsensusParams, error)
	DumpConsensusState(ctx context.Context) (*coretypes.ResultDumpConsensusState, error)
	Events(ctx context.Context, req *coretypes.RequestEvents) (*coretypes.ResultEvents, error)
	Genesis(ctx context.Context) (*coretypes.ResultGenesis, error)
	GenesisChunked(ctx context.Context, req *coretypes.RequestGenesisChunked) (*coretypes.ResultGenesisChunk, error)
	GetConsensusState(ctx context.Context) (*coretypes.ResultConsensusState, error)
	Header(ctx context.Context, req *coretypes.RequestBlockInfo) (*coretypes.ResultHeader, error)
	HeaderByHash(ctx context.Context, req *coretypes.RequestBlockByHash) (*coretypes.ResultHeader, error)
	Health(ctx context.Context) (*coretypes.ResultHealth, error)
	NetInfo(ctx context.Context) (*coretypes.ResultNetInfo, error)
	NumUnconfirmedTxs(ctx context.Context) (*coretypes.ResultUnconfirmedTxs, error)
	RemoveTx(ctx context.Context, req *coretypes.RequestRemoveTx) error
	Status(ctx context.Context) (*coretypes.ResultStatus, error)
	Subscribe(ctx context.Context, req *coretypes.RequestSubscribe) (*coretypes.ResultSubscribe, error)
	Tx(ctx context.Context, req *coretypes.RequestTx) (*coretypes.ResultTx, error)
	TxSearch(ctx context.Context, req *coretypes.RequestTxSearch) (*coretypes.ResultTxSearch, error)
	UnconfirmedTxs(ctx context.Context, req *coretypes.RequestUnconfirmedTxs) (*coretypes.ResultUnconfirmedTxs, error)
	Unsubscribe(ctx context.Context, req *coretypes.RequestUnsubscribe) (*coretypes.ResultUnsubscribe, error)
	UnsubscribeAll(ctx context.Context) (*coretypes.ResultUnsubscribe, error)
	Validators(ctx context.Context, req *coretypes.RequestValidators) (*coretypes.ResultValidators, error)
}

// RPCUnsafe defines the set of "unsafe" methods that may optionally be
// exported by the RPC service.
type RPCUnsafe interface {
	UnsafeFlushMempool(ctx context.Context) (*coretypes.ResultUnsafeFlushMempool, error)
}
