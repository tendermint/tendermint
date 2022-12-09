package consensus

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// for testing
func assertMempool(txn txNotifier) mempl.Mempool {
	return txn.(mempl.Mempool)
}

func TestMempoolNoProgressUntilTxsAvailable(t *testing.T) {
	config := ResetConfig("consensus_mempool_txs_available_test")
	defer os.RemoveAll(config.RootDir)
	config.Consensus.CreateEmptyBlocks = false
	state, privVals := randGenesisState(1, false, 10, nil)
	app := kvstore.NewInMemoryApplication()
	resp, err := app.Info(context.Background(), proxy.RequestInfo)
	require.NoError(t, err)
	state.AppHash = resp.LastBlockAppHash
	cs := newStateWithConfig(config, state, privVals[0], app)
	assertMempool(cs.txNotifier).EnableTxsAvailable()
	height, round := cs.Height, cs.Round
	newBlockCh := subscribe(cs.eventBus, types.EventQueryNewBlock)
	startTestRound(cs, height, round)

	ensureNewEventOnChannel(newBlockCh) // first block gets committed
	ensureNoNewEventOnChannel(newBlockCh)
	deliverTxsRange(t, cs, 0, 1)
	ensureNewEventOnChannel(newBlockCh) // commit txs
	ensureNewEventOnChannel(newBlockCh) // commit updated app hash
	ensureNoNewEventOnChannel(newBlockCh)
}

func TestMempoolProgressAfterCreateEmptyBlocksInterval(t *testing.T) {
	config := ResetConfig("consensus_mempool_txs_available_test")
	defer os.RemoveAll(config.RootDir)

	config.Consensus.CreateEmptyBlocksInterval = ensureTimeout
	state, privVals := randGenesisState(1, false, 10, nil)
	app := kvstore.NewInMemoryApplication()
	resp, err := app.Info(context.Background(), proxy.RequestInfo)
	require.NoError(t, err)
	state.AppHash = resp.LastBlockAppHash
	cs := newStateWithConfig(config, state, privVals[0], app)

	assertMempool(cs.txNotifier).EnableTxsAvailable()

	newBlockCh := subscribe(cs.eventBus, types.EventQueryNewBlock)
	startTestRound(cs, cs.Height, cs.Round)

	ensureNewEventOnChannel(newBlockCh)   // first block gets committed
	ensureNoNewEventOnChannel(newBlockCh) // then we dont make a block ...
	ensureNewEventOnChannel(newBlockCh)   // until the CreateEmptyBlocksInterval has passed
}

func TestMempoolProgressInHigherRound(t *testing.T) {
	config := ResetConfig("consensus_mempool_txs_available_test")
	defer os.RemoveAll(config.RootDir)
	config.Consensus.CreateEmptyBlocks = false
	state, privVals := randGenesisState(1, false, 10, nil)
	cs := newStateWithConfig(config, state, privVals[0], kvstore.NewInMemoryApplication())
	assertMempool(cs.txNotifier).EnableTxsAvailable()
	height, round := cs.Height, cs.Round
	newBlockCh := subscribe(cs.eventBus, types.EventQueryNewBlock)
	newRoundCh := subscribe(cs.eventBus, types.EventQueryNewRound)
	timeoutCh := subscribe(cs.eventBus, types.EventQueryTimeoutPropose)
	cs.setProposal = func(proposal *types.Proposal) error {
		if cs.Height == 2 && cs.Round == 0 {
			// dont set the proposal in round 0 so we timeout and
			// go to next round
			cs.Logger.Info("Ignoring set proposal at height 2, round 0")
			return nil
		}
		return cs.defaultSetProposal(proposal)
	}
	startTestRound(cs, height, round)

	ensureNewRound(newRoundCh, height, round) // first round at first height
	ensureNewEventOnChannel(newBlockCh)       // first block gets committed

	height++ // moving to the next height
	round = 0

	ensureNewRound(newRoundCh, height, round) // first round at next height
	deliverTxsRange(t, cs, 0, 1)              // we deliver txs, but dont set a proposal so we get the next round
	ensureNewTimeout(timeoutCh, height, round, cs.config.TimeoutPropose.Nanoseconds())

	round++                                   // moving to the next round
	ensureNewRound(newRoundCh, height, round) // wait for the next round
	ensureNewEventOnChannel(newBlockCh)       // now we can commit the block
}

func deliverTxsRange(t *testing.T, cs *State, start, end int) {
	// Deliver some txs.
	for i := start; i < end; i++ {
		err := assertMempool(cs.txNotifier).CheckTx(kvstore.NewTx(fmt.Sprintf("%d", i), "true"), nil, mempl.TxInfo{})
		require.NoError(t, err)
	}
}

func TestMempoolTxConcurrentWithCommit(t *testing.T) {
	state, privVals := randGenesisState(1, false, 10, nil)
	blockDB := dbm.NewMemDB()
	stateStore := sm.NewStore(blockDB, sm.StoreOptions{DiscardABCIResponses: false})
	cs := newStateWithConfigAndBlockStore(config, state, privVals[0], kvstore.NewInMemoryApplication(), blockDB)
	err := stateStore.Save(state)
	require.NoError(t, err)
	newBlockEventsCh := subscribe(cs.eventBus, types.EventQueryNewBlockEvents)

	const numTxs int64 = 3000
	go deliverTxsRange(t, cs, 0, int(numTxs))

	startTestRound(cs, cs.Height, cs.Round)
	for n := int64(0); n < numTxs; {
		select {
		case msg := <-newBlockEventsCh:
			event := msg.Data().(types.EventDataNewBlockEvents)
			n += event.NumTxs
		case <-time.After(30 * time.Second):
			t.Fatal("Timed out waiting 30s to commit blocks with transactions")
		}
	}
}

func TestMempoolRmBadTx(t *testing.T) {
	state, privVals := randGenesisState(1, false, 10, nil)
	app := kvstore.NewInMemoryApplication()
	blockDB := dbm.NewMemDB()
	stateStore := sm.NewStore(blockDB, sm.StoreOptions{DiscardABCIResponses: false})
	cs := newStateWithConfigAndBlockStore(config, state, privVals[0], app, blockDB)
	err := stateStore.Save(state)
	require.NoError(t, err)

	// increment the counter by 1
	txBytes := kvstore.NewTx("key", "value")
	res, err := app.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{Txs: [][]byte{txBytes}})
	require.NoError(t, err)
	assert.False(t, res.TxResults[0].IsErr())
	assert.True(t, len(res.AgreedAppData) > 0)

	_, err = app.Commit(context.Background(), &abci.RequestCommit{})
	require.NoError(t, err)

	emptyMempoolCh := make(chan struct{})
	checkTxRespCh := make(chan struct{})
	go func() {
		// Try to send the tx through the mempool.
		// CheckTx should not err, but the app should return a bad abci code
		// and the tx should get removed from the pool
		invalidTx := []byte("invalidTx")
		err := assertMempool(cs.txNotifier).CheckTx(invalidTx, func(r *abci.ResponseCheckTx) {
			if r.Code != kvstore.CodeTypeInvalidTxFormat {
				t.Errorf("expected checktx to return invalid format, got %v", r)
				return
			}
			checkTxRespCh <- struct{}{}
		}, mempl.TxInfo{})
		if err != nil {
			t.Errorf("error after CheckTx: %v", err)
			return
		}

		// check for the tx
		for {
			txs := assertMempool(cs.txNotifier).ReapMaxBytesMaxGas(int64(len(invalidTx)), -1)
			if len(txs) == 0 {
				emptyMempoolCh <- struct{}{}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Wait until the tx returns
	ticker := time.After(time.Second * 5)
	select {
	case <-checkTxRespCh:
		// success
	case <-ticker:
		t.Errorf("timed out waiting for tx to return")
		return
	}

	// Wait until the tx is removed
	ticker = time.After(time.Second * 5)
	select {
	case <-emptyMempoolCh:
		// success
	case <-ticker:
		t.Errorf("timed out waiting for tx to be removed")
		return
	}
}
