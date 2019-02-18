package consensus

import (
	"encoding/binary"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/abci/example/code"
	abci "github.com/tendermint/tendermint/abci/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// for testing
func assertMempool(txn txNotifier) sm.Mempool {
	return txn.(sm.Mempool)
}

func TestMempoolNoProgressUntilTxsAvailable(t *testing.T) {
	config := ResetConfig("consensus_mempool_txs_available_test")
	defer os.RemoveAll(config.RootDir)
	config.Consensus.CreateEmptyBlocks = false
	state, privVals := randGenesisState(1, false, 10)
	cs := newConsensusStateWithConfig(config, state, privVals[0], NewCounterApplication())
	assertMempool(cs.txNotifier).EnableTxsAvailable()
	height, round := cs.Height, cs.Round
	newBlockCh := subscribe(cs.eventBus, types.EventQueryNewBlock)
	startTestRound(cs, height, round)

	ensureNewEventOnChannel(newBlockCh) // first block gets committed
	ensureNoNewEventOnChannel(newBlockCh)
	deliverTxsRange(cs, 0, 1)
	ensureNewEventOnChannel(newBlockCh) // commit txs
	ensureNewEventOnChannel(newBlockCh) // commit updated app hash
	ensureNoNewEventOnChannel(newBlockCh)
}

func TestMempoolProgressAfterCreateEmptyBlocksInterval(t *testing.T) {
	config := ResetConfig("consensus_mempool_txs_available_test")
	defer os.RemoveAll(config.RootDir)
	config.Consensus.CreateEmptyBlocksInterval = ensureTimeout
	state, privVals := randGenesisState(1, false, 10)
	cs := newConsensusStateWithConfig(config, state, privVals[0], NewCounterApplication())
	assertMempool(cs.txNotifier).EnableTxsAvailable()
	height, round := cs.Height, cs.Round
	newBlockCh := subscribe(cs.eventBus, types.EventQueryNewBlock)
	startTestRound(cs, height, round)

	ensureNewEventOnChannel(newBlockCh)   // first block gets committed
	ensureNoNewEventOnChannel(newBlockCh) // then we dont make a block ...
	ensureNewEventOnChannel(newBlockCh)   // until the CreateEmptyBlocksInterval has passed
}

func TestMempoolProgressInHigherRound(t *testing.T) {
	config := ResetConfig("consensus_mempool_txs_available_test")
	defer os.RemoveAll(config.RootDir)
	config.Consensus.CreateEmptyBlocks = false
	state, privVals := randGenesisState(1, false, 10)
	cs := newConsensusStateWithConfig(config, state, privVals[0], NewCounterApplication())
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

	height = height + 1 // moving to the next height
	round = 0

	ensureNewRound(newRoundCh, height, round) // first round at next height
	deliverTxsRange(cs, 0, 1)                 // we deliver txs, but dont set a proposal so we get the next round
	ensureNewTimeout(timeoutCh, height, round, cs.config.TimeoutPropose.Nanoseconds())

	round = round + 1                         // moving to the next round
	ensureNewRound(newRoundCh, height, round) // wait for the next round
	ensureNewEventOnChannel(newBlockCh)       // now we can commit the block
}

func deliverTxsRange(cs *ConsensusState, start, end int) {
	// Deliver some txs.
	for i := start; i < end; i++ {
		txBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(txBytes, uint64(i))
		err := assertMempool(cs.txNotifier).CheckTx(txBytes, nil)
		if err != nil {
			panic(fmt.Sprintf("Error after CheckTx: %v", err))
		}
	}
}

func TestMempoolTxConcurrentWithCommit(t *testing.T) {
	state, privVals := randGenesisState(1, false, 10)
	cs := newConsensusState(state, privVals[0], NewCounterApplication())
	height, round := cs.Height, cs.Round
	newBlockCh := subscribe(cs.eventBus, types.EventQueryNewBlock)

	NTxs := 3000
	go deliverTxsRange(cs, 0, NTxs)

	startTestRound(cs, height, round)
	for nTxs := 0; nTxs < NTxs; {
		ticker := time.NewTicker(time.Second * 30)
		select {
		case b := <-newBlockCh:
			evt := b.(types.EventDataNewBlock)
			nTxs += int(evt.Block.Header.NumTxs)
		case <-ticker.C:
			panic("Timed out waiting to commit blocks with transactions")
		}
	}
}

func TestMempoolRmBadTx(t *testing.T) {
	state, privVals := randGenesisState(1, false, 10)
	app := NewCounterApplication()
	cs := newConsensusState(state, privVals[0], app)

	// increment the counter by 1
	txBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(txBytes, uint64(0))

	resDeliver := app.DeliverTx(txBytes)
	assert.False(t, resDeliver.IsErr(), fmt.Sprintf("expected no error. got %v", resDeliver))

	resCommit := app.Commit()
	assert.True(t, len(resCommit.Data) > 0)

	emptyMempoolCh := make(chan struct{})
	checkTxRespCh := make(chan struct{})
	go func() {
		// Try to send the tx through the mempool.
		// CheckTx should not err, but the app should return a bad abci code
		// and the tx should get removed from the pool
		err := assertMempool(cs.txNotifier).CheckTx(txBytes, func(r *abci.Response) {
			if r.GetCheckTx().Code != code.CodeTypeBadNonce {
				t.Fatalf("expected checktx to return bad nonce, got %v", r)
			}
			checkTxRespCh <- struct{}{}
		})
		if err != nil {
			t.Fatalf("Error after CheckTx: %v", err)
		}

		// check for the tx
		for {
			txs := assertMempool(cs.txNotifier).ReapMaxBytesMaxGas(int64(len(txBytes)), -1)
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
		t.Fatalf("Timed out waiting for tx to return")
	}

	// Wait until the tx is removed
	ticker = time.After(time.Second * 5)
	select {
	case <-emptyMempoolCh:
		// success
	case <-ticker:
		t.Fatalf("Timed out waiting for tx to be removed")
	}
}

// CounterApplication that maintains a mempool state and resets it upon commit
type CounterApplication struct {
	abci.BaseApplication

	txCount        int
	mempoolTxCount int
}

func NewCounterApplication() *CounterApplication {
	return &CounterApplication{}
}

func (app *CounterApplication) Info(req abci.RequestInfo) abci.ResponseInfo {
	return abci.ResponseInfo{Data: fmt.Sprintf("txs:%v", app.txCount)}
}

func (app *CounterApplication) DeliverTx(tx []byte) abci.ResponseDeliverTx {
	txValue := txAsUint64(tx)
	if txValue != uint64(app.txCount) {
		return abci.ResponseDeliverTx{
			Code: code.CodeTypeBadNonce,
			Log:  fmt.Sprintf("Invalid nonce. Expected %v, got %v", app.txCount, txValue)}
	}
	app.txCount++
	return abci.ResponseDeliverTx{Code: code.CodeTypeOK}
}

func (app *CounterApplication) CheckTx(tx []byte) abci.ResponseCheckTx {
	txValue := txAsUint64(tx)
	if txValue != uint64(app.mempoolTxCount) {
		return abci.ResponseCheckTx{
			Code: code.CodeTypeBadNonce,
			Log:  fmt.Sprintf("Invalid nonce. Expected %v, got %v", app.mempoolTxCount, txValue)}
	}
	app.mempoolTxCount++
	return abci.ResponseCheckTx{Code: code.CodeTypeOK}
}

func txAsUint64(tx []byte) uint64 {
	tx8 := make([]byte, 8)
	copy(tx8[len(tx8)-len(tx):], tx)
	return binary.BigEndian.Uint64(tx8)
}

func (app *CounterApplication) Commit() abci.ResponseCommit {
	app.mempoolTxCount = app.txCount
	if app.txCount == 0 {
		return abci.ResponseCommit{}
	}
	hash := make([]byte, 8)
	binary.BigEndian.PutUint64(hash, uint64(app.txCount))
	return abci.ResponseCommit{Data: hash}
}
