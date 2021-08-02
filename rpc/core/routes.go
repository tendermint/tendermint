package core

import (
	rpc "github.com/tendermint/tendermint/rpc/jsonrpc/server"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/indexer"
)

// TODO: better system than "unsafe" prefix

type RoutesMap map[string]*rpc.RPCFunc

// Routes is a map of available routes.
func (env *Environment) GetRoutes() RoutesMap {
	return CombineRoutes(env.InfoRoutes(),
		env.SubscribeRoutes(),
		env.BroadcastTxRoutes(),
		env.ABCIQueryRoutes(),
		env.EvidenceRoutes(),
	)
}

func (env *Environment) InfoRoutes() RoutesMap {
	return RoutesMap{
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
	}
}

func (env *Environment) SubscribeRoutes() RoutesMap {
	return RoutesMap{
		// subscribe/unsubscribe are reserved for websocket events.
		"subscribe":       rpc.NewWSRPCFunc(env.Subscribe, "query"),
		"unsubscribe":     rpc.NewWSRPCFunc(env.Unsubscribe, "query"),
		"unsubscribe_all": rpc.NewWSRPCFunc(env.UnsubscribeAll, ""),
	}
}

func (env *Environment) BroadcastTxRoutes() RoutesMap {
	// tx broadcast API
	return RoutesMap{
		"broadcast_tx_commit": rpc.NewRPCFunc(env.BroadcastTxCommit, "tx", false),
		"broadcast_tx_sync":   rpc.NewRPCFunc(env.BroadcastTxSync, "tx", false),
		"broadcast_tx_async":  rpc.NewRPCFunc(env.BroadcastTxAsync, "tx", false),
	}
}

func (env *Environment) EvidenceRoutes() RoutesMap {
	// evidence API
	return RoutesMap{
		"broadcast_evidence": rpc.NewRPCFunc(env.BroadcastEvidence, "evidence", false),
	}
}

func (env *Environment) ABCIQueryRoutes() RoutesMap {
	// abci API
	return RoutesMap{
		"abci_query": rpc.NewRPCFunc(env.ABCIQuery, "path,data,height,prove", false),
		"abci_info":  rpc.NewRPCFunc(env.ABCIInfo, "", true),
	}
}

// AddUnsafeRoutes adds unsafe routes.
func (env *Environment) UnsafeRoutes() RoutesMap {
	// control API
	return RoutesMap{
		"dial_seeds":           rpc.NewRPCFunc(env.UnsafeDialSeeds, "seeds", false),
		"dial_peers":           rpc.NewRPCFunc(env.UnsafeDialPeers, "peers,persistent,unconditional,private", false),
		"unsafe_flush_mempool": rpc.NewRPCFunc(env.UnsafeFlushMempool, "", false),
	}
}

func CombineRoutes(routesMaps ...RoutesMap) RoutesMap {
	res := RoutesMap{}
	for _, routesMap := range routesMaps {
		for path, rpcFunc := range routesMap {
			res[path] = rpcFunc
		}
	}
	return res
}

func DebugRoutes(store sm.Store, blockStore sm.BlockStore, eventSinks []indexer.EventSink) RoutesMap {
	env := &Environment{
		EventSinks: eventSinks,
		StateStore: store,
		BlockStore: blockStore,
	}
	return RoutesMap{
		"blockchain":       rpc.NewRPCFunc(env.BlockchainInfo, "minHeight,maxHeight", true),
		"consensus_params": rpc.NewRPCFunc(env.ConsensusParams, "height", true),
		"block":            rpc.NewRPCFunc(env.Block, "height", true),
		"block_by_hash":    rpc.NewRPCFunc(env.BlockByHash, "hash", true),
		"block_results":    rpc.NewRPCFunc(env.BlockResults, "height", true),
		"commit":           rpc.NewRPCFunc(env.Commit, "height", true),
		"validators":       rpc.NewRPCFunc(env.Validators, "height,page,per_page", true),
		"tx":               rpc.NewRPCFunc(env.Tx, "hash,prove", true),
		"tx_search":        rpc.NewRPCFunc(env.TxSearch, "query,prove,page,per_page,order_by", false),
		"block_search":     rpc.NewRPCFunc(env.BlockSearch, "query,page,per_page,order_by", false),
	}
}
