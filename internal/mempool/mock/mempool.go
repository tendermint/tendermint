package mock

import (
	"context"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/libs/clist"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/types"
)

// Mempool is an empty implementation of a Mempool, useful for testing.
type Mempool struct{}

var _ mempool.MempoolABCI = Mempool{}

func (Mempool) Size() int { return 0 }
func (Mempool) CheckTx(context.Context, types.Tx, func(*abci.ResponseCheckTx), mempool.TxInfo) error {
	return nil
}
func (m Mempool) Remove(ctx context.Context, opts ...mempool.RemOptFn) error { return nil }
func (Mempool) RemoveTxByKey(txKey types.TxKey) error                        { return nil }
func (m Mempool) PoolMeta() mempool.PoolMeta                                 { return mempool.PoolMeta{} }
func (Mempool) PrepBlockFinality(ctx context.Context) (finishFn func(), err error) {
	return func() {}, nil
}
func (m Mempool) Reap(ctx context.Context, opts ...mempool.ReapOptFn) (types.Txs, error) {
	return types.Txs{}, nil
}
func (Mempool) Update(
	_ context.Context,
	_ int64,
	_ types.Txs,
	_ []*abci.ExecTxResult,
	_ mempool.PreCheckFunc,
	_ mempool.PostCheckFunc,
) error {
	return nil
}
func (Mempool) Flush(ctx context.Context) error        { return nil }
func (Mempool) FlushAppConn(ctx context.Context) error { return nil }
func (Mempool) TxsAvailable() <-chan struct{}          { return make(chan struct{}) }
func (Mempool) EnableTxsAvailable()                    {}
func (Mempool) SizeBytes() int64                       { return 0 }

func (Mempool) TxsFront() *clist.CElement    { return nil }
func (Mempool) TxsWaitChan() <-chan struct{} { return nil }

func (Mempool) InitWAL() error { return nil }
func (Mempool) CloseWAL()      {}
