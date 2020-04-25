package http_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/lite2/provider"
	"github.com/tendermint/tendermint/lite2/provider/http"
	litehttp "github.com/tendermint/tendermint/lite2/provider/http"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpctest "github.com/tendermint/tendermint/rpc/test"
	"github.com/tendermint/tendermint/types"
)

func TestNewProvider(t *testing.T) {
	c, err := http.New("chain-test", "192.168.0.1:26657")
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("%s", c), "http{http://192.168.0.1:26657}")

	c, err = http.New("chain-test", "http://153.200.0.1:26657")
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("%s", c), "http{http://153.200.0.1:26657}")

	c, err = http.New("chain-test", "153.200.0.1")
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("%s", c), "http{http://153.200.0.1}")
}

func TestMain(m *testing.M) {
	app := kvstore.NewApplication()
	app.RetainBlocks = 5
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
	p, err := litehttp.New(chainID, rpcAddr)
	require.Nil(t, err)
	require.NotNil(t, p)

	// let it produce some blocks
	err = rpcclient.WaitForHeight(p.(rpcclient.StatusClient), 6, nil)
	require.Nil(t, err)

	// let's get the highest block
	sh, err := p.SignedHeader(0)

	require.Nil(t, err, "%+v", err)
	assert.True(t, sh.Height < 5000)

	// let's check this is valid somehow
	assert.Nil(t, sh.ValidateBasic(chainID))

	// historical queries now work :)
	lower := sh.Height - 3
	sh, err = p.SignedHeader(lower)
	assert.Nil(t, err, "%+v", err)
	assert.Equal(t, lower, sh.Height)

	// fetching missing heights (both future and pruned) should return appropriate errors
	_, err = p.SignedHeader(1000)
	require.Error(t, err)
	assert.Equal(t, provider.ErrSignedHeaderNotFound, err)

	_, err = p.ValidatorSet(1000)
	require.Error(t, err)
	assert.Equal(t, provider.ErrValidatorSetNotFound, err)

	_, err = p.SignedHeader(1)
	require.Error(t, err)
	assert.Equal(t, provider.ErrSignedHeaderNotFound, err)

	_, err = p.ValidatorSet(1)
	require.Error(t, err)
	assert.Equal(t, provider.ErrValidatorSetNotFound, err)
}
