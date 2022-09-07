package consensus

import (
	"context"

	abciclient "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/libs/clist"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/internal/proxy"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------

type emptyMempool struct{}

var _ mempool.Mempool = emptyMempool{}

func (emptyMempool) Lock()     {}
func (emptyMempool) Unlock()   {}
func (emptyMempool) Size() int { return 0 }
func (emptyMempool) CheckTx(context.Context, types.Tx, func(*abci.ResponseCheckTx), mempool.TxInfo) error {
	return nil
}
func (emptyMempool) RemoveTxByKey(txKey types.TxKey) error   { return nil }
func (emptyMempool) ReapMaxBytesMaxGas(_, _ int64) types.Txs { return types.Txs{} }
func (emptyMempool) ReapMaxTxs(n int) types.Txs              { return types.Txs{} }
func (emptyMempool) Update(
	_ context.Context,
	_ int64,
	_ types.Txs,
	_ []*abci.ExecTxResult,
	_ mempool.PreCheckFunc,
	_ mempool.PostCheckFunc,
	_ bool,
) error {
	return nil
}
func (emptyMempool) Flush()                                 {}
func (emptyMempool) FlushAppConn(ctx context.Context) error { return nil }
func (emptyMempool) TxsAvailable() <-chan struct{}          { return make(chan struct{}) }
func (emptyMempool) EnableTxsAvailable()                    {}
func (emptyMempool) SizeBytes() int64                       { return 0 }

func (emptyMempool) TxsFront() *clist.CElement    { return nil }
func (emptyMempool) TxsWaitChan() <-chan struct{} { return nil }

func (emptyMempool) InitWAL() error { return nil }
func (emptyMempool) CloseWAL()      {}

//-----------------------------------------------------------------------------
// mockProxyApp uses Responses to FinalizeBlock to give the right results.
//
// Useful because we don't want to call Commit() twice for the same block on
// the real app.

func newMockProxyApp(
	logger log.Logger,
	appHash []byte,
	finalizeBlockResponses *abci.ResponseFinalizeBlock,
) (abciclient.Client, error) {
	return proxy.New(abciclient.NewLocalClient(logger, &mockProxyApp{
		appHash:                appHash,
		finalizeBlockResponses: finalizeBlockResponses,
	}), logger, proxy.NopMetrics()), nil
}

type mockProxyApp struct {
	abci.BaseApplication

	appHash                []byte
	txCount                int
	finalizeBlockResponses *abci.ResponseFinalizeBlock
}

func (mock *mockProxyApp) FinalizeBlock(_ context.Context, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	r := mock.finalizeBlockResponses
	mock.txCount++
	if r == nil {
		return &abci.ResponseFinalizeBlock{}, nil
	}
	return r, nil
}

func (mock *mockProxyApp) Commit(context.Context) (*abci.ResponseCommit, error) {
	return &abci.ResponseCommit{}, nil
}
