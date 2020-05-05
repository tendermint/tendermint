package consensus

import (
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/clist"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------

type mempoolStub struct{}

var _ mempl.Mempool = mempoolStub{}

func (mempoolStub) Lock()     {}
func (mempoolStub) Unlock()   {}
func (mempoolStub) Size() int { return 0 }
func (mempoolStub) CheckTx(_ types.Tx, _ func(*abci.Response), _ mempl.TxInfo) error {
	return nil
}
func (mempoolStub) ReapMaxBytesMaxGas(_, _ int64) types.Txs { return types.Txs{} }
func (mempoolStub) ReapMaxTxs(n int) types.Txs              { return types.Txs{} }
func (mempoolStub) Update(
	_ int64,
	_ types.Txs,
	_ []*abci.ResponseDeliverTx,
	_ mempl.PreCheckFunc,
	_ mempl.PostCheckFunc,
) error {
	return nil
}
func (mempoolStub) Flush()                        {}
func (mempoolStub) FlushAppConn() error           { return nil }
func (mempoolStub) TxsAvailable() <-chan struct{} { return make(chan struct{}) }
func (mempoolStub) EnableTxsAvailable()           {}
func (mempoolStub) TxsBytes() int64               { return 0 }

func (mempoolStub) TxsFront() *clist.CElement    { return nil }
func (mempoolStub) TxsWaitChan() <-chan struct{} { return nil }

func (mempoolStub) InitWAL()  {}
func (mempoolStub) CloseWAL() {}

//-----------------------------------------------------------------------------

type evPoolStub struct{}

var _ sm.EvidencePool = evPoolStub{}

func (evPoolStub) PendingEvidence(int64) []types.Evidence { return nil }
func (evPoolStub) AddEvidence(types.Evidence) error       { return nil }
func (evPoolStub) Update(*types.Block, sm.State)          {}
func (evPoolStub) IsCommitted(types.Evidence) bool        { return false }
func (evPoolStub) IsPending(types.Evidence) bool          { return false }

//-----------------------------------------------------------------------------
// mockProxyApp uses ABCIResponses to give the right results.
//
// Useful because we don't want to call Commit() twice for the same block on
// the real app.

func newMockProxyApp(appHash []byte, abciResponses *sm.ABCIResponses) proxy.AppConnConsensus {
	clientCreator := proxy.NewLocalClientCreator(&mockProxyApp{
		appHash:       appHash,
		abciResponses: abciResponses,
	})
	cli, _ := clientCreator.NewABCIClient()
	err := cli.Start()
	if err != nil {
		panic(err)
	}
	return proxy.NewAppConnConsensus(cli)
}

type mockProxyApp struct {
	abci.BaseApplication

	appHash       []byte
	txCount       int
	abciResponses *sm.ABCIResponses
}

func (mock *mockProxyApp) DeliverTx(req abci.RequestDeliverTx) abci.ResponseDeliverTx {
	r := mock.abciResponses.DeliverTxs[mock.txCount]
	mock.txCount++
	if r == nil { //it could be nil because of amino unMarshall, it will cause an empty ResponseDeliverTx to become nil
		return abci.ResponseDeliverTx{}
	}
	return *r
}

func (mock *mockProxyApp) EndBlock(req abci.RequestEndBlock) abci.ResponseEndBlock {
	mock.txCount = 0
	return *mock.abciResponses.EndBlock
}

func (mock *mockProxyApp) Commit() abci.ResponseCommit {
	return abci.ResponseCommit{Data: mock.appHash}
}
