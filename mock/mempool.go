package mock

import (
	"crypto/sha256"
	"fmt"

	abci "github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/clist"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/trace"
	"github.com/tendermint/tendermint/types"
)

// Mempool is an empty implementation of a Mempool, useful for testing.
type Mempool struct{}

func (m Mempool) GetAddressList() []string {
	return nil
}

func (m Mempool) GetTxByHash(hash [sha256.Size]byte) (types.Tx, error) {
	return nil, mempl.ErrNoSuchTx
}

var _ mempl.Mempool = Mempool{}

func (Mempool) Lock()     {}
func (Mempool) Unlock()   {}
func (Mempool) Size() int { return 0 }
func (Mempool) CheckTx(_ types.Tx, _ func(*abci.Response), _ mempl.TxInfo) error {
	return nil
}
func (Mempool) ReapMaxBytesMaxGas(_, _ int64) types.Txs       { return types.Txs{} }
func (Mempool) ReapMaxTxs(n int) types.Txs                    { return types.Txs{} }
func (Mempool) ReapUserTxsCnt(address string) int             { return 0 }
func (Mempool) GetUserPendingTxsCnt(address string) int       { return 0 }
func (Mempool) ReapUserTxs(address string, max int) types.Txs { return types.Txs{} }
func (Mempool) Update(
	_ int64,
	txs types.Txs,
	deliverTxResponses []*abci.ResponseDeliverTx,
	_ mempl.PreCheckFunc,
	_ mempl.PostCheckFunc,
) error {
	var gasUsed uint64
	for i := range txs {
		gasUsed += uint64(deliverTxResponses[i].GasUsed)
	}
	trace.GetElapsedInfo().AddInfo(trace.GasUsed, fmt.Sprintf("%d", gasUsed))
	return nil
}
func (Mempool) Flush()                        {}
func (Mempool) FlushAppConn() error           { return nil }
func (Mempool) TxsAvailable() <-chan struct{} { return make(chan struct{}) }
func (Mempool) EnableTxsAvailable()           {}
func (Mempool) TxsBytes() int64               { return 0 }

func (Mempool) TxsFront() *clist.CElement    { return nil }
func (Mempool) TxsWaitChan() <-chan struct{} { return nil }

func (Mempool) InitWAL() error                              { return nil }
func (Mempool) CloseWAL()                                   {}
func (Mempool) SetEventBus(eventBus types.TxEventPublisher) {}

func (Mempool) GetConfig() *cfg.MempoolConfig {
	return cfg.DefaultMempoolConfig()
}

func (Mempool) SetAccountRetriever(_ mempl.AccountRetriever) {
}

func (Mempool) SetTxInfoParser(_ mempl.TxInfoParser) {

}
