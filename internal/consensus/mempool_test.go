package consensus

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/code"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/mempool"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

// for testing
func assertMempool(t *testing.T, txn txNotifier) mempool.Mempool {
	t.Helper()
	mp, ok := txn.(mempool.Mempool)
	require.True(t, ok)
	return mp
}

func TestMempoolNoProgressUntilTxsAvailable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	baseConfig := configSetup(t)

	config, err := ResetConfig(t.TempDir(), "consensus_mempool_txs_available_test")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(config.RootDir) })

	config.Consensus.CreateEmptyBlocks = false
	state, privVals := makeGenesisState(ctx, t, baseConfig, genesisStateArgs{
		Validators: 1,
		Power:      10,
		Params:     factory.ConsensusParams()})
	cs := newStateWithConfig(ctx, t, log.NewNopLogger(), config, state, privVals[0], NewCounterApplication())
	assertMempool(t, cs.txNotifier).EnableTxsAvailable()
	height, round := cs.Height, cs.Round
	newBlockCh := subscribe(ctx, t, cs.eventBus, types.EventQueryNewBlock)
	startTestRound(ctx, cs, height, round)

	ensureNewEventOnChannel(t, newBlockCh) // first block gets committed
	ensureNoNewEventOnChannel(t, newBlockCh)
	checkTxsRange(ctx, t, cs, 0, 1)
	ensureNewEventOnChannel(t, newBlockCh) // commit txs
	ensureNewEventOnChannel(t, newBlockCh) // commit updated app hash
	ensureNoNewEventOnChannel(t, newBlockCh)
}

func TestMempoolProgressAfterCreateEmptyBlocksInterval(t *testing.T) {
	baseConfig := configSetup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config, err := ResetConfig(t.TempDir(), "consensus_mempool_txs_available_test")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(config.RootDir) })

	config.Consensus.CreateEmptyBlocksInterval = ensureTimeout
	state, privVals := makeGenesisState(ctx, t, baseConfig, genesisStateArgs{
		Validators: 1,
		Power:      10,
		Params:     factory.ConsensusParams()})
	cs := newStateWithConfig(ctx, t, log.NewNopLogger(), config, state, privVals[0], NewCounterApplication())

	assertMempool(t, cs.txNotifier).EnableTxsAvailable()

	newBlockCh := subscribe(ctx, t, cs.eventBus, types.EventQueryNewBlock)
	startTestRound(ctx, cs, cs.Height, cs.Round)

	ensureNewEventOnChannel(t, newBlockCh)   // first block gets committed
	ensureNoNewEventOnChannel(t, newBlockCh) // then we dont make a block ...
	ensureNewEventOnChannel(t, newBlockCh)   // until the CreateEmptyBlocksInterval has passed
}

func TestMempoolProgressInHigherRound(t *testing.T) {
	baseConfig := configSetup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config, err := ResetConfig(t.TempDir(), "consensus_mempool_txs_available_test")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(config.RootDir) })

	config.Consensus.CreateEmptyBlocks = false
	state, privVals := makeGenesisState(ctx, t, baseConfig, genesisStateArgs{
		Validators: 1,
		Power:      10,
		Params:     factory.ConsensusParams()})
	cs := newStateWithConfig(ctx, t, log.NewNopLogger(), config, state, privVals[0], NewCounterApplication())
	assertMempool(t, cs.txNotifier).EnableTxsAvailable()
	height, round := cs.Height, cs.Round
	newBlockCh := subscribe(ctx, t, cs.eventBus, types.EventQueryNewBlock)
	newRoundCh := subscribe(ctx, t, cs.eventBus, types.EventQueryNewRound)
	timeoutCh := subscribe(ctx, t, cs.eventBus, types.EventQueryTimeoutPropose)
	cs.setProposal = func(proposal *types.Proposal, recvTime time.Time) error {
		if cs.Height == 2 && cs.Round == 0 {
			// dont set the proposal in round 0 so we timeout and
			// go to next round
			return nil
		}
		return cs.defaultSetProposal(proposal, recvTime)
	}
	startTestRound(ctx, cs, height, round)

	ensureNewRound(t, newRoundCh, height, round) // first round at first height
	ensureNewEventOnChannel(t, newBlockCh)       // first block gets committed

	height++ // moving to the next height
	round = 0

	ensureNewRound(t, newRoundCh, height, round) // first round at next height
	checkTxsRange(ctx, t, cs, 0, 1)              // we deliver txs, but don't set a proposal so we get the next round
	ensureNewTimeout(t, timeoutCh, height, round, cs.state.ConsensusParams.Timeout.ProposeTimeout(round).Nanoseconds())

	round++                                      // moving to the next round
	ensureNewRound(t, newRoundCh, height, round) // wait for the next round
	ensureNewEventOnChannel(t, newBlockCh)       // now we can commit the block
}

func checkTxsRange(ctx context.Context, t *testing.T, cs *State, start, end int) {
	t.Helper()
	// Deliver some txs.
	for i := start; i < end; i++ {
		txBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(txBytes, uint64(i))
		var rCode uint32
		err := assertMempool(t, cs.txNotifier).CheckTx(ctx, txBytes, func(r *abci.ResponseCheckTx) { rCode = r.Code }, mempool.TxInfo{})
		require.NoError(t, err, "error after checkTx")
		require.Equal(t, code.CodeTypeOK, rCode, "checkTx code is error, txBytes %X", txBytes)
	}
}

func TestMempoolTxConcurrentWithCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := configSetup(t)
	logger := log.NewNopLogger()
	state, privVals := makeGenesisState(ctx, t, config, genesisStateArgs{
		Validators: 1,
		Power:      10,
		Params:     factory.ConsensusParams(),
	})
	stateStore := sm.NewStore(dbm.NewMemDB())
	blockStore := store.NewBlockStore(dbm.NewMemDB())

	cs := newStateWithConfigAndBlockStore(
		ctx,
		t,
		logger, config, state, privVals[0], NewCounterApplication(), blockStore)

	err := stateStore.Save(state)
	require.NoError(t, err)
	newBlockHeaderCh := subscribe(ctx, t, cs.eventBus, types.EventQueryNewBlockHeader)

	const numTxs int64 = 3000
	go checkTxsRange(ctx, t, cs, 0, int(numTxs))

	startTestRound(ctx, cs, cs.Height, cs.Round)
	for n := int64(0); n < numTxs; {
		select {
		case msg := <-newBlockHeaderCh:
			headerEvent := msg.Data().(types.EventDataNewBlockHeader)
			n += headerEvent.NumTxs
			logger.Info("new transactions", "nTxs", headerEvent.NumTxs, "total", n)
		case <-time.After(30 * time.Second):
			t.Fatal("Timed out waiting 30s to commit blocks with transactions")
		}
	}
}

func TestMempoolRmBadTx(t *testing.T) {
	config := configSetup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	state, privVals := makeGenesisState(ctx, t, config, genesisStateArgs{
		Validators: 1,
		Power:      10,
		Params:     factory.ConsensusParams()})
	app := NewCounterApplication()
	stateStore := sm.NewStore(dbm.NewMemDB())
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	cs := newStateWithConfigAndBlockStore(ctx, t, log.NewNopLogger(), config, state, privVals[0], app, blockStore)
	err := stateStore.Save(state)
	require.NoError(t, err)

	// increment the counter by 1
	txBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(txBytes, uint64(0))

	resFinalize, err := app.FinalizeBlock(ctx, &abci.RequestFinalizeBlock{Txs: [][]byte{txBytes}})
	require.NoError(t, err)
	assert.False(t, resFinalize.TxResults[0].IsErr(), fmt.Sprintf("expected no error. got %v", resFinalize))
	assert.True(t, len(resFinalize.AppHash) > 0)

	_, err = app.Commit(ctx)
	require.NoError(t, err)

	emptyMempoolCh := make(chan struct{})
	checkTxRespCh := make(chan struct{})
	go func() {
		// Try to send an out-of-sequence transaction through the mempool.
		// CheckTx should not err, but the app should return a bad abci code
		// and the tx should get removed from the pool
		binary.BigEndian.PutUint64(txBytes, uint64(5))
		err := assertMempool(t, cs.txNotifier).CheckTx(ctx, txBytes, func(r *abci.ResponseCheckTx) {
			if r.Code != code.CodeTypeBadNonce {
				t.Errorf("expected checktx to return bad nonce, got %#v", r)
				return
			}
			checkTxRespCh <- struct{}{}
		}, mempool.TxInfo{})
		if err != nil {
			t.Errorf("error after CheckTx: %v", err)
			return
		}

		// check for the tx
		for {
			txs := assertMempool(t, cs.txNotifier).ReapMaxBytesMaxGas(int64(len(txBytes)), -1)
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

// CounterApplication that maintains a mempool state and resets it upon commit
type CounterApplication struct {
	abci.BaseApplication

	txCount        int
	mempoolTxCount int
	mu             sync.Mutex
}

func NewCounterApplication() *CounterApplication {
	return &CounterApplication{}
}

func (app *CounterApplication) Info(_ context.Context, req *abci.RequestInfo) (*abci.ResponseInfo, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	return &abci.ResponseInfo{Data: fmt.Sprintf("txs:%v", app.txCount)}, nil
}

func (app *CounterApplication) FinalizeBlock(_ context.Context, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	respTxs := make([]*abci.ExecTxResult, len(req.Txs))
	for i, tx := range req.Txs {
		txValue := txAsUint64(tx)
		if txValue != uint64(app.txCount) {
			respTxs[i] = &abci.ExecTxResult{
				Code: code.CodeTypeBadNonce,
				Log:  fmt.Sprintf("Invalid nonce. Expected %d, got %d", app.txCount, txValue),
			}
			continue
		}
		app.txCount++
		respTxs[i] = &abci.ExecTxResult{Code: code.CodeTypeOK}
	}

	res := &abci.ResponseFinalizeBlock{TxResults: respTxs}

	if app.txCount > 0 {
		res.AppHash = make([]byte, 8)
		binary.BigEndian.PutUint64(res.AppHash, uint64(app.txCount))
	}

	return res, nil
}

func (app *CounterApplication) CheckTx(_ context.Context, req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
	app.mu.Lock()
	defer app.mu.Unlock()
	if req.Type == abci.CheckTxType_New {
		txValue := txAsUint64(req.Tx)
		if txValue != uint64(app.mempoolTxCount) {
			return &abci.ResponseCheckTx{
				Code: code.CodeTypeBadNonce,
			}, nil
		}
		app.mempoolTxCount++
	}
	return &abci.ResponseCheckTx{Code: code.CodeTypeOK}, nil
}

func txAsUint64(tx []byte) uint64 {
	tx8 := make([]byte, 8)
	copy(tx8[len(tx8)-len(tx):], tx)
	return binary.BigEndian.Uint64(tx8)
}

func (app *CounterApplication) Commit(context.Context) (*abci.ResponseCommit, error) {
	app.mu.Lock()
	defer app.mu.Unlock()
	return &abci.ResponseCommit{}, nil
}

func (app *CounterApplication) PrepareProposal(_ context.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
	trs := make([]*abci.TxRecord, 0, len(req.Txs))
	var totalBytes int64
	for _, tx := range req.Txs {
		totalBytes += int64(len(tx))
		if totalBytes > req.MaxTxBytes {
			break
		}
		trs = append(trs, &abci.TxRecord{
			Action: abci.TxRecord_UNMODIFIED,
			Tx:     tx,
		})
	}
	return &abci.ResponsePrepareProposal{TxRecords: trs}, nil
}

func (app *CounterApplication) ProcessProposal(_ context.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
	return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
}
