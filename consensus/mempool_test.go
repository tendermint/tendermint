package consensus

import (
	"encoding/binary"
	"testing"
	"time"

	abci "github.com/tendermint/abci/types"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
)

func init() {
	config = ResetConfig("consensus_mempool_test")
}

func TestNoProgressUntilTxsAvailable(t *testing.T) {
	config := ResetConfig("consensus_mempool_txs_available_test")
	config.Consensus.CreateEmptyBlocks = false
	state, privVals := randGenesisState(1, false, 10)
	cs := newConsensusStateWithConfig(config, state, privVals[0], NewCounterApplication())
	cs.mempool.EnableTxsAvailable()
	height, round := cs.Height, cs.Round
	newBlockCh := subscribe(cs.eventBus, types.EventQueryNewBlock)
	startTestRound(cs, height, round)

	ensureNewStep(newBlockCh) // first block gets committed
	ensureNoNewStep(newBlockCh)
	deliverTxsRange(cs, 0, 1)
	ensureNewStep(newBlockCh) // commit txs
	ensureNewStep(newBlockCh) // commit updated app hash
	ensureNoNewStep(newBlockCh)
}

func TestProgressAfterCreateEmptyBlocksInterval(t *testing.T) {
	config := ResetConfig("consensus_mempool_txs_available_test")
	config.Consensus.CreateEmptyBlocksInterval = int(ensureTimeout.Seconds())
	state, privVals := randGenesisState(1, false, 10)
	cs := newConsensusStateWithConfig(config, state, privVals[0], NewCounterApplication())
	cs.mempool.EnableTxsAvailable()
	height, round := cs.Height, cs.Round
	newBlockCh := subscribe(cs.eventBus, types.EventQueryNewBlock)
	startTestRound(cs, height, round)

	ensureNewStep(newBlockCh)   // first block gets committed
	ensureNoNewStep(newBlockCh) // then we dont make a block ...
	ensureNewStep(newBlockCh)   // until the CreateEmptyBlocksInterval has passed
}

func TestProgressInHigherRound(t *testing.T) {
	config := ResetConfig("consensus_mempool_txs_available_test")
	config.Consensus.CreateEmptyBlocks = false
	state, privVals := randGenesisState(1, false, 10)
	cs := newConsensusStateWithConfig(config, state, privVals[0], NewCounterApplication())
	cs.mempool.EnableTxsAvailable()
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

	ensureNewStep(newRoundCh) // first round at first height
	ensureNewStep(newBlockCh) // first block gets committed
	ensureNewStep(newRoundCh) // first round at next height
	deliverTxsRange(cs, 0, 1) // we deliver txs, but dont set a proposal so we get the next round
	<-timeoutCh
	ensureNewStep(newRoundCh) // wait for the next round
	ensureNewStep(newBlockCh) // now we can commit the block
}

func deliverTxsRange(cs *ConsensusState, start, end int) {
	// Deliver some txs.
	for i := start; i < end; i++ {
		txBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(txBytes, uint64(i))
		err := cs.mempool.CheckTx(txBytes, nil)
		if err != nil {
			panic(cmn.Fmt("Error after CheckTx: %v", err))
		}
	}
}

func TestTxConcurrentWithCommit(t *testing.T) {
	state, privVals := randGenesisState(1, false, 10)
	cs := newConsensusState(state, privVals[0], NewCounterApplication())
	height, round := cs.Height, cs.Round
	newBlockCh := subscribe(cs.eventBus, types.EventQueryNewBlock)

	NTxs := 10000
	go deliverTxsRange(cs, 0, NTxs)

	startTestRound(cs, height, round)
	ticker := time.NewTicker(time.Second * 20)
	for nTxs := 0; nTxs < NTxs; {
		select {
		case b := <-newBlockCh:
			nTxs += b.(types.TMEventData).Unwrap().(types.EventDataNewBlock).Block.Header.NumTxs
		case <-ticker.C:
			panic("Timed out waiting to commit blocks with transactions")
		}
	}
}

func TestRmBadTx(t *testing.T) {
	state, privVals := randGenesisState(1, false, 10)
	app := NewCounterApplication()
	cs := newConsensusState(state, privVals[0], app)

	// increment the counter by 1
	txBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(txBytes, uint64(0))
	app.DeliverTx(txBytes)
	app.Commit()

	emptyMempoolCh := make(chan struct{})
	checkTxRespCh := make(chan struct{})
	go func() {
		// Try to send the tx through the mempool.
		// CheckTx should not err, but the app should return a bad abci code
		// and the tx should get removed from the pool
		err := cs.mempool.CheckTx(txBytes, func(r *abci.Response) {
			if r.GetCheckTx().Code != abci.CodeType_BadNonce {
				t.Fatalf("expected checktx to return bad nonce, got %v", r)
			}
			checkTxRespCh <- struct{}{}
		})
		if err != nil {
			t.Fatal("Error after CheckTx: %v", err)
		}

		// check for the tx
		for {
			txs := cs.mempool.Reap(1)
			if len(txs) == 0 {
				emptyMempoolCh <- struct{}{}
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
	return abci.ResponseInfo{Data: cmn.Fmt("txs:%v", app.txCount)}
}

func (app *CounterApplication) DeliverTx(tx []byte) abci.Result {
	return runTx(tx, &app.txCount)
}

func (app *CounterApplication) CheckTx(tx []byte) abci.Result {
	return runTx(tx, &app.mempoolTxCount)
}

func runTx(tx []byte, countPtr *int) abci.Result {
	count := *countPtr
	tx8 := make([]byte, 8)
	copy(tx8[len(tx8)-len(tx):], tx)
	txValue := binary.BigEndian.Uint64(tx8)
	if txValue != uint64(count) {
		return abci.ErrBadNonce.AppendLog(cmn.Fmt("Invalid nonce. Expected %v, got %v", count, txValue))
	}
	*countPtr += 1
	return abci.OK
}

func (app *CounterApplication) Commit() abci.Result {
	app.mempoolTxCount = app.txCount
	if app.txCount == 0 {
		return abci.OK
	} else {
		hash := make([]byte, 8)
		binary.BigEndian.PutUint64(hash, uint64(app.txCount))
		return abci.NewResultOK(hash, "")
	}
}
