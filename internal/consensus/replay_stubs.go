package consensus

import (
	"context"

	abciclient "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/libs/clist"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/internal/proxy"
	"github.com/tendermint/tendermint/libs/log"
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------

type emptyMempool struct{}

var _ mempool.MempoolABCI = emptyMempool{}

func (emptyMempool) Lock()     {}
func (emptyMempool) Unlock()   {}
func (emptyMempool) size() int { return 0 }
func (emptyMempool) CheckTx(context.Context, types.Tx, func(*abci.ResponseCheckTx), mempool.TxInfo) error {
	return nil
}
func (m emptyMempool) PoolMeta() mempool.PoolMeta { return mempool.PoolMeta{} }
func (m emptyMempool) PrepBlockFinality(ctx context.Context) (finishFn func(), err error) {
	return func() {}, nil
}
func (m emptyMempool) Reap(ctx context.Context, opts ...mempool.ReapOptFn) (types.Txs, error) {
	return types.Txs{}, nil
}
func (m emptyMempool) Remove(ctx context.Context, opts ...mempool.RemOptFn) error { return nil }
func (emptyMempool) RemoveTxByKey(txKey types.TxKey) error                        { return nil }
func (emptyMempool) ReapMaxBytesMaxGas(_, _ int64) types.Txs                      { return types.Txs{} }
func (emptyMempool) ReapMaxTxs(n int) types.Txs                                   { return types.Txs{} }
func (emptyMempool) Update(
	_ context.Context,
	_ int64,
	_ types.Txs,
	_ []*abci.ExecTxResult,
	_ mempool.PreCheckFunc,
	_ mempool.PostCheckFunc,
) error {
	return nil
}
func (emptyMempool) Flush(ctx context.Context) error        { return nil }
func (emptyMempool) FlushAppConn(ctx context.Context) error { return nil }
func (emptyMempool) TxsAvailable() <-chan struct{}          { return make(chan struct{}) }
func (emptyMempool) EnableTxsAvailable()                    {}
func (emptyMempool) SizeBytes() int64                       { return 0 }

func (emptyMempool) TxsFront() *clist.CElement    { return nil }
func (emptyMempool) TxsWaitChan() <-chan struct{} { return nil }

func (emptyMempool) InitWAL() error { return nil }
func (emptyMempool) CloseWAL()      {}

//-----------------------------------------------------------------------------
// mockProxyApp uses ABCIResponses to give the right results.
//
// Useful because we don't want to call Commit() twice for the same block on
// the real app.

func newMockProxyApp(
	ctx context.Context,
	logger log.Logger,
	appHash []byte,
	abciResponses *tmstate.ABCIResponses,
) (abciclient.Client, error) {
	return proxy.New(abciclient.NewLocalClient(logger, &mockProxyApp{
		appHash:       appHash,
		abciResponses: abciResponses,
	}), logger, proxy.NopMetrics()), nil
}

type mockProxyApp struct {
	abci.BaseApplication

	appHash       []byte
	txCount       int
	abciResponses *tmstate.ABCIResponses
}

func (mock *mockProxyApp) FinalizeBlock(req abci.RequestFinalizeBlock) abci.ResponseFinalizeBlock {
	r := mock.abciResponses.FinalizeBlock
	mock.txCount++
	if r == nil {
		return abci.ResponseFinalizeBlock{}
	}
	return *r
}

func (mock *mockProxyApp) Commit() abci.ResponseCommit {
	return abci.ResponseCommit{Data: mock.appHash}
}
