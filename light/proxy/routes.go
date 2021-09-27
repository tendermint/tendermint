package proxy

import (
	"github.com/tendermint/tendermint/libs/bytes"
	lrpc "github.com/tendermint/tendermint/light/rpc"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/coretypes"
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
		"health":               rpcserver.NewRPCFunc(makeHealthFunc(c), "", false),
		"status":               rpcserver.NewRPCFunc(makeStatusFunc(c), "", false),
		"net_info":             rpcserver.NewRPCFunc(makeNetInfoFunc(c), "", false),
		"blockchain":           rpcserver.NewRPCFunc(makeBlockchainInfoFunc(c), "minHeight,maxHeight", true),
		"genesis":              rpcserver.NewRPCFunc(makeGenesisFunc(c), "", true),
		"genesis_chunked":      rpcserver.NewRPCFunc(makeGenesisChunkedFunc(c), "", true),
		"block":                rpcserver.NewRPCFunc(makeBlockFunc(c), "height", true),
		"block_by_hash":        rpcserver.NewRPCFunc(makeBlockByHashFunc(c), "hash", true),
		"block_results":        rpcserver.NewRPCFunc(makeBlockResultsFunc(c), "height", true),
		"commit":               rpcserver.NewRPCFunc(makeCommitFunc(c), "height", true),
		"tx":                   rpcserver.NewRPCFunc(makeTxFunc(c), "hash,prove", true),
		"tx_search":            rpcserver.NewRPCFunc(makeTxSearchFunc(c), "query,prove,page,per_page,order_by", false),
		"block_search":         rpcserver.NewRPCFunc(makeBlockSearchFunc(c), "query,page,per_page,order_by", false),
		"validators":           rpcserver.NewRPCFunc(makeValidatorsFunc(c), "height,page,per_page", true),
		"dump_consensus_state": rpcserver.NewRPCFunc(makeDumpConsensusStateFunc(c), "", false),
		"consensus_state":      rpcserver.NewRPCFunc(makeConsensusStateFunc(c), "", false),
		"consensus_params":     rpcserver.NewRPCFunc(makeConsensusParamsFunc(c), "height", true),
		"unconfirmed_txs":      rpcserver.NewRPCFunc(makeUnconfirmedTxsFunc(c), "limit", false),
		"num_unconfirmed_txs":  rpcserver.NewRPCFunc(makeNumUnconfirmedTxsFunc(c), "", false),

		// tx broadcast API
		"broadcast_tx_commit": rpcserver.NewRPCFunc(makeBroadcastTxCommitFunc(c), "tx", false),
		"broadcast_tx_sync":   rpcserver.NewRPCFunc(makeBroadcastTxSyncFunc(c), "tx", false),
		"broadcast_tx_async":  rpcserver.NewRPCFunc(makeBroadcastTxAsyncFunc(c), "tx", false),

		// abci API
		"abci_query": rpcserver.NewRPCFunc(makeABCIQueryFunc(c), "path,data,height,prove", false),
		"abci_info":  rpcserver.NewRPCFunc(makeABCIInfoFunc(c), "", true),

		// evidence API
		"broadcast_evidence": rpcserver.NewRPCFunc(makeBroadcastEvidenceFunc(c), "evidence", false),
	}
}

type rpcHealthFunc func(ctx *rpctypes.Context) (*coretypes.ResultHealth, error)

func makeHealthFunc(c *lrpc.Client) rpcHealthFunc {
	return func(ctx *rpctypes.Context) (*coretypes.ResultHealth, error) {
		return c.Health(ctx.Context())
	}
}

type rpcStatusFunc func(ctx *rpctypes.Context) (*coretypes.ResultStatus, error)

// nolint: interfacer
func makeStatusFunc(c *lrpc.Client) rpcStatusFunc {
	return func(ctx *rpctypes.Context) (*coretypes.ResultStatus, error) {
		return c.Status(ctx.Context())
	}
}

type rpcNetInfoFunc func(ctx *rpctypes.Context) (*coretypes.ResultNetInfo, error)

func makeNetInfoFunc(c *lrpc.Client) rpcNetInfoFunc {
	return func(ctx *rpctypes.Context) (*coretypes.ResultNetInfo, error) {
		return c.NetInfo(ctx.Context())
	}
}

type rpcBlockchainInfoFunc func(ctx *rpctypes.Context, minHeight, maxHeight int64) (*coretypes.ResultBlockchainInfo, error) //nolint:lll

func makeBlockchainInfoFunc(c *lrpc.Client) rpcBlockchainInfoFunc {
	return func(ctx *rpctypes.Context, minHeight, maxHeight int64) (*coretypes.ResultBlockchainInfo, error) {
		return c.BlockchainInfo(ctx.Context(), minHeight, maxHeight)
	}
}

type rpcGenesisFunc func(ctx *rpctypes.Context) (*coretypes.ResultGenesis, error)

func makeGenesisFunc(c *lrpc.Client) rpcGenesisFunc {
	return func(ctx *rpctypes.Context) (*coretypes.ResultGenesis, error) {
		return c.Genesis(ctx.Context())
	}
}

type rpcGenesisChunkedFunc func(ctx *rpctypes.Context, chunk uint) (*coretypes.ResultGenesisChunk, error)

func makeGenesisChunkedFunc(c *lrpc.Client) rpcGenesisChunkedFunc {
	return func(ctx *rpctypes.Context, chunk uint) (*coretypes.ResultGenesisChunk, error) {
		return c.GenesisChunked(ctx.Context(), chunk)
	}
}

type rpcBlockFunc func(ctx *rpctypes.Context, height *int64) (*coretypes.ResultBlock, error)

func makeBlockFunc(c *lrpc.Client) rpcBlockFunc {
	return func(ctx *rpctypes.Context, height *int64) (*coretypes.ResultBlock, error) {
		return c.Block(ctx.Context(), height)
	}
}

type rpcBlockByHashFunc func(ctx *rpctypes.Context, hash []byte) (*coretypes.ResultBlock, error)

func makeBlockByHashFunc(c *lrpc.Client) rpcBlockByHashFunc {
	return func(ctx *rpctypes.Context, hash []byte) (*coretypes.ResultBlock, error) {
		return c.BlockByHash(ctx.Context(), hash)
	}
}

type rpcBlockResultsFunc func(ctx *rpctypes.Context, height *int64) (*coretypes.ResultBlockResults, error)

func makeBlockResultsFunc(c *lrpc.Client) rpcBlockResultsFunc {
	return func(ctx *rpctypes.Context, height *int64) (*coretypes.ResultBlockResults, error) {
		return c.BlockResults(ctx.Context(), height)
	}
}

type rpcCommitFunc func(ctx *rpctypes.Context, height *int64) (*coretypes.ResultCommit, error)

func makeCommitFunc(c *lrpc.Client) rpcCommitFunc {
	return func(ctx *rpctypes.Context, height *int64) (*coretypes.ResultCommit, error) {
		return c.Commit(ctx.Context(), height)
	}
}

type rpcTxFunc func(ctx *rpctypes.Context, hash []byte, prove bool) (*coretypes.ResultTx, error)

func makeTxFunc(c *lrpc.Client) rpcTxFunc {
	return func(ctx *rpctypes.Context, hash []byte, prove bool) (*coretypes.ResultTx, error) {
		return c.Tx(ctx.Context(), hash, prove)
	}
}

type rpcTxSearchFunc func(
	ctx *rpctypes.Context,
	query string,
	prove bool,
	page, perPage *int,
	orderBy string,
) (*coretypes.ResultTxSearch, error)

func makeTxSearchFunc(c *lrpc.Client) rpcTxSearchFunc {
	return func(
		ctx *rpctypes.Context,
		query string,
		prove bool,
		page, perPage *int,
		orderBy string,
	) (*coretypes.ResultTxSearch, error) {
		return c.TxSearch(ctx.Context(), query, prove, page, perPage, orderBy)
	}
}

type rpcBlockSearchFunc func(
	ctx *rpctypes.Context,
	query string,
	prove bool,
	page, perPage *int,
	orderBy string,
) (*coretypes.ResultBlockSearch, error)

func makeBlockSearchFunc(c *lrpc.Client) rpcBlockSearchFunc {
	return func(
		ctx *rpctypes.Context,
		query string,
		prove bool,
		page, perPage *int,
		orderBy string,
	) (*coretypes.ResultBlockSearch, error) {
		return c.BlockSearch(ctx.Context(), query, page, perPage, orderBy)
	}
}

type rpcValidatorsFunc func(ctx *rpctypes.Context, height *int64,
	page, perPage *int) (*coretypes.ResultValidators, error)

func makeValidatorsFunc(c *lrpc.Client) rpcValidatorsFunc {
	return func(ctx *rpctypes.Context, height *int64, page, perPage *int) (*coretypes.ResultValidators, error) {
		return c.Validators(ctx.Context(), height, page, perPage)
	}
}

type rpcDumpConsensusStateFunc func(ctx *rpctypes.Context) (*coretypes.ResultDumpConsensusState, error)

func makeDumpConsensusStateFunc(c *lrpc.Client) rpcDumpConsensusStateFunc {
	return func(ctx *rpctypes.Context) (*coretypes.ResultDumpConsensusState, error) {
		return c.DumpConsensusState(ctx.Context())
	}
}

type rpcConsensusStateFunc func(ctx *rpctypes.Context) (*coretypes.ResultConsensusState, error)

func makeConsensusStateFunc(c *lrpc.Client) rpcConsensusStateFunc {
	return func(ctx *rpctypes.Context) (*coretypes.ResultConsensusState, error) {
		return c.ConsensusState(ctx.Context())
	}
}

type rpcConsensusParamsFunc func(ctx *rpctypes.Context, height *int64) (*coretypes.ResultConsensusParams, error)

func makeConsensusParamsFunc(c *lrpc.Client) rpcConsensusParamsFunc {
	return func(ctx *rpctypes.Context, height *int64) (*coretypes.ResultConsensusParams, error) {
		return c.ConsensusParams(ctx.Context(), height)
	}
}

type rpcUnconfirmedTxsFunc func(ctx *rpctypes.Context, limit *int) (*coretypes.ResultUnconfirmedTxs, error)

func makeUnconfirmedTxsFunc(c *lrpc.Client) rpcUnconfirmedTxsFunc {
	return func(ctx *rpctypes.Context, limit *int) (*coretypes.ResultUnconfirmedTxs, error) {
		return c.UnconfirmedTxs(ctx.Context(), limit)
	}
}

type rpcNumUnconfirmedTxsFunc func(ctx *rpctypes.Context) (*coretypes.ResultUnconfirmedTxs, error)

func makeNumUnconfirmedTxsFunc(c *lrpc.Client) rpcNumUnconfirmedTxsFunc {
	return func(ctx *rpctypes.Context) (*coretypes.ResultUnconfirmedTxs, error) {
		return c.NumUnconfirmedTxs(ctx.Context())
	}
}

type rpcBroadcastTxCommitFunc func(ctx *rpctypes.Context, tx types.Tx) (*coretypes.ResultBroadcastTxCommit, error)

func makeBroadcastTxCommitFunc(c *lrpc.Client) rpcBroadcastTxCommitFunc {
	return func(ctx *rpctypes.Context, tx types.Tx) (*coretypes.ResultBroadcastTxCommit, error) {
		return c.BroadcastTxCommit(ctx.Context(), tx)
	}
}

type rpcBroadcastTxSyncFunc func(ctx *rpctypes.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error)

func makeBroadcastTxSyncFunc(c *lrpc.Client) rpcBroadcastTxSyncFunc {
	return func(ctx *rpctypes.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
		return c.BroadcastTxSync(ctx.Context(), tx)
	}
}

type rpcBroadcastTxAsyncFunc func(ctx *rpctypes.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error)

func makeBroadcastTxAsyncFunc(c *lrpc.Client) rpcBroadcastTxAsyncFunc {
	return func(ctx *rpctypes.Context, tx types.Tx) (*coretypes.ResultBroadcastTx, error) {
		return c.BroadcastTxAsync(ctx.Context(), tx)
	}
}

type rpcABCIQueryFunc func(ctx *rpctypes.Context, path string,
	data bytes.HexBytes, height int64, prove bool) (*coretypes.ResultABCIQuery, error)

func makeABCIQueryFunc(c *lrpc.Client) rpcABCIQueryFunc {
	return func(ctx *rpctypes.Context, path string, data bytes.HexBytes,
		height int64, prove bool) (*coretypes.ResultABCIQuery, error) {

		return c.ABCIQueryWithOptions(ctx.Context(), path, data, rpcclient.ABCIQueryOptions{
			Height: height,
			Prove:  prove,
		})
	}
}

type rpcABCIInfoFunc func(ctx *rpctypes.Context) (*coretypes.ResultABCIInfo, error)

func makeABCIInfoFunc(c *lrpc.Client) rpcABCIInfoFunc {
	return func(ctx *rpctypes.Context) (*coretypes.ResultABCIInfo, error) {
		return c.ABCIInfo(ctx.Context())
	}
}

type rpcBroadcastEvidenceFunc func(ctx *rpctypes.Context, ev types.Evidence) (*coretypes.ResultBroadcastEvidence, error)

// nolint: interfacer
func makeBroadcastEvidenceFunc(c *lrpc.Client) rpcBroadcastEvidenceFunc {
	return func(ctx *rpctypes.Context, ev types.Evidence) (*coretypes.ResultBroadcastEvidence, error) {
		return c.BroadcastEvidence(ctx.Context(), ev)
	}
}
