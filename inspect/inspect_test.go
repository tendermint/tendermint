package inspect_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	abci_types "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/inspect"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	http_client "github.com/tendermint/tendermint/rpc/client/http"
	"github.com/tendermint/tendermint/state/indexer"
	indexer_mocks "github.com/tendermint/tendermint/state/indexer/mocks"
	state_mocks "github.com/tendermint/tendermint/state/mocks"
	"github.com/tendermint/tendermint/types"
)

func TestInspectConstructor(t *testing.T) {
	cfg := config.ResetTestRoot("test")
	t.Cleanup(leaktest.Check(t))
	defer func() { _ = os.RemoveAll(cfg.RootDir) }()
	t.Run("from config", func(t *testing.T) {
		d, err := inspect.NewFromConfig(cfg)
		require.NoError(t, err)
		require.NotNil(t, d)
	})

}

func TestInspectRun(t *testing.T) {
	cfg := config.ResetTestRoot("test")
	t.Cleanup(leaktest.Check(t))
	defer func() { _ = os.RemoveAll(cfg.RootDir) }()
	t.Run("from config", func(t *testing.T) {
		d, err := inspect.NewFromConfig(cfg)
		require.NoError(t, err)
		ctx, cancel := context.WithCancel(context.Background())
		stoppedWG := &sync.WaitGroup{}
		stoppedWG.Add(1)
		go func() {
			require.NoError(t, d.Run(ctx))
			stoppedWG.Done()
		}()
		cancel()
		stoppedWG.Wait()
	})

}

func TestInspectServeInfoRPC(t *testing.T) {
	testHeight := int64(1)
	testBlock := new(types.Block)
	testBlock.Header.Height = testHeight
	testBlock.Header.LastCommitHash = []byte("test hash")
	stateStoreMock := &state_mocks.Store{}

	blockStoreMock := &state_mocks.BlockStore{}
	blockStoreMock.On("Height").Return(testHeight)
	blockStoreMock.On("Base").Return(int64(0))
	blockStoreMock.On("LoadBlockMeta", testHeight).Return(&types.BlockMeta{})
	blockStoreMock.On("LoadBlock", testHeight).Return(testBlock)
	eventSinkMock := &indexer_mocks.EventSink{}
	eventSinkMock.On("Stop").Return(nil)

	rpcConfig := config.TestRPCConfig()
	l := log.TestingLogger()
	d := inspect.New(rpcConfig, blockStoreMock, stateStoreMock, []indexer.EventSink{eventSinkMock}, l)
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(t, d.Run(ctx))
	}()
	// force context switch
	time.Sleep(10 * time.Millisecond)
	requireConnect(t, rpcConfig.ListenAddress, 15)
	cli, err := http_client.New(rpcConfig.ListenAddress)
	require.NoError(t, err)
	resultBlock, err := cli.Block(context.Background(), &testHeight)
	require.NoError(t, err)
	require.Equal(t, testBlock.Height, resultBlock.Block.Height)
	require.Equal(t, testBlock.LastCommitHash, resultBlock.Block.LastCommitHash)
	cancel()
	wg.Wait()

	blockStoreMock.AssertExpectations(t)
	stateStoreMock.AssertExpectations(t)
}

func TestInspectTxSearch(t *testing.T) {
	testHash := []byte("test")
	testTx := []byte("tx")
	testQuery := fmt.Sprintf("tx.hash='%s'", string(testHash))
	testTxResult := &abci_types.TxResult{
		Height: 1,
		Index:  100,
		Tx:     testTx,
	}

	stateStoreMock := &state_mocks.Store{}
	blockStoreMock := &state_mocks.BlockStore{}
	eventSinkMock := &indexer_mocks.EventSink{}
	eventSinkMock.On("Stop").Return(nil)
	eventSinkMock.On("Type").Return(indexer.KV)
	eventSinkMock.On("SearchTxEvents", mock.Anything,
		mock.MatchedBy(func(q *query.Query) bool { return testQuery == q.String() })).
		Return([]*abci_types.TxResult{testTxResult}, nil)

	rpcConfig := config.TestRPCConfig()
	l := log.TestingLogger()
	d := inspect.New(rpcConfig, blockStoreMock, stateStoreMock, []indexer.EventSink{eventSinkMock}, l)
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(t, d.Run(ctx))
	}()
	requireConnect(t, rpcConfig.ListenAddress, 15)
	cli, err := http_client.New(rpcConfig.ListenAddress)
	require.NoError(t, err)

	var page int = 1
	resultTxSearch, err := cli.TxSearch(context.Background(), testQuery, false, &page, &page, "")
	require.NoError(t, err)
	require.Len(t, resultTxSearch.Txs, 1)
	require.Equal(t, types.Tx(testTx), resultTxSearch.Txs[0].Tx)

	cancel()
	wg.Wait()

	eventSinkMock.AssertExpectations(t)
	stateStoreMock.AssertExpectations(t)
	blockStoreMock.AssertExpectations(t)
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
	}
	t.Fatalf("unable to connect to server %s after %d tries: %s", addr, retries, err)
}
