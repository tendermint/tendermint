package core

import (
	rpc "github.com/tendermint/tendermint/rpc/jsonrpc/server"
)

// TODO: better system than "unsafe" prefix

type RoutesMap map[string]*rpc.RPCFunc

// Routes is a map of available routes.
func (env *Environment) GetRoutes() RoutesMap {
	return RoutesMap{
		// subscribe/unsubscribe are reserved for websocket events.
		"subscribe":       rpc.NewWSRPCFunc(env.Subscribe, "query"),
		"unsubscribe":     rpc.NewWSRPCFunc(env.Unsubscribe, "query"),
		"unsubscribe_all": rpc.NewWSRPCFunc(env.UnsubscribeAll, ""),

		// info API
		"health":               rpc.NewRPCFunc(env.Health, ""),
		"status":               rpc.NewRPCFunc(env.Status, ""),
		"net_info":             rpc.NewRPCFunc(env.NetInfo, ""),
		"blockchain":           rpc.NewRPCFunc(env.BlockchainInfo, "minHeight,maxHeight"),
		"genesis":              rpc.NewRPCFunc(env.Genesis, ""),
		"genesis_chunked":      rpc.NewRPCFunc(env.GenesisChunked, "chunk"),
		"block":                rpc.NewRPCFunc(env.Block, "height"),
		"block_by_hash":        rpc.NewRPCFunc(env.BlockByHash, "hash"),
		"block_results":        rpc.NewRPCFunc(env.BlockResults, "height"),
		"commit":               rpc.NewRPCFunc(env.Commit, "height"),
		"check_tx":             rpc.NewRPCFunc(env.CheckTx, "tx"),
		"tx":                   rpc.NewRPCFunc(env.Tx, "hash,prove"),
		"tx_search":            rpc.NewRPCFunc(env.TxSearch, "query,prove,page,per_page,order_by"),
		"block_search":         rpc.NewRPCFunc(env.BlockSearch, "query,page,per_page,order_by"),
		"validators":           rpc.NewRPCFunc(env.Validators, "height,page,per_page"),
		"dump_consensus_state": rpc.NewRPCFunc(env.DumpConsensusState, ""),
		"consensus_state":      rpc.NewRPCFunc(env.GetConsensusState, ""),
		"consensus_params":     rpc.NewRPCFunc(env.ConsensusParams, "height"),
		"unconfirmed_txs":      rpc.NewRPCFunc(env.UnconfirmedTxs, "limit"),
		"num_unconfirmed_txs":  rpc.NewRPCFunc(env.NumUnconfirmedTxs, ""),

		// tx broadcast API
		"broadcast_tx_commit": rpc.NewRPCFunc(env.BroadcastTxCommit, "tx"),
		"broadcast_tx_sync":   rpc.NewRPCFunc(env.BroadcastTxSync, "tx"),
		"broadcast_tx_async":  rpc.NewRPCFunc(env.BroadcastTxAsync, "tx"),

		// abci API
		"abci_query": rpc.NewRPCFunc(env.ABCIQuery, "path,data,height,prove"),
		"abci_info":  rpc.NewRPCFunc(env.ABCIInfo, ""),

		// evidence API
		"broadcast_evidence": rpc.NewRPCFunc(env.BroadcastEvidence, "evidence"),
	}
}

// AddUnsafeRoutes adds unsafe routes.
func (env *Environment) AddUnsafeRoutes(routes RoutesMap) {
	// control API
	routes["dial_seeds"] = rpc.NewRPCFunc(env.UnsafeDialSeeds, "seeds")
	routes["dial_peers"] = rpc.NewRPCFunc(env.UnsafeDialPeers, "peers,persistent,unconditional,private")
	routes["unsafe_flush_mempool"] = rpc.NewRPCFunc(env.UnsafeFlushMempool, "")
}
