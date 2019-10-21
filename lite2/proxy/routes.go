package proxy

import (
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/rpc/client"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tendermint/types"
)

// RPCRoutes just routes everything to the given client, as if it were a
// tendermint fullnode.
func RPCRoutes(c rpcclient.Client) map[string]*rpcserver.RPCFunc {
	return map[string]*rpcserver.RPCFunc{
		// Subscribe/unsubscribe are reserved for websocket events.
		"subscribe":       rpcserver.NewWSRPCFunc(c.(Wrapper).SubscribeWS, "query"),
		"unsubscribe":     rpcserver.NewWSRPCFunc(c.(Wrapper).UnsubscribeWS, "query"),
		"unsubscribe_all": rpcserver.NewWSRPCFunc(c.(Wrapper).UnsubscribeAllWS, ""),

		// info API
		"health":               rpc.NewRPCFunc(makeHealthFunc(c), ""),
		"status":               rpc.NewRPCFunc(makeStatusFunc(c), ""),
		"net_info":             rpc.NewRPCFunc(makeNetInfoFunc(c), ""),
		"blockchain":           rpc.NewRPCFunc(makeBlockchainInfoFunc(c), "minHeight,maxHeight"),
		"genesis":              rpc.NewRPCFunc(makeGenesisFunc(c), ""),
		"block":                rpc.NewRPCFunc(makeBlockFunc(c), "height"),
		"block_results":        rpc.NewRPCFunc(makeBlockResultsFunc(c), "height"),
		"commit":               rpc.NewRPCFunc(makeCommitFunc(c), "height"),
		"tx":                   rpc.NewRPCFunc(makeTxFunc(c), "hash,prove"),
		"tx_search":            rpc.NewRPCFunc(makeTxSearchFunc(c), "query,prove,page,per_page"),
		"validators":           rpc.NewRPCFunc(makeValidatorsFunc(c), "height"),
		"dump_consensus_state": rpc.NewRPCFunc(makeDumpConsensusStateFunc(c), ""),
		"consensus_state":      rpc.NewRPCFunc(makeConsensusStateFunc(c), ""),
		"consensus_params":     rpc.NewRPCFunc(makeConsensusParamsFunc(c), "height"),
		"unconfirmed_txs":      rpc.NewRPCFunc(makeUnconfirmedTxsFunc(c), "limit"),
		"num_unconfirmed_txs":  rpc.NewRPCFunc(makeNumUnconfirmedTxsFunc(c), ""),

		// tx broadcast API
		"broadcast_tx_commit": rpc.NewRPCFunc(makeBroadcastTxCommitFunc(c), "tx"),
		"broadcast_tx_sync":   rpc.NewRPCFunc(makeBroadcastTxSyncFunc(c), "tx"),
		"broadcast_tx_async":  rpc.NewRPCFunc(makeBroadcastTxAsyncFunc(c), "tx"),

		// abci API
		"abci_query": rpc.NewRPCFunc(makeABCIQueryFunc(c), "path,data,height,prove"),
		"abci_info":  rpc.NewRPCFunc(makeABCIInfoFunc(c), ""),

		// evidence API
		"broadcast_evidence": rpc.NewRPCFunc(makeBroadcastEvidenceFunc(c), "evidence"),
	}
}

func makeHealthFunc(c client.StatusClient) func(ctx *rpctypes.Context) (*ctypes.ResultHealth, error) {
	return func(ctx *rpctypes.Context) (*ctypes.ResultHealth, error) {
		return c.Health()
	}
}

func makeStatusFunc(c client.StatusClient) func(ctx *rpctypes.Context) (*ctypes.ResultStatus, error) {
	return func(ctx *rpctypes.Context) (*ctypes.ResultStatus, error) {
		return c.Status()
	}
}

func makeNetInfoFunc(c rpcclient.Client) func(ctx *rpctypes.Context) (*ctypes.ResultNetInfo, error) {
	return func(ctx *rpctypes.Context, minHeight, maxHeight int64) (*ctypes.ResultNetInfo, error) {
		return c.NetInfo()
	}
}

func makeBlockchainInfoFunc(c rpcclient.Client) func(ctx *rpctypes.Context, minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error) {
	return func(ctx *rpctypes.Context, minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error) {
		return c.BlockchainInfo(minHeight, maxHeight)
	}
}

func makeGenesisFunc(c rpcclient.Client) func(ctx *rpctypes.Context) (*ctypes.ResultGenesis, error) {
	return func(ctx *rpctypes.Context) (*ctypes.ResultGenesis, error) {
		return c.Genesis()
	}
}

func makeBlockFunc(c rpcclient.Client) func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultBlock, error) {
	return func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultBlock, error) {
		return c.Block(height)
	}
}

func makeBlockResultsFunc(c rpcclient.Client) func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultBlockResults, error) {
	return func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultBlockResults, error) {
		return c.BlockResults(height)
	}
}

func makeCommitFunc(c rpcclient.Client) func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultCommit, error) {
	return func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultCommit, error) {
		return c.Commit(height)
	}
}

func makeTxFunc(c rpcclient.Client) func(ctx *rpctypes.Context, hash []byte, prove bool) (*ctypes.ResultTx, error) {
	return func(ctx *rpctypes.Context, hash []byte, prove bool) (*ctypes.ResultTx, error) {
		return c.Tx(hash, prove)
	}
}

func makeTxSearchFunc(c rpcclient.Client) func(ctx *rpctypes.Context, query string, prove bool, page, perPage int) (*ctypes.ResultTxSearch, error) {
	return func(ctx *rpctypes.Context, query string, prove bool, page, perPage int) (*ctypes.ResultTxSearch, error) {
		return c.TxSearch(query, prove, page, perPage)
	}
}

func makeValidatorsFunc(c rpcclient.Client) func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultValidators, error) {
	return func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultValidators, error) {
		return c.Validators(height)
	}
}

func makeDumpConsensusStateFunc(c rpcclient.Client) func(ctx *rpctypes.Context) (*ctypes.ResultDumpConsensusState, error) {
	return func(ctx *rpctypes.Context) (*ctypes.ResultDumpConsensusState, error) {
		return c.DumpConsensusState()
	}
}

func makeConsensusStateFunc(c rpcclient.Client) func(ctx *rpctypes.Context) (*ctypes.ResultConsensusState, error) {
	return func(ctx *rpctypes.Context) (*ctypes.ResultConsensusState, error) {
		return c.ConsensusState()
	}
}

func makeConsensusParamsFunc(c rpcclient.Client) func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultConsensusParams, error) {
	return func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultConsensusParams, error) {
		return c.ConsensusParams(height)
	}
}

func makeUnconfirmedTxsFunc(c rpcclient.Client) func(ctx *rpctypes.Context, limit int) (*ctypes.ResultUnconfirmedTxs, error) {
	return func(ctx *rpctypes.Context, limit int) (*ctypes.ResultUnconfirmedTxs, error) {
		return c.UnconfirmedTxs(limit)
	}
}

func makeNumUnconfirmedTxsFunc(c rpcclient.Client) func(ctx *rpctypes.Context) (*ctypes.ResultUnconfirmedTxs, error) {
	return func(ctx *rpctypes.Context) (*ctypes.ResultUnconfirmedTxs, error) {
		return c.NumUnconfirmedTxs()
	}
}

func makeBroadcastTxCommitFunc(c rpcclient.Client) func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	return func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
		return c.BroadcastTxCommit(tx)
	}
}

func makeBroadcastTxSyncFunc(c rpcclient.Client) func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
		return c.BroadcastTxSync(tx)
	}
}

func makeBroadcastTxAsyncFunc(c rpcclient.Client) func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
		return c.BroadcastTxAsync(tx)
	}
}

func makeABCIQueryFunc(c rpcclient.Client) func(ctx *rpctypes.Context, path string, data cmn.HexBytes) (*ctypes.ResultABCIQuery, error) {
	return func(ctx *rpctypes.Context, path string, data cmn.HexBytes) (*ctypes.ResultABCIQuery, error) {
		return c.ABCIQuery(path, data)
	}
}

func makeABCIInfoFunc(c rpcclient.Client) func(ctx *rpctypes.Context) (*ctypes.ResultABCIInfo, error) {
	return func(ctx *rpctypes.Context) (*ctypes.ResultABCIInfo, error) {
		return c.ABCIInfo()
	}
}

func makeBroadcastEvidenceFunc(c rpcclient.Client) func(ctx *rpctypes.Context, ev types.Evidence) (*ctypes.ResultBroadcastEvidence, error) {
	return func(ctx *rpctypes.Context, ev types.Evidence) (*ctypes.ResultBroadcastEvidence, error) {
		return c.BroadcastEvidence(ev)
	}
}
