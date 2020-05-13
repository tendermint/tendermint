package proxy

import (
	"github.com/tendermint/tendermint/libs/bytes"
	lrpc "github.com/tendermint/tendermint/lite2/rpc"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpcserver "github.com/tendermint/tendermint/rpc/jsonrpc/server"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
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
		"tx_search":            rpcserver.NewRPCFunc(makeTxSearchFunc(c), "query,prove,page,per_page,order_by"),
		"validators":           rpcserver.NewRPCFunc(makeValidatorsFunc(c), "height,page,per_page"),
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

type rpcHealthFunc func(ctx *rpctypes.Context) (*ctypes.ResultHealth, error)

func makeHealthFunc(c *lrpc.Client) rpcHealthFunc {
	return func(ctx *rpctypes.Context) (*ctypes.ResultHealth, error) {
		return c.Health()
	}
}

type rpcStatusFunc func(ctx *rpctypes.Context) (*ctypes.ResultStatus, error)

// nolint: interfacer
func makeStatusFunc(c *lrpc.Client) rpcStatusFunc {
	return func(ctx *rpctypes.Context) (*ctypes.ResultStatus, error) {
		return c.Status()
	}
}

type rpcNetInfoFunc func(ctx *rpctypes.Context, minHeight, maxHeight int64) (*ctypes.ResultNetInfo, error)

func makeNetInfoFunc(c *lrpc.Client) rpcNetInfoFunc {
	return func(ctx *rpctypes.Context, minHeight, maxHeight int64) (*ctypes.ResultNetInfo, error) {
		return c.NetInfo()
	}
}

type rpcBlockchainInfoFunc func(ctx *rpctypes.Context, minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error)

func makeBlockchainInfoFunc(c *lrpc.Client) rpcBlockchainInfoFunc {
	return func(ctx *rpctypes.Context, minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error) {
		return c.BlockchainInfo(minHeight, maxHeight)
	}
}

type rpcGenesisFunc func(ctx *rpctypes.Context) (*ctypes.ResultGenesis, error)

func makeGenesisFunc(c *lrpc.Client) rpcGenesisFunc {
	return func(ctx *rpctypes.Context) (*ctypes.ResultGenesis, error) {
		return c.Genesis()
	}
}

type rpcBlockFunc func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultBlock, error)

func makeBlockFunc(c *lrpc.Client) rpcBlockFunc {
	return func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultBlock, error) {
		return c.Block(height)
	}
}

type rpcBlockResultsFunc func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultBlockResults, error)

func makeBlockResultsFunc(c *lrpc.Client) rpcBlockResultsFunc {
	return func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultBlockResults, error) {
		return c.BlockResults(height)
	}
}

type rpcCommitFunc func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultCommit, error)

func makeCommitFunc(c *lrpc.Client) rpcCommitFunc {
	return func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultCommit, error) {
		return c.Commit(height)
	}
}

type rpcTxFunc func(ctx *rpctypes.Context, hash []byte, prove bool) (*ctypes.ResultTx, error)

func makeTxFunc(c *lrpc.Client) rpcTxFunc {
	return func(ctx *rpctypes.Context, hash []byte, prove bool) (*ctypes.ResultTx, error) {
		return c.Tx(hash, prove)
	}
}

type rpcTxSearchFunc func(ctx *rpctypes.Context, query string, prove bool,
	page, perPage int, orderBy string) (*ctypes.ResultTxSearch, error)

func makeTxSearchFunc(c *lrpc.Client) rpcTxSearchFunc {
	return func(ctx *rpctypes.Context, query string, prove bool, page, perPage int, orderBy string) (
		*ctypes.ResultTxSearch, error) {
		return c.TxSearch(query, prove, page, perPage, orderBy)
	}
}

type rpcValidatorsFunc func(ctx *rpctypes.Context, height *int64,
	page, perPage int) (*ctypes.ResultValidators, error)

func makeValidatorsFunc(c *lrpc.Client) rpcValidatorsFunc {
	return func(ctx *rpctypes.Context, height *int64, page, perPage int) (*ctypes.ResultValidators, error) {
		return c.Validators(height, page, perPage)
	}
}

type rpcDumpConsensusStateFunc func(ctx *rpctypes.Context) (*ctypes.ResultDumpConsensusState, error)

func makeDumpConsensusStateFunc(c *lrpc.Client) rpcDumpConsensusStateFunc {
	return func(ctx *rpctypes.Context) (*ctypes.ResultDumpConsensusState, error) {
		return c.DumpConsensusState()
	}
}

type rpcConsensusStateFunc func(ctx *rpctypes.Context) (*ctypes.ResultConsensusState, error)

func makeConsensusStateFunc(c *lrpc.Client) rpcConsensusStateFunc {
	return func(ctx *rpctypes.Context) (*ctypes.ResultConsensusState, error) {
		return c.ConsensusState()
	}
}

type rpcConsensusParamsFunc func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultConsensusParams, error)

func makeConsensusParamsFunc(c *lrpc.Client) rpcConsensusParamsFunc {
	return func(ctx *rpctypes.Context, height *int64) (*ctypes.ResultConsensusParams, error) {
		return c.ConsensusParams(height)
	}
}

type rpcUnconfirmedTxsFunc func(ctx *rpctypes.Context, limit int) (*ctypes.ResultUnconfirmedTxs, error)

func makeUnconfirmedTxsFunc(c *lrpc.Client) rpcUnconfirmedTxsFunc {
	return func(ctx *rpctypes.Context, limit int) (*ctypes.ResultUnconfirmedTxs, error) {
		return c.UnconfirmedTxs(limit)
	}
}

type rpcNumUnconfirmedTxsFunc func(ctx *rpctypes.Context) (*ctypes.ResultUnconfirmedTxs, error)

func makeNumUnconfirmedTxsFunc(c *lrpc.Client) rpcNumUnconfirmedTxsFunc {
	return func(ctx *rpctypes.Context) (*ctypes.ResultUnconfirmedTxs, error) {
		return c.NumUnconfirmedTxs()
	}
}

type rpcBroadcastTxCommitFunc func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error)

func makeBroadcastTxCommitFunc(c *lrpc.Client) rpcBroadcastTxCommitFunc {
	return func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
		return c.BroadcastTxCommit(tx)
	}
}

type rpcBroadcastTxSyncFunc func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error)

func makeBroadcastTxSyncFunc(c *lrpc.Client) rpcBroadcastTxSyncFunc {
	return func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
		return c.BroadcastTxSync(tx)
	}
}

type rpcBroadcastTxAsyncFunc func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error)

func makeBroadcastTxAsyncFunc(c *lrpc.Client) rpcBroadcastTxAsyncFunc {
	return func(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
		return c.BroadcastTxAsync(tx)
	}
}

type rpcABCIQueryFunc func(ctx *rpctypes.Context, path string, data bytes.HexBytes) (*ctypes.ResultABCIQuery, error)

func makeABCIQueryFunc(c *lrpc.Client) rpcABCIQueryFunc {
	return func(ctx *rpctypes.Context, path string, data bytes.HexBytes) (*ctypes.ResultABCIQuery, error) {
		return c.ABCIQuery(path, data)
	}
}

type rpcABCIInfoFunc func(ctx *rpctypes.Context) (*ctypes.ResultABCIInfo, error)

func makeABCIInfoFunc(c *lrpc.Client) rpcABCIInfoFunc {
	return func(ctx *rpctypes.Context) (*ctypes.ResultABCIInfo, error) {
		return c.ABCIInfo()
	}
}

type rpcBroadcastEvidenceFunc func(ctx *rpctypes.Context, ev types.Evidence) (*ctypes.ResultBroadcastEvidence, error)

// nolint: interfacer
func makeBroadcastEvidenceFunc(c *lrpc.Client) rpcBroadcastEvidenceFunc {
	return func(ctx *rpctypes.Context, ev types.Evidence) (*ctypes.ResultBroadcastEvidence, error) {
		return c.BroadcastEvidence(ev)
	}
}
