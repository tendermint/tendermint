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
	"health":               rpc.NewRPCFunc(Health, "", rpc.FuncOptions{}),
	"status":               rpc.NewRPCFunc(Status, "", rpc.FuncOptions{}),
	"net_info":             rpc.NewRPCFunc(NetInfo, "", rpc.FuncOptions{}),
	"blockchain":           rpc.NewRPCFunc(BlockchainInfo, "minHeight,maxHeight", rpc.CacheFuncOpts),
	"genesis":              rpc.NewRPCFunc(Genesis, "", rpc.CacheFuncOpts),
	"genesis_chunked":      rpc.NewRPCFunc(GenesisChunked, "chunk", rpc.CacheFuncOpts),
	"block":                rpc.NewRPCFunc(Block, "height", rpc.CacheFuncOpts),
	"block_by_hash":        rpc.NewRPCFunc(BlockByHash, "hash", rpc.CacheFuncOpts),
	"block_results":        rpc.NewRPCFunc(BlockResults, "height", rpc.CacheFuncOpts),
	"commit":               rpc.NewRPCFunc(Commit, "height", rpc.CacheFuncOpts),
	"header":               rpc.NewRPCFunc(Header, "height", rpc.CacheFuncOpts),
	"header_by_hash":       rpc.NewRPCFunc(HeaderByHash, "hash", rpc.CacheFuncOpts),
	"check_tx":             rpc.NewRPCFunc(CheckTx, "tx", rpc.CacheFuncOpts),
	"tx":                   rpc.NewRPCFunc(Tx, "hash,prove", rpc.CacheFuncOpts),
	"tx_search":            rpc.NewRPCFunc(TxSearch, "query,prove,page,per_page,order_by", rpc.FuncOptions{}),
	"block_search":         rpc.NewRPCFunc(BlockSearch, "query,page,per_page,order_by", rpc.FuncOptions{}),
	"validators":           rpc.NewRPCFunc(Validators, "height,page,per_page", rpc.CacheFuncOpts),
	"dump_consensus_state": rpc.NewRPCFunc(DumpConsensusState, "", rpc.FuncOptions{}),
	"consensus_state":      rpc.NewRPCFunc(ConsensusState, "", rpc.FuncOptions{}),
	"consensus_params":     rpc.NewRPCFunc(ConsensusParams, "height", rpc.CacheFuncOpts),
	"unconfirmed_txs":      rpc.NewRPCFunc(UnconfirmedTxs, "limit", rpc.FuncOptions{}),
	"num_unconfirmed_txs":  rpc.NewRPCFunc(NumUnconfirmedTxs, "", rpc.FuncOptions{}),

	// tx broadcast API
	"broadcast_tx_commit": rpc.NewRPCFunc(BroadcastTxCommit, "tx", rpc.FuncOptions{}),
	"broadcast_tx_sync":   rpc.NewRPCFunc(BroadcastTxSync, "tx", rpc.FuncOptions{}),
	"broadcast_tx_async":  rpc.NewRPCFunc(BroadcastTxAsync, "tx", rpc.FuncOptions{}),

	// abci API
	"abci_query": rpc.NewRPCFunc(ABCIQuery, "path,data,height,prove", rpc.FuncOptions{}),
	"abci_info":  rpc.NewRPCFunc(ABCIInfo, "", rpc.CacheFuncOpts),

	// evidence API
	"broadcast_evidence": rpc.NewRPCFunc(BroadcastEvidence, "evidence", rpc.FuncOptions{}),
}

// AddUnsafeRoutes adds unsafe routes.
func AddUnsafeRoutes() {
	// control API
	Routes["dial_seeds"] = rpc.NewRPCFunc(UnsafeDialSeeds, "seeds", rpc.FuncOptions{})
	Routes["dial_peers"] = rpc.NewRPCFunc(UnsafeDialPeers, "peers,persistent,unconditional,private", rpc.FuncOptions{})
	Routes["unsafe_flush_mempool"] = rpc.NewRPCFunc(UnsafeFlushMempool, "", rpc.FuncOptions{})
}
