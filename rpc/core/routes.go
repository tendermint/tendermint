package core

import (
	rpc "github.com/tendermint/tendermint/rpc/jsonrpc/server"
)

// TODO: better system than "unsafe" prefix

// Routes is a map of available routes.
var Routes = map[string]*rpc.RPCFunc{
	// subscribe/unsubscribe are reserved for websocket events.
	"subscribe":       rpc.NewWSRPCFunc(Subscribe, "query"),
	"unsubscribe":     rpc.NewWSRPCFunc(Unsubscribe, "query"),
	"unsubscribe_all": rpc.NewWSRPCFunc(UnsubscribeAll, ""),

	// info API
	"health":               rpc.NewRPCFunc(Health, "", false),
	"status":               rpc.NewRPCFunc(Status, "", false),
	"net_info":             rpc.NewRPCFunc(NetInfo, "", false),
	"blockchain":           rpc.NewRPCFunc(BlockchainInfo, "minHeight,maxHeight", true),
	"genesis":              rpc.NewRPCFunc(Genesis, "", true),
	"genesis_chunked":      rpc.NewRPCFunc(GenesisChunked, "chunk", true),
	"block":                rpc.NewRPCFunc(Block, "height", true),
	"block_by_hash":        rpc.NewRPCFunc(BlockByHash, "hash", true),
	"block_results":        rpc.NewRPCFunc(BlockResults, "height", true),
	"commit":               rpc.NewRPCFunc(Commit, "height", true),
	"check_tx":             rpc.NewRPCFunc(CheckTx, "tx", true),
	"tx":                   rpc.NewRPCFunc(Tx, "hash,prove", true),
	"tx_search":            rpc.NewRPCFunc(TxSearch, "query,prove,page,per_page,order_by", false),
	"block_search":         rpc.NewRPCFunc(BlockSearch, "query,page,per_page,order_by", false),
	"validators":           rpc.NewRPCFunc(Validators, "height,page,per_page", true),
	"dump_consensus_state": rpc.NewRPCFunc(DumpConsensusState, "", false),
	"consensus_state":      rpc.NewRPCFunc(ConsensusState, "", false),
	"consensus_params":     rpc.NewRPCFunc(ConsensusParams, "height", true),
	"unconfirmed_txs":      rpc.NewRPCFunc(UnconfirmedTxs, "limit", false),
	"num_unconfirmed_txs":  rpc.NewRPCFunc(NumUnconfirmedTxs, "", false),

	// tx broadcast API
	"broadcast_tx_commit": rpc.NewRPCFunc(BroadcastTxCommit, "tx", false),
	"broadcast_tx_sync":   rpc.NewRPCFunc(BroadcastTxSync, "tx", false),
	"broadcast_tx_async":  rpc.NewRPCFunc(BroadcastTxAsync, "tx", false),

	// abci API
	"abci_query": rpc.NewRPCFunc(ABCIQuery, "path,data,height,prove", false),
	"abci_info":  rpc.NewRPCFunc(ABCIInfo, "", true),

	// evidence API
	"broadcast_evidence": rpc.NewRPCFunc(BroadcastEvidence, "evidence", false),
}

// AddUnsafeRoutes adds unsafe routes.
func AddUnsafeRoutes() {
	// control API
	Routes["dial_seeds"] = rpc.NewRPCFunc(UnsafeDialSeeds, "seeds", false)
	Routes["dial_peers"] = rpc.NewRPCFunc(UnsafeDialPeers, "peers,persistent,unconditional,private", false)
	Routes["unsafe_flush_mempool"] = rpc.NewRPCFunc(UnsafeFlushMempool, "", false)
}
