package mock

import (
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/clist"
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/types"
)

// Mempool is an empty implementation of a Mempool, useful for testing.
type Mempool struct{}

var _ mempool.Mempool = Mempool{}

func (Mempool) Lock()     {}
func (Mempool) Unlock()   {}
func (Mempool) Size() int { return 0 }
func (Mempool) CheckTx(_ types.Tx, _ func(*abci.Response), _ mempool.TxInfo) error {
	return nil
}
func (Mempool) RemoveTxByKey(txKey types.TxKey) error   { return nil }
func (Mempool) ReapMaxBytesMaxGas(_, _ int64) types.Txs { return types.Txs{} }
func (Mempool) ReapMaxTxs(n int) types.Txs              { return types.Txs{} }
func (Mempool) Update(
	_ int64,
	_ types.Txs,
	_ []*abci.ResponseDeliverTx,
	_ mempool.PreCheckFunc,
	_ mempool.PostCheckFunc,
) error {
	return nil
}
func (Mempool) Flush()                        {}
func (Mempool) FlushAppConn() error           { return nil }
func (Mempool) TxsAvailable() <-chan struct{} { return make(chan struct{}) }
func (Mempool) EnableTxsAvailable()           {}
func (Mempool) SizeBytes() int64              { return 0 }

func (Mempool) TxsFront() *clist.CElement    { return nil }
func (Mempool) TxsWaitChan() <-chan struct{} { return nil }

func (Mempool) InitWAL() error { return nil }
func (Mempool) CloseWAL()      {}
