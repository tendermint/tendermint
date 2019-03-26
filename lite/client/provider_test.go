package client

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpctest "github.com/tendermint/tendermint/rpc/test"
	"github.com/tendermint/tendermint/types"
)

func TestMain(m *testing.M) {
	app := kvstore.NewKVStoreApplication()
	node := rpctest.StartTendermint(app)

	code := m.Run()

	rpctest.StopTendermint(node)
	os.Exit(code)
}

func TestProvider(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	cfg := rpctest.GetConfig()
	defer os.RemoveAll(cfg.RootDir)
	rpcAddr := cfg.RPC.ListenAddress
	genDoc, err := types.GenesisDocFromFile(cfg.GenesisFile())
	if err != nil {
		panic(err)
	}
	chainID := genDoc.ChainID
	t.Log("chainID:", chainID)
	p := NewHTTPProvider(chainID, rpcAddr)
	require.NotNil(t, p)

	// let it produce some blocks
	err = rpcclient.WaitForHeight(p.(*provider).client, 6, nil)
	require.Nil(err)

	// let's get the highest block
	fc, err := p.LatestFullCommit(chainID, 1, 1<<63-1)

	require.Nil(err, "%+v", err)
	sh := fc.Height()
	assert.True(sh < 5000)

	// let's check this is valid somehow
	assert.Nil(fc.ValidateFull(chainID))

	// historical queries now work :)
	lower := sh - 5
	fc, err = p.LatestFullCommit(chainID, lower, lower)
	assert.Nil(err, "%+v", err)
	assert.Equal(lower, fc.Height())

}
