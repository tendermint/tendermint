package client_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rpctest "github.com/tendermint/tendermint/rpc/test"

	"github.com/tendermint/tendermint/certifiers"
	"github.com/tendermint/tendermint/certifiers/client"
	certerr "github.com/tendermint/tendermint/certifiers/errors"
)

func TestProvider(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	cfg := rpctest.GetConfig()
	rpcAddr := cfg.RPC.ListenAddress
	chainID := cfg.ChainID
	p := client.NewHTTPProvider(rpcAddr)
	require.NotNil(t, p)

	// let it produce some blocks
	time.Sleep(500 * time.Millisecond)

	// let's get the highest block
	seed, err := p.LatestCommit()

	require.Nil(err, "%+v", err)
	sh := seed.Height()
	vhash := seed.Header.ValidatorsHash
	assert.True(sh < 5000)

	// let's check this is valid somehow
	assert.Nil(seed.ValidateBasic(chainID))
	cert := certifiers.NewStatic(chainID, seed.Validators)

	// historical queries now work :)
	lower := sh - 5
	seed, err = p.GetByHeight(lower)
	assert.Nil(err, "%+v", err)
	assert.Equal(lower, seed.Height())

	// also get by hash (given the match)
	seed, err = p.GetByHash(vhash)
	require.Nil(err, "%+v", err)
	require.Equal(vhash, seed.Header.ValidatorsHash)
	err = cert.Certify(seed.Commit)
	assert.Nil(err, "%+v", err)

	// get by hash fails without match
	seed, err = p.GetByHash([]byte("foobar"))
	assert.NotNil(err)
	assert.True(certerr.IsCommitNotFoundErr(err))

	// storing the seed silently ignored
	err = p.StoreCommit(seed)
	assert.Nil(err, "%+v", err)
}
