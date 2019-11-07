package proxy

import (
	cmn "github.com/tendermint/tendermint/libs/common"
	lrpc "github.com/tendermint/tendermint/lite2/rpc"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpcserver "github.com/tendermint/tendermint/rpc/lib/server"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tendermint/types"
)

func RPCRoutes(c *lrpc.Client) map[string]*rpcserver.RPCFunc {
	return map[string]*rpcserver.RPCFunc{
		// Subscribe/unsubscribe are reserved for websocket events.
		"subscribe":       rpcserver.NewWSRPCFunc(c.SubscribeWS, "query"),
		"unsubscribe":     rpcserver.NewWSRPCFunc(c.UnsubscribeWS, "query"),
		"unsubscribe_all": rpcserver.NewWSRPCFunc(c.UnsubscribeAllWS, ""),

		// info API
		"health":               rpcserver.NewRPCFunc(makeHealthFunc(c), ""),
		"status":               rpcserver.NewRPCFunc(makeStatusFunc(c), ""),
		"net_info":             rpcserver.NewRPCFunc(makeNetInfoFunc(c), ""),
		"blockchain":           rpcserver.NewRPCFunc(makeBlockchainInfoFunc(c), "minHeight,maxHeight"),
		"genesis":              rpcserver.NewRPCFunc(makeGenesisFunc(c), ""),
		"block":                rpcserver.NewRPCFunc(makeBlockFunc(c), "height"),
		"block_results":        rpcserver.NewRPCFunc(makeBlockResultsFunc(c), "height"),
		"commit":               rpcserver.NewRPCFunc(makeCommitFunc(c), "height"),
		"tx":                   rpcserver.NewRPCFunc(makeTxFunc(c), "hash,prove"),
		"tx_search":            rpcserver.NewRPCFunc(makeTxSearchFunc(c), "query,prove,page,per_page"),
		"validators":           rpcserver.NewRPCFunc(makeValidatorsFunc(c), "height"),
		"dump_consensus_state": rpcserver.NewRPCFunc(makeDumpConsensusStateFunc(c), ""),
		"consensus_state":      rpcserver.NewRPCFunc(makeConsensusStateFunc(c), ""),
		"consensus_params":     rpcserver.NewRPCFunc(makeConsensusParamsFunc(c), "height"),
		"unconfirmed_txs":      rpcserver.NewRPCFunc(makeUnconfirmedTxsFunc(c), "limit"),
		"num_unconfirmed_txs":  rpcserver.NewRPCFunc(makeNumUnconfirmedTxsFunc(c), ""),

		// tx broadcast API
		"broadcast_tx_commit": rpcserver.NewRPCFunc(makeBroadcastTxCommitFunc(c), "tx"),
		"broadcast_tx_sync":   rpcserver.NewRPCFunc(makeBroadcastTxSyncFunc(c), "tx"),
		"broadcast_tx_async":  rpcserver.NewRPCFunc(makeBroadcastTxAsyncFunc(c), "tx"),

		// abci API
		"abci_query": rpcserver.NewRPCFunc(makeABCIQueryFunc(c), "path,data,height,prove"),
		"abci_info":  rpcserver.NewRPCFunc(makeABCIInfoFunc(c), ""),

		// evidence API
		"broadcast_evidence": rpcserver.NewRPCFunc(makeBroadcastEvidenceFunc(c), "evidence"),
	}
}

func makeHealthFunc(c *lrpc.Client) func(ctx *rpctypes.Context) (*ctypes.ResultHealth, error) {
	return func(ctx *rpctypes.Context) (*ctypes.ResultHealth, error) {
		return c.Health()
	}
}

func makeStatusFunc(c *lrpc.Client) func(ctx *rpctypes.Context) (*ctypes.ResultStatus, error) {
	return func(ctx *rpctypes.Context) (*ctypes.ResultStatus, error) {
		return c.Status()
	}
}

func makeNetInfoFunc(c *lrpc.Client) func(ctx *rpctypes.Context, minHeight, maxHeight int64) (*ctypes.ResultNetInfo, error) {
	return func(ctx *rpctypes.Context, minHeight, maxHeight int64) (*ctypes.ResultNetInfo, error) {
		return c.NetInfo()
	}
}

func makeBlockchainInfoFunc(c *lrpc.Client) func(ctx *rpctypes.Context, minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error) {
	return func(ctx *rpctypes.Context, minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error) {
		return c.BlockchainInfo(minHeight, maxHeight)
	}
}

func makeGenesisFunc(c *lrpc.Client) func(ctx *rpctypes.Context) (*ctypes.ResultGenesis, error) {
	return func(ctx *rpctypes.Context) (*ctypes.ResultGenesis, error) {
		return c.Genesis()
	}
}

func makeBlockFunc(c *lrpc.Client) func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultBlock, error) {
	return func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultBlock, error) {
		return c.Block(height)
	}
}

func makeBlockResultsFunc(c *lrpc.Client) func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultBlockResults, error) {
	return func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultBlockResults, error) {
		return c.BlockResults(height)
	}
}

func makeCommitFunc(c *lrpc.Client) func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultCommit, error) {
	return func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultCommit, error) {
		return c.Commit(height)
	}
}

func makeTxFunc(c *lrpc.Client) func(ctx *rpctypes.Context, hash []byte, prove bool) (*ctypes.ResultTx, error) {
	return func(ctx *rpctypes.Context, hash []byte, prove bool) (*ctypes.ResultTx, error) {
		return c.Tx(hash, prove)
	}
}

func makeTxSearchFunc(c *lrpc.Client) func(ctx *rpctypes.Context, query string, prove bool, page, perPage int) (*ctypes.ResultTxSearch, error) {
	return func(ctx *rpctypes.Context, query string, prove bool, page, perPage int) (*ctypes.ResultTxSearch, error) {
		return c.TxSearch(query, prove, page, perPage)
	}
}

func makeValidatorsFunc(c *lrpc.Client) func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultValidators, error) {
	return func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultValidators, error) {
		return c.Validators(height)
	}
}

func makeDumpConsensusStateFunc(c *lrpc.Client) func(ctx *rpctypes.Context) (*ctypes.ResultDumpConsensusState, error) {
	return func(ctx *rpctypes.Context) (*ctypes.ResultDumpConsensusState, error) {
		return c.DumpConsensusState()
	}
}

func makeConsensusStateFunc(c *lrpc.Client) func(ctx *rpctypes.Context) (*ctypes.ResultConsensusState, error) {
	return func(ctx *rpctypes.Context) (*ctypes.ResultConsensusState, error) {
		return c.ConsensusState()
	}
}

func makeConsensusParamsFunc(c *lrpc.Client) func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultConsensusParams, error) {
	return func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultConsensusParams, error) {
		return c.ConsensusParams(height)
	}
}

func makeUnconfirmedTxsFunc(c *lrpc.Client) func(ctx *rpctypes.Context, limit int) (*ctypes.ResultUnconfirmedTxs, error) {
	return func(ctx *rpctypes.Context, limit int) (*ctypes.ResultUnconfirmedTxs, error) {
		return c.UnconfirmedTxs(limit)
	}
}

func makeNumUnconfirmedTxsFunc(c *lrpc.Client) func(ctx *rpctypes.Context) (*ctypes.ResultUnconfirmedTxs, error) {
	return func(ctx *rpctypes.Context) (*ctypes.ResultUnconfirmedTxs, error) {
		return c.NumUnconfirmedTxs()
	}
}

func makeBroadcastTxCommitFunc(c *lrpc.Client) func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	return func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
		return c.BroadcastTxCommit(tx)
	}
}

func makeBroadcastTxSyncFunc(c *lrpc.Client) func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
		return c.BroadcastTxSync(tx)
	}
}

func makeBroadcastTxAsyncFunc(c *lrpc.Client) func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
		return c.BroadcastTxAsync(tx)
	}
}

func makeABCIQueryFunc(c *lrpc.Client) func(ctx *rpctypes.Context, path string, data cmn.HexBytes) (*ctypes.ResultABCIQuery, error) {
	return func(ctx *rpctypes.Context, path string, data cmn.HexBytes) (*ctypes.ResultABCIQuery, error) {
		return c.ABCIQuery(path, data)
	}
}

func makeABCIInfoFunc(c *lrpc.Client) func(ctx *rpctypes.Context) (*ctypes.ResultABCIInfo, error) {
	return func(ctx *rpctypes.Context) (*ctypes.ResultABCIInfo, error) {
		return c.ABCIInfo()
	}
}

func makeBroadcastEvidenceFunc(c *lrpc.Client) func(ctx *rpctypes.Context, ev types.Evidence) (*ctypes.ResultBroadcastEvidence, error) {
	return func(ctx *rpctypes.Context, ev types.Evidence) (*ctypes.ResultBroadcastEvidence, error) {
		return c.BroadcastEvidence(ev)
	}
}
