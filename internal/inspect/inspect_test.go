package inspect_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/inspect"
	"github.com/tendermint/tendermint/internal/pubsub/query"
	"github.com/tendermint/tendermint/internal/state/indexer"
	indexermocks "github.com/tendermint/tendermint/internal/state/indexer/mocks"
	statemocks "github.com/tendermint/tendermint/internal/state/mocks"
	"github.com/tendermint/tendermint/libs/log"
	httpclient "github.com/tendermint/tendermint/rpc/client/http"
	"github.com/tendermint/tendermint/types"
)

func TestInspectConstructor(t *testing.T) {
	cfg, err := config.ResetTestRoot(t.TempDir(), "test")
	require.NoError(t, err)
	testLogger := log.NewNopLogger()
	t.Cleanup(leaktest.Check(t))
	defer func() { _ = os.RemoveAll(cfg.RootDir) }()
	t.Run("from config", func(t *testing.T) {
		logger := testLogger.With(t.Name())
		d, err := inspect.NewFromConfig(logger, cfg)
		require.NoError(t, err)
		require.NotNil(t, d)
	})

}

func TestInspectRun(t *testing.T) {
	cfg, err := config.ResetTestRoot(t.TempDir(), "test")
	require.NoError(t, err)

	testLogger := log.NewNopLogger()
	t.Cleanup(leaktest.Check(t))
	defer func() { _ = os.RemoveAll(cfg.RootDir) }()
	t.Run("from config", func(t *testing.T) {
		logger := testLogger.With(t.Name())
		d, err := inspect.NewFromConfig(logger, cfg)
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		stoppedWG := &sync.WaitGroup{}
		stoppedWG.Add(1)
		go func() {
			defer stoppedWG.Done()
			require.NoError(t, d.Run(ctx))
		}()
		time.Sleep(100 * time.Millisecond)
		cancel()
		stoppedWG.Wait()
	})

}

func TestBlock(t *testing.T) {
	testHeight := int64(1)
	testBlock := new(types.Block)
	testBlock.Header.Height = testHeight
	testBlock.Header.LastCommitHash = []byte("test hash")
	stateStoreMock := &statemocks.Store{}

	blockStoreMock := &statemocks.BlockStore{}
	blockStoreMock.On("Height").Return(testHeight)
	blockStoreMock.On("Base").Return(int64(0))
	blockStoreMock.On("LoadBlockMeta", testHeight).Return(&types.BlockMeta{})
	blockStoreMock.On("LoadBlock", testHeight).Return(testBlock)
	eventSinkMock := &indexermocks.EventSink{}
	eventSinkMock.On("Stop").Return(nil)
	eventSinkMock.On("Type").Return(indexer.EventSinkType("Mock"))

	rpcConfig := config.TestRPCConfig()
	l := log.NewNopLogger()
	d := inspect.New(rpcConfig, blockStoreMock, stateStoreMock, []indexer.EventSink{eventSinkMock}, l)
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		require.NoError(t, d.Run(ctx))
	}()
	// FIXME: used to induce context switch.
	// Determine more deterministic method for prompting a context switch
	runtime.Gosched()
	requireConnect(t, rpcConfig.ListenAddress, 20)
	cli, err := httpclient.New(rpcConfig.ListenAddress)
	require.NoError(t, err)
	resultBlock, err := cli.Block(ctx, &testHeight)
	require.NoError(t, err)
	require.Equal(t, testBlock.Height, resultBlock.Block.Height)
	require.Equal(t, testBlock.LastCommitHash, resultBlock.Block.LastCommitHash)
	cancel()
	wg.Wait()

	blockStoreMock.AssertExpectations(t)
	stateStoreMock.AssertExpectations(t)
}

func TestTxSearch(t *testing.T) {
	testHash := []byte("test")
	testTx := []byte("tx")
	testQuery := fmt.Sprintf("tx.hash = '%s'", string(testHash))
	testTxResult := &abcitypes.TxResult{
		Height: 1,
		Index:  100,
		Tx:     testTx,
	}

	stateStoreMock := &statemocks.Store{}
	blockStoreMock := &statemocks.BlockStore{}
	eventSinkMock := &indexermocks.EventSink{}
	eventSinkMock.On("Stop").Return(nil)
	eventSinkMock.On("Type").Return(indexer.KV)
	eventSinkMock.On("SearchTxEvents", mock.Anything,
		mock.MatchedBy(func(q *query.Query) bool { return testQuery == q.String() })).
		Return([]*abcitypes.TxResult{testTxResult}, nil)

	rpcConfig := config.TestRPCConfig()
	l := log.NewNopLogger()
	d := inspect.New(rpcConfig, blockStoreMock, stateStoreMock, []indexer.EventSink{eventSinkMock}, l)
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	startedWG := &sync.WaitGroup{}
	startedWG.Add(1)
	go func() {
		startedWG.Done()
		defer wg.Done()
		require.NoError(t, d.Run(ctx))
	}()
	// FIXME: used to induce context switch.
	// Determine more deterministic method for prompting a context switch
	startedWG.Wait()
	requireConnect(t, rpcConfig.ListenAddress, 20)
	cli, err := httpclient.New(rpcConfig.ListenAddress)
	require.NoError(t, err)

	var page = 1
	resultTxSearch, err := cli.TxSearch(ctx, testQuery, false, &page, &page, "")
	require.NoError(t, err)
	require.Len(t, resultTxSearch.Txs, 1)
	require.Equal(t, types.Tx(testTx), resultTxSearch.Txs[0].Tx)

	cancel()
	wg.Wait()

	eventSinkMock.AssertExpectations(t)
	stateStoreMock.AssertExpectations(t)
	blockStoreMock.AssertExpectations(t)
}
func TestTx(t *testing.T) {
	testHash := []byte("test")
	testTx := []byte("tx")

	stateStoreMock := &statemocks.Store{}
	blockStoreMock := &statemocks.BlockStore{}
	eventSinkMock := &indexermocks.EventSink{}
	eventSinkMock.On("Stop").Return(nil)
	eventSinkMock.On("Type").Return(indexer.KV)
	eventSinkMock.On("GetTxByHash", testHash).Return(&abcitypes.TxResult{
		Tx: testTx,
	}, nil)

	rpcConfig := config.TestRPCConfig()
	l := log.NewNopLogger()
	d := inspect.New(rpcConfig, blockStoreMock, stateStoreMock, []indexer.EventSink{eventSinkMock}, l)
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	startedWG := &sync.WaitGroup{}
	startedWG.Add(1)
	go func() {
		startedWG.Done()
		defer wg.Done()
		require.NoError(t, d.Run(ctx))
	}()
	// FIXME: used to induce context switch.
	// Determine more deterministic method for prompting a context switch
	startedWG.Wait()
	requireConnect(t, rpcConfig.ListenAddress, 20)
	cli, err := httpclient.New(rpcConfig.ListenAddress)
	require.NoError(t, err)

	res, err := cli.Tx(ctx, testHash, false)
	require.NoError(t, err)
	require.Equal(t, types.Tx(testTx), res.Tx)

	cancel()
	wg.Wait()

	eventSinkMock.AssertExpectations(t)
	stateStoreMock.AssertExpectations(t)
	blockStoreMock.AssertExpectations(t)
}
func TestConsensusParams(t *testing.T) {
	testHeight := int64(1)
	testMaxGas := int64(55)
	stateStoreMock := &statemocks.Store{}
	blockStoreMock := &statemocks.BlockStore{}
	blockStoreMock.On("Height").Return(testHeight)
	blockStoreMock.On("Base").Return(int64(0))
	stateStoreMock.On("LoadConsensusParams", testHeight).Return(types.ConsensusParams{
		Block: types.BlockParams{
			MaxGas: testMaxGas,
		},
	}, nil)
	eventSinkMock := &indexermocks.EventSink{}
	eventSinkMock.On("Stop").Return(nil)
	eventSinkMock.On("Type").Return(indexer.EventSinkType("Mock"))

	rpcConfig := config.TestRPCConfig()
	l := log.NewNopLogger()
	d := inspect.New(rpcConfig, blockStoreMock, stateStoreMock, []indexer.EventSink{eventSinkMock}, l)

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	startedWG := &sync.WaitGroup{}
	startedWG.Add(1)
	go func() {
		startedWG.Done()
		defer wg.Done()
		require.NoError(t, d.Run(ctx))
	}()
	// FIXME: used to induce context switch.
	// Determine more deterministic method for prompting a context switch
	startedWG.Wait()
	requireConnect(t, rpcConfig.ListenAddress, 20)
	cli, err := httpclient.New(rpcConfig.ListenAddress)
	require.NoError(t, err)
	params, err := cli.ConsensusParams(ctx, &testHeight)
	require.NoError(t, err)
	require.Equal(t, params.ConsensusParams.Block.MaxGas, testMaxGas)

	cancel()
	wg.Wait()

	blockStoreMock.AssertExpectations(t)
	stateStoreMock.AssertExpectations(t)
}

func TestBlockResults(t *testing.T) {
	testHeight := int64(1)
	testGasUsed := int64(100)
	stateStoreMock := &statemocks.Store{}
	//	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	stateStoreMock.On("LoadFinalizeBlockResponses", testHeight).Return(&abcitypes.ResponseFinalizeBlock{
		TxResults: []*abcitypes.ExecTxResult{
			{
				GasUsed: testGasUsed,
			},
		},
	}, nil)
	blockStoreMock := &statemocks.BlockStore{}
	blockStoreMock.On("Base").Return(int64(0))
	blockStoreMock.On("Height").Return(testHeight)
	eventSinkMock := &indexermocks.EventSink{}
	eventSinkMock.On("Stop").Return(nil)
	eventSinkMock.On("Type").Return(indexer.EventSinkType("Mock"))

	rpcConfig := config.TestRPCConfig()
	l := log.NewNopLogger()
	d := inspect.New(rpcConfig, blockStoreMock, stateStoreMock, []indexer.EventSink{eventSinkMock}, l)

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	startedWG := &sync.WaitGroup{}
	startedWG.Add(1)
	go func() {
		startedWG.Done()
		defer wg.Done()
		require.NoError(t, d.Run(ctx))
	}()
	// FIXME: used to induce context switch.
	// Determine more deterministic method for prompting a context switch
	startedWG.Wait()
	requireConnect(t, rpcConfig.ListenAddress, 20)
	cli, err := httpclient.New(rpcConfig.ListenAddress)
	require.NoError(t, err)
	res, err := cli.BlockResults(ctx, &testHeight)
	require.NoError(t, err)
	require.Equal(t, res.TotalGasUsed, testGasUsed)

	cancel()
	wg.Wait()

	blockStoreMock.AssertExpectations(t)
	stateStoreMock.AssertExpectations(t)
}

func TestCommit(t *testing.T) {
	testHeight := int64(1)
	testRound := int32(101)
	stateStoreMock := &statemocks.Store{}
	blockStoreMock := &statemocks.BlockStore{}
	blockStoreMock.On("Base").Return(int64(0))
	blockStoreMock.On("Height").Return(testHeight)
	blockStoreMock.On("LoadBlockMeta", testHeight).Return(&types.BlockMeta{}, nil)
	blockStoreMock.On("LoadSeenCommit").Return(&types.Commit{
		Height: testHeight,
		Round:  testRound,
	}, nil)
	eventSinkMock := &indexermocks.EventSink{}
	eventSinkMock.On("Stop").Return(nil)
	eventSinkMock.On("Type").Return(indexer.EventSinkType("Mock"))

	rpcConfig := config.TestRPCConfig()
	l := log.NewNopLogger()
	d := inspect.New(rpcConfig, blockStoreMock, stateStoreMock, []indexer.EventSink{eventSinkMock}, l)

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	startedWG := &sync.WaitGroup{}
	startedWG.Add(1)
	go func() {
		startedWG.Done()
		defer wg.Done()
		require.NoError(t, d.Run(ctx))
	}()
	// FIXME: used to induce context switch.
	// Determine more deterministic method for prompting a context switch
	startedWG.Wait()
	requireConnect(t, rpcConfig.ListenAddress, 20)
	cli, err := httpclient.New(rpcConfig.ListenAddress)
	require.NoError(t, err)
	res, err := cli.Commit(ctx, &testHeight)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, res.SignedHeader.Commit.Round, testRound)

	cancel()
	wg.Wait()

	blockStoreMock.AssertExpectations(t)
	stateStoreMock.AssertExpectations(t)
}

func TestBlockByHash(t *testing.T) {
	testHeight := int64(1)
	testHash := []byte("test hash")
	testBlock := new(types.Block)
	testBlock.Header.Height = testHeight
	testBlock.Header.LastCommitHash = testHash
	stateStoreMock := &statemocks.Store{}
	blockStoreMock := &statemocks.BlockStore{}
	blockStoreMock.On("LoadBlockMeta", testHeight).Return(&types.BlockMeta{
		BlockID: types.BlockID{
			Hash: testHash,
		},
		Header: types.Header{
			Height: testHeight,
		},
	}, nil)
	blockStoreMock.On("LoadBlockByHash", testHash).Return(testBlock, nil)
	eventSinkMock := &indexermocks.EventSink{}
	eventSinkMock.On("Stop").Return(nil)
	eventSinkMock.On("Type").Return(indexer.EventSinkType("Mock"))

	rpcConfig := config.TestRPCConfig()
	l := log.NewNopLogger()
	d := inspect.New(rpcConfig, blockStoreMock, stateStoreMock, []indexer.EventSink{eventSinkMock}, l)

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	startedWG := &sync.WaitGroup{}
	startedWG.Add(1)
	go func() {
		startedWG.Done()
		defer wg.Done()
		require.NoError(t, d.Run(ctx))
	}()
	// FIXME: used to induce context switch.
	// Determine more deterministic method for prompting a context switch
	startedWG.Wait()
	requireConnect(t, rpcConfig.ListenAddress, 20)
	cli, err := httpclient.New(rpcConfig.ListenAddress)
	require.NoError(t, err)
	res, err := cli.BlockByHash(ctx, testHash)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, []byte(res.BlockID.Hash), testHash)

	cancel()
	wg.Wait()

	blockStoreMock.AssertExpectations(t)
	stateStoreMock.AssertExpectations(t)
}

func TestBlockchain(t *testing.T) {
	testHeight := int64(1)
	testBlock := new(types.Block)
	testBlockHash := []byte("test hash")
	testBlock.Header.Height = testHeight
	testBlock.Header.LastCommitHash = testBlockHash
	stateStoreMock := &statemocks.Store{}

	blockStoreMock := &statemocks.BlockStore{}
	blockStoreMock.On("Height").Return(testHeight)
	blockStoreMock.On("Base").Return(int64(0))
	blockStoreMock.On("LoadBlockMeta", testHeight).Return(&types.BlockMeta{
		BlockID: types.BlockID{
			Hash: testBlockHash,
		},
	})
	eventSinkMock := &indexermocks.EventSink{}
	eventSinkMock.On("Stop").Return(nil)
	eventSinkMock.On("Type").Return(indexer.EventSinkType("Mock"))

	rpcConfig := config.TestRPCConfig()
	l := log.NewNopLogger()
	d := inspect.New(rpcConfig, blockStoreMock, stateStoreMock, []indexer.EventSink{eventSinkMock}, l)

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	startedWG := &sync.WaitGroup{}
	startedWG.Add(1)
	go func() {
		startedWG.Done()
		defer wg.Done()
		require.NoError(t, d.Run(ctx))
	}()
	// FIXME: used to induce context switch.
	// Determine more deterministic method for prompting a context switch
	startedWG.Wait()
	requireConnect(t, rpcConfig.ListenAddress, 20)
	cli, err := httpclient.New(rpcConfig.ListenAddress)
	require.NoError(t, err)
	res, err := cli.BlockchainInfo(ctx, 0, 100)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, testBlockHash, []byte(res.BlockMetas[0].BlockID.Hash))

	cancel()
	wg.Wait()

	blockStoreMock.AssertExpectations(t)
	stateStoreMock.AssertExpectations(t)
}

func TestValidators(t *testing.T) {
	testHeight := int64(1)
	testVotingPower := int64(100)
	testValidators := types.ValidatorSet{
		Validators: []*types.Validator{
			{
				VotingPower: testVotingPower,
			},
		},
	}
	stateStoreMock := &statemocks.Store{}
	stateStoreMock.On("LoadValidators", testHeight).Return(&testValidators, nil)

	blockStoreMock := &statemocks.BlockStore{}
	blockStoreMock.On("Height").Return(testHeight)
	blockStoreMock.On("Base").Return(int64(0))
	eventSinkMock := &indexermocks.EventSink{}
	eventSinkMock.On("Stop").Return(nil)
	eventSinkMock.On("Type").Return(indexer.EventSinkType("Mock"))

	rpcConfig := config.TestRPCConfig()
	l := log.NewNopLogger()
	d := inspect.New(rpcConfig, blockStoreMock, stateStoreMock, []indexer.EventSink{eventSinkMock}, l)

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	startedWG := &sync.WaitGroup{}
	startedWG.Add(1)
	go func() {
		startedWG.Done()
		defer wg.Done()
		require.NoError(t, d.Run(ctx))
	}()
	// FIXME: used to induce context switch.
	// Determine more deterministic method for prompting a context switch
	startedWG.Wait()
	requireConnect(t, rpcConfig.ListenAddress, 20)
	cli, err := httpclient.New(rpcConfig.ListenAddress)
	require.NoError(t, err)

	testPage := 1
	testPerPage := 100
	res, err := cli.Validators(ctx, &testHeight, &testPage, &testPerPage)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, testVotingPower, res.Validators[0].VotingPower)

	cancel()
	wg.Wait()

	blockStoreMock.AssertExpectations(t)
	stateStoreMock.AssertExpectations(t)
}

func TestBlockSearch(t *testing.T) {
	testHeight := int64(1)
	testBlockHash := []byte("test hash")
	testQuery := "block.height = 1"
	stateStoreMock := &statemocks.Store{}

	blockStoreMock := &statemocks.BlockStore{}
	eventSinkMock := &indexermocks.EventSink{}
	eventSinkMock.On("Stop").Return(nil)
	eventSinkMock.On("Type").Return(indexer.KV)
	blockStoreMock.On("LoadBlock", testHeight).Return(&types.Block{
		Header: types.Header{
			Height: testHeight,
		},
	}, nil)
	blockStoreMock.On("LoadBlockMeta", testHeight).Return(&types.BlockMeta{
		BlockID: types.BlockID{
			Hash: testBlockHash,
		},
	})
	eventSinkMock.On("SearchBlockEvents", mock.Anything,
		mock.MatchedBy(func(q *query.Query) bool { return testQuery == q.String() })).
		Return([]int64{testHeight}, nil)
	rpcConfig := config.TestRPCConfig()
	l := log.NewNopLogger()
	d := inspect.New(rpcConfig, blockStoreMock, stateStoreMock, []indexer.EventSink{eventSinkMock}, l)

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	startedWG := &sync.WaitGroup{}
	startedWG.Add(1)
	go func() {
		startedWG.Done()
		defer wg.Done()
		require.NoError(t, d.Run(ctx))
	}()
	// FIXME: used to induce context switch.
	// Determine more deterministic method for prompting a context switch
	startedWG.Wait()
	requireConnect(t, rpcConfig.ListenAddress, 20)
	cli, err := httpclient.New(rpcConfig.ListenAddress)
	require.NoError(t, err)

	testPage := 1
	testPerPage := 100
	testOrderBy := "desc"
	res, err := cli.BlockSearch(ctx, testQuery, &testPage, &testPerPage, testOrderBy)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, testBlockHash, []byte(res.Blocks[0].BlockID.Hash))

	cancel()
	wg.Wait()

	blockStoreMock.AssertExpectations(t)
	stateStoreMock.AssertExpectations(t)
}

func requireConnect(t testing.TB, addr string, retries int) {
	parts := strings.SplitN(addr, "://", 2)
	if len(parts) != 2 {
		t.Fatalf("malformed address to dial: %s", addr)
	}
	var err error
	for i := 0; i < retries; i++ {
		var conn net.Conn
		conn, err = net.Dial(parts[0], parts[1])
		if err == nil {
			conn.Close()
			return
		}
		// FIXME attempt to yield and let the other goroutine continue execution.
		time.Sleep(time.Microsecond * 100)
	}
	t.Fatalf("unable to connect to server %s after %d tries: %s", addr, retries, err)
}
