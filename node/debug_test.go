package node_test

import (
	"context"
	"testing"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/node"
	http_client "github.com/tendermint/tendermint/rpc/client/http"
	state_mocks "github.com/tendermint/tendermint/state/mocks"
	"github.com/tendermint/tendermint/types"
)

func TestDebugConstructor(t *testing.T) {
	t.Cleanup(leaktest.Check(t))
	t.Run("from config", func(t *testing.T) {
		d, err := node.NewDebugFromConfig(&config.Config{
			BaseConfig: config.TestBaseConfig(),
			RPC:        config.TestRPCConfig(),
		})
		require.NoError(t, err)
		require.NotNil(t, d)

		d.OnStop()
	})

}

func TestDebugRun(t *testing.T) {
	t.Cleanup(leaktest.Check(t))
	t.Run("from config", func(t *testing.T) {
		d, err := node.NewDebugFromConfig(&config.Config{
			BaseConfig: config.TestBaseConfig(),
			RPC:        config.TestRPCConfig(),
		})
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

	rpcConfig := config.TestRPCConfig()
	d := node.NewDebug(rpcConfig, blockStoreMock, stateStoreMock)
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
