package http_test

import (
	"context"
	"errors"
	"testing"
	"time"

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
	require.Equal(t, c.ID(), "http{http://192.168.0.1:26657}")

	c, err = lighthttp.New("chain-test", "http://153.200.0.1:26657")
	require.NoError(t, err)
	require.Equal(t, c.ID(), "http{http://153.200.0.1:26657}")

	c, err = lighthttp.New("chain-test", "153.200.0.1")
	require.NoError(t, err)
	require.Equal(t, c.ID(), "http{http://153.200.0.1}")
}

func TestProvider(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)

	// start a tendermint node in the background to test against
	app := kvstore.NewApplication()
	app.RetainBlocks = 9
	_, closer, err := rpctest.StartTendermint(ctx, cfg, app)
	require.NoError(t, err)

	rpcAddr := cfg.RPC.ListenAddress
	genDoc, err := types.GenesisDocFromFile(cfg.GenesisFile())
	require.NoError(t, err)

	chainID := genDoc.ChainID
	t.Log("chainID:", chainID)

	c, err := rpchttp.New(rpcAddr)
	require.NoError(t, err)

	p := lighthttp.NewWithClient(chainID, c)
	require.NoError(t, err)
	require.NotNil(t, p)

	// let it produce some blocks
	err = rpcclient.WaitForHeight(ctx, c, 10, nil)
	require.NoError(t, err)

	// let's get the highest block
	lb, err := p.LightBlock(ctx, 0)
	require.NoError(t, err)
	assert.True(t, lb.Height < 9001, "height=%d", lb.Height)

	// let's check this is valid somehow
	assert.Nil(t, lb.ValidateBasic(chainID))

	// historical queries now work :)
	lower := lb.Height - 3
	lb, err = p.LightBlock(ctx, lower)
	require.NoError(t, err)
	assert.Equal(t, lower, lb.Height)

	// fetching missing heights (both future and pruned) should return appropriate errors
	lb, err = p.LightBlock(ctx, 9001)
	require.Error(t, err)
	require.Nil(t, lb)
	assert.ErrorIs(t, err, provider.ErrHeightTooHigh)

	lb, err = p.LightBlock(ctx, 1)
	require.Error(t, err)
	require.Nil(t, lb)
	assert.ErrorIs(t, err, provider.ErrLightBlockNotFound)

	// if the provider is unable to provide four more blocks then we should return
	// an unreliable peer error
	for i := 0; i < 4; i++ {
		_, err = p.LightBlock(ctx, 1)
	}
	assert.IsType(t, provider.ErrUnreliableProvider{}, err)

	// shut down tendermint node
	require.NoError(t, closer(ctx))
	cancel()

	time.Sleep(10 * time.Second)
	lb, err = p.LightBlock(ctx, lower+2)
	// Either the connection should be refused, or the context canceled.
	require.Error(t, err)
	require.Nil(t, lb)
	if !errors.Is(err, provider.ErrConnectionClosed) && !errors.Is(err, context.Canceled) {
		assert.Fail(t, "Incorrect error", "wanted connection closed or context canceled, got %v", err)
	}
}
