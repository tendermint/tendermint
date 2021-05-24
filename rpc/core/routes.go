package core

import (
	rpc "github.com/tendermint/tendermint/rpc/jsonrpc/server"
)

// TODO: better system than "unsafe" prefix

// Routes is a map of available routes.
<<<<<<< HEAD
var Routes = map[string]*rpc.RPCFunc{
	// subscribe/unsubscribe are reserved for websocket events.
	"subscribe":       rpc.NewWSRPCFunc(Subscribe, "query"),
	"unsubscribe":     rpc.NewWSRPCFunc(Unsubscribe, "query"),
	"unsubscribe_all": rpc.NewWSRPCFunc(UnsubscribeAll, ""),

	// info API
	"health":               rpc.NewRPCFunc(Health, ""),
	"status":               rpc.NewRPCFunc(Status, ""),
	"net_info":             rpc.NewRPCFunc(NetInfo, ""),
	"blockchain":           rpc.NewRPCFunc(BlockchainInfo, "minHeight,maxHeight"),
	"genesis":              rpc.NewRPCFunc(Genesis, ""),
	"block":                rpc.NewRPCFunc(Block, "height"),
	"block_by_hash":        rpc.NewRPCFunc(BlockByHash, "hash"),
	"block_results":        rpc.NewRPCFunc(BlockResults, "height"),
	"commit":               rpc.NewRPCFunc(Commit, "height"),
	"check_tx":             rpc.NewRPCFunc(CheckTx, "tx"),
	"tx":                   rpc.NewRPCFunc(Tx, "hash,prove"),
	"tx_search":            rpc.NewRPCFunc(TxSearch, "query,prove,page,per_page,order_by"),
	"block_search":         rpc.NewRPCFunc(BlockSearch, "query,page,per_page,order_by"),
	"validators":           rpc.NewRPCFunc(Validators, "height,page,per_page"),
	"dump_consensus_state": rpc.NewRPCFunc(DumpConsensusState, ""),
	"consensus_state":      rpc.NewRPCFunc(ConsensusState, ""),
	"consensus_params":     rpc.NewRPCFunc(ConsensusParams, "height"),
	"unconfirmed_txs":      rpc.NewRPCFunc(UnconfirmedTxs, "limit"),
	"num_unconfirmed_txs":  rpc.NewRPCFunc(NumUnconfirmedTxs, ""),

	// tx broadcast API
	"broadcast_tx_commit": rpc.NewRPCFunc(BroadcastTxCommit, "tx"),
	"broadcast_tx_sync":   rpc.NewRPCFunc(BroadcastTxSync, "tx"),
	"broadcast_tx_async":  rpc.NewRPCFunc(BroadcastTxAsync, "tx"),

	// abci API
	"abci_query": rpc.NewRPCFunc(ABCIQuery, "path,data,height,prove"),
	"abci_info":  rpc.NewRPCFunc(ABCIInfo, ""),

	// evidence API
	"broadcast_evidence": rpc.NewRPCFunc(BroadcastEvidence, "evidence"),
=======
func (env *Environment) GetRoutes() RoutesMap {
	return RoutesMap{
		// subscribe/unsubscribe are reserved for websocket events.
		"subscribe":       rpc.NewWSRPCFunc(env.Subscribe, "query"),
		"unsubscribe":     rpc.NewWSRPCFunc(env.Unsubscribe, "query"),
		"unsubscribe_all": rpc.NewWSRPCFunc(env.UnsubscribeAll, ""),

		// info API
		"health":               rpc.NewRPCFunc(env.Health, "", false),
		"status":               rpc.NewRPCFunc(env.Status, "", false),
		"net_info":             rpc.NewRPCFunc(env.NetInfo, "", false),
		"blockchain":           rpc.NewRPCFunc(env.BlockchainInfo, "minHeight,maxHeight", true),
		"genesis":              rpc.NewRPCFunc(env.Genesis, "", true),
		"genesis_chunked":      rpc.NewRPCFunc(env.GenesisChunked, "chunk", true),
		"block":                rpc.NewRPCFunc(env.Block, "height", true),
		"block_by_hash":        rpc.NewRPCFunc(env.BlockByHash, "hash", true),
		"block_results":        rpc.NewRPCFunc(env.BlockResults, "height", true),
		"commit":               rpc.NewRPCFunc(env.Commit, "height", true),
		"check_tx":             rpc.NewRPCFunc(env.CheckTx, "tx", true),
		"tx":                   rpc.NewRPCFunc(env.Tx, "hash,prove", true),
		"tx_search":            rpc.NewRPCFunc(env.TxSearch, "query,prove,page,per_page,order_by", false),
		"block_search":         rpc.NewRPCFunc(env.BlockSearch, "query,page,per_page,order_by", false),
		"validators":           rpc.NewRPCFunc(env.Validators, "height,page,per_page", true),
		"dump_consensus_state": rpc.NewRPCFunc(env.DumpConsensusState, "", false),
		"consensus_state":      rpc.NewRPCFunc(env.GetConsensusState, "", false),
		"consensus_params":     rpc.NewRPCFunc(env.ConsensusParams, "height", true),
		"unconfirmed_txs":      rpc.NewRPCFunc(env.UnconfirmedTxs, "limit", false),
		"num_unconfirmed_txs":  rpc.NewRPCFunc(env.NumUnconfirmedTxs, "", false),

		// tx broadcast API
		"broadcast_tx_commit": rpc.NewRPCFunc(env.BroadcastTxCommit, "tx", false),
		"broadcast_tx_sync":   rpc.NewRPCFunc(env.BroadcastTxSync, "tx", false),
		"broadcast_tx_async":  rpc.NewRPCFunc(env.BroadcastTxAsync, "tx", false),

		// abci API
		"abci_query": rpc.NewRPCFunc(env.ABCIQuery, "path,data,height,prove", false),
		"abci_info":  rpc.NewRPCFunc(env.ABCIInfo, "", true),

		// evidence API
		"broadcast_evidence": rpc.NewRPCFunc(env.BroadcastEvidence, "evidence", false),
	}
>>>>>>> d9134063e (rpc: add chunked rpc interface (#6445))
}

// AddUnsafeRoutes adds unsafe routes.
func AddUnsafeRoutes() {
	// control API
	Routes["dial_seeds"] = rpc.NewRPCFunc(UnsafeDialSeeds, "seeds")
	Routes["dial_peers"] = rpc.NewRPCFunc(UnsafeDialPeers, "peers,persistent,unconditional,private")
	Routes["unsafe_flush_mempool"] = rpc.NewRPCFunc(UnsafeFlushMempool, "")
}
