package http_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/light/provider"
	lighthttp "github.com/tendermint/tendermint/light/provider/http"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	rpctest "github.com/tendermint/tendermint/rpc/test"
	"github.com/tendermint/tendermint/types"
)

func TestNewProvider(t *testing.T) {
	c, err := lighthttp.New("chain-test", "192.168.0.1:26657")
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("%s", c), "http{http://192.168.0.1:26657}")

	c, err = lighthttp.New("chain-test", "http://153.200.0.1:26657")
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("%s", c), "http{http://153.200.0.1:26657}")

	c, err = lighthttp.New("chain-test", "153.200.0.1")
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("%s", c), "http{http://153.200.0.1}")
}

func TestMain(m *testing.M) {
	app := kvstore.NewApplication()
	app.RetainBlocks = 9
	node := rpctest.StartTendermint(app)

	code := m.Run()

	rpctest.StopTendermint(node)
	os.Exit(code)
}

func TestProvider(t *testing.T) {
	cfg := rpctest.GetConfig()
	defer os.RemoveAll(cfg.RootDir)
	rpcAddr := cfg.RPC.ListenAddress
	genDoc, err := types.GenesisDocFromFile(cfg.GenesisFile())
	if err != nil {
		panic(err)
	}
	chainID := genDoc.ChainID
	t.Log("chainID:", chainID)

	c, err := rpchttp.New(rpcAddr, "/websocket")
	require.Nil(t, err)

	p := lighthttp.NewWithClient(chainID, c)
	require.NoError(t, err)
	require.NotNil(t, p)

	// let it produce some blocks
	err = rpcclient.WaitForHeight(c, 10, nil)
	require.NoError(t, err)

	// let's get the highest block
	sh, err := p.LightBlock(context.Background(), 0)
	require.NoError(t, err)
	assert.True(t, sh.Height < 1000)

	// let's check this is valid somehow
	assert.Nil(t, sh.ValidateBasic(chainID))

	// historical queries now work :)
	lower := sh.Height - 3
	sh, err = p.LightBlock(context.Background(), lower)
	require.NoError(t, err)
	assert.Equal(t, lower, sh.Height)

	// fetching missing heights (both future and pruned) should return appropriate errors
	lb, err := p.LightBlock(context.Background(), 1000)
	require.Error(t, err)
	require.Nil(t, lb)
	assert.Equal(t, provider.ErrHeightTooHigh, err)

	_, err = p.LightBlock(context.Background(), 1)
	require.Error(t, err)
	assert.Equal(t, provider.ErrLightBlockNotFound, err)
}
