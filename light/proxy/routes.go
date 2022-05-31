package proxy

import (
	"context"

	lrpc "github.com/tendermint/tendermint/light/rpc"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/coretypes"
)

// proxyService wraps a light RPC client to export the RPC service interfaces.
// The interfaces are implemented by delegating to the underlying node via the
// specified client.
type proxyService struct {
	Client *lrpc.Client
}

func (p proxyService) ABCIInfo(ctx context.Context) (*coretypes.ResultABCIInfo, error) { panic("ok") }

func (p proxyService) ABCIQuery(ctx context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
	return p.Client.ABCIQueryWithOptions(ctx, req.Path, req.Data, rpcclient.ABCIQueryOptions{
		Height: int64(req.Height),
		Prove:  req.Prove,
	})
}

func (p proxyService) Block(ctx context.Context, req *coretypes.RequestBlockInfo) (*coretypes.ResultBlock, error) {
	return p.Client.Block(ctx, (*int64)(req.Height))
}

func (p proxyService) BlockByHash(ctx context.Context, req *coretypes.RequestBlockByHash) (*coretypes.ResultBlock, error) {
	return p.Client.BlockByHash(ctx, req.Hash)
}

func (p proxyService) BlockResults(ctx context.Context, req *coretypes.RequestBlockInfo) (*coretypes.ResultBlockResults, error) {
	return p.Client.BlockResults(ctx, (*int64)(req.Height))
}

func (p proxyService) BlockSearch(ctx context.Context, req *coretypes.RequestBlockSearch) (*coretypes.ResultBlockSearch, error) {
	return p.Client.BlockSearch(ctx, req.Query, req.Page.IntPtr(), req.PerPage.IntPtr(), req.OrderBy)
}

func (p proxyService) BlockchainInfo(ctx context.Context, req *coretypes.RequestBlockchainInfo) (*coretypes.ResultBlockchainInfo, error) {
	return p.Client.BlockchainInfo(ctx, int64(req.MinHeight), int64(req.MaxHeight))
}

func (p proxyService) BroadcastEvidence(ctx context.Context, req *coretypes.RequestBroadcastEvidence) (*coretypes.ResultBroadcastEvidence, error) {
	return p.Client.BroadcastEvidence(ctx, req.Evidence)
}

func (p proxyService) BroadcastTxAsync(ctx context.Context, req *coretypes.RequestBroadcastTx) (*coretypes.ResultBroadcastTx, error) {
	return p.Client.BroadcastTxAsync(ctx, req.Tx)
}

func (p proxyService) BroadcastTx(ctx context.Context, req *coretypes.RequestBroadcastTx) (*coretypes.ResultBroadcastTx, error) {
	return p.Client.BroadcastTx(ctx, req.Tx)
}

func (p proxyService) BroadcastTxCommit(ctx context.Context, req *coretypes.RequestBroadcastTx) (*coretypes.ResultBroadcastTxCommit, error) {
	return p.Client.BroadcastTxCommit(ctx, req.Tx)
}

func (p proxyService) BroadcastTxSync(ctx context.Context, req *coretypes.RequestBroadcastTx) (*coretypes.ResultBroadcastTx, error) {
	return p.Client.BroadcastTxSync(ctx, req.Tx)
}

func (p proxyService) CheckTx(ctx context.Context, req *coretypes.RequestCheckTx) (*coretypes.ResultCheckTx, error) {
	return p.Client.CheckTx(ctx, req.Tx)
}

func (p proxyService) Commit(ctx context.Context, req *coretypes.RequestBlockInfo) (*coretypes.ResultCommit, error) {
	return p.Client.Commit(ctx, (*int64)(req.Height))
}

func (p proxyService) ConsensusParams(ctx context.Context, req *coretypes.RequestConsensusParams) (*coretypes.ResultConsensusParams, error) {
	return p.Client.ConsensusParams(ctx, (*int64)(req.Height))
}

func (p proxyService) DumpConsensusState(ctx context.Context) (*coretypes.ResultDumpConsensusState, error) {
	return p.Client.DumpConsensusState(ctx)
}

func (p proxyService) Events(ctx context.Context, req *coretypes.RequestEvents) (*coretypes.ResultEvents, error) {
	return p.Client.Events(ctx, req)
}

func (p proxyService) Genesis(ctx context.Context) (*coretypes.ResultGenesis, error) {
	return p.Client.Genesis(ctx)
}

func (p proxyService) GenesisChunked(ctx context.Context, req *coretypes.RequestGenesisChunked) (*coretypes.ResultGenesisChunk, error) {
	return p.Client.GenesisChunked(ctx, uint(req.Chunk))
}

func (p proxyService) GetConsensusState(ctx context.Context) (*coretypes.ResultConsensusState, error) {
	return p.Client.ConsensusState(ctx)
}

func (p proxyService) Header(ctx context.Context, req *coretypes.RequestBlockInfo) (*coretypes.ResultHeader, error) {
	return p.Client.Header(ctx, (*int64)(req.Height))
}

func (p proxyService) HeaderByHash(ctx context.Context, req *coretypes.RequestBlockByHash) (*coretypes.ResultHeader, error) {
	return p.Client.HeaderByHash(ctx, req.Hash)
}

func (p proxyService) Health(ctx context.Context) (*coretypes.ResultHealth, error) {
	return p.Client.Health(ctx)
}

func (p proxyService) NetInfo(ctx context.Context) (*coretypes.ResultNetInfo, error) {
	return p.Client.NetInfo(ctx)
}

func (p proxyService) NumUnconfirmedTxs(ctx context.Context) (*coretypes.ResultUnconfirmedTxs, error) {
	return p.Client.NumUnconfirmedTxs(ctx)
}

func (p proxyService) RemoveTx(ctx context.Context, req *coretypes.RequestRemoveTx) error {
	return p.Client.RemoveTx(ctx, req.TxKey)
}

func (p proxyService) Status(ctx context.Context) (*coretypes.ResultStatus, error) {
	return p.Client.Status(ctx)
}

func (p proxyService) Subscribe(ctx context.Context, req *coretypes.RequestSubscribe) (*coretypes.ResultSubscribe, error) {
	return p.Client.SubscribeWS(ctx, req.Query)
}

func (p proxyService) Tx(ctx context.Context, req *coretypes.RequestTx) (*coretypes.ResultTx, error) {
	return p.Client.Tx(ctx, req.Hash, req.Prove)
}

func (p proxyService) TxSearch(ctx context.Context, req *coretypes.RequestTxSearch) (*coretypes.ResultTxSearch, error) {
	return p.Client.TxSearch(ctx, req.Query, req.Prove, req.Page.IntPtr(), req.PerPage.IntPtr(), req.OrderBy)
}

func (p proxyService) UnconfirmedTxs(ctx context.Context, req *coretypes.RequestUnconfirmedTxs) (*coretypes.ResultUnconfirmedTxs, error) {
	return p.Client.UnconfirmedTxs(ctx, req.Page.IntPtr(), req.PerPage.IntPtr())
}

func (p proxyService) Unsubscribe(ctx context.Context, req *coretypes.RequestUnsubscribe) (*coretypes.ResultUnsubscribe, error) {
	return p.Client.UnsubscribeWS(ctx, req.Query)
}

func (p proxyService) UnsubscribeAll(ctx context.Context) (*coretypes.ResultUnsubscribe, error) {
	return p.Client.UnsubscribeAllWS(ctx)
}

func (p proxyService) Validators(ctx context.Context, req *coretypes.RequestValidators) (*coretypes.ResultValidators, error) {
	return p.Client.Validators(ctx, (*int64)(req.Height), req.Page.IntPtr(), req.PerPage.IntPtr())
}
