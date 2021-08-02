package node_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	abci_types "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/node"
	http_client "github.com/tendermint/tendermint/rpc/client/http"
	"github.com/tendermint/tendermint/state/indexer"
	indexer_mocks "github.com/tendermint/tendermint/state/indexer/mocks"
	state_mocks "github.com/tendermint/tendermint/state/mocks"
	"github.com/tendermint/tendermint/types"
)

func TestDebugConstructor(t *testing.T) {
	config := cfg.ResetTestRoot("test")
	t.Cleanup(leaktest.Check(t))
	defer func() { _ = os.RemoveAll(config.RootDir) }()
	t.Run("from config", func(t *testing.T) {
		d, err := node.NewDebugFromConfig(config)
		require.NoError(t, err)
		require.NotNil(t, d)

		d.OnStop()
	})

}

func TestDebugRun(t *testing.T) {
	config := cfg.ResetTestRoot("test")
	t.Cleanup(leaktest.Check(t))
	defer func() { _ = os.RemoveAll(config.RootDir) }()
	t.Run("from config", func(t *testing.T) {
		d, err := node.NewDebugFromConfig(config)
		require.NoError(t, err)
		err = d.OnStart()
		require.NoError(t, err)
		d.OnStop()
	})

}

func TestDebugServeInfoRPC(t *testing.T) {
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

	rpcConfig := config.TestRPCConfig()
	d := node.NewDebug(rpcConfig, blockStoreMock, stateStoreMock, []indexer.EventSink{eventSinkMock})
	require.NoError(t, d.OnStart())
	cli, err := http_client.New(rpcConfig.ListenAddress)
	require.NoError(t, err)
	resultBlock, err := cli.Block(context.Background(), &testHeight)
	require.NoError(t, err)
	require.Equal(t, testBlock.Height, resultBlock.Block.Height)
	require.Equal(t, testBlock.LastCommitHash, resultBlock.Block.LastCommitHash)

	d.OnStop()

	blockStoreMock.AssertExpectations(t)
	stateStoreMock.AssertExpectations(t)
}

func TestDebugTxSearch(t *testing.T) {
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
	eventSinkMock.On("Type").Return(indexer.KV)
	eventSinkMock.On("SearchTxEvents", mock.Anything, mock.MatchedBy(func(q *query.Query) bool { return testQuery == q.String() })).
		Return([]*abci_types.TxResult{testTxResult}, nil)

	rpcConfig := config.TestRPCConfig()
	d := node.NewDebug(rpcConfig, blockStoreMock, stateStoreMock, []indexer.EventSink{eventSinkMock})
	require.NoError(t, d.OnStart())
	cli, err := http_client.New(rpcConfig.ListenAddress)
	require.NoError(t, err)

	var page int = 1
	resultTxSearch, err := cli.TxSearch(context.Background(), testQuery, false, &page, &page, "")
	require.NoError(t, err)
	require.Len(t, resultTxSearch.Txs, 1)
	require.Equal(t, types.Tx(testTx), resultTxSearch.Txs[0].Tx)

	d.OnStop()

	eventSinkMock.AssertExpectations(t)
}
