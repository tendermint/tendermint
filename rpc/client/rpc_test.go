package client_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	merkle "github.com/tendermint/go-merkle"
	rpctest "github.com/tendermint/tendermint/rpc/test"
	"github.com/tendermint/tendermint/types"
)

// Make sure status is correct (we connect properly)
func TestStatus(t *testing.T) {
	c := rpctest.GetClient()
	chainID := rpctest.GetConfig().GetString("chain_id")
	status, err := c.Status()
	if assert.Nil(t, err) {
		assert.Equal(t, chainID, status.NodeInfo.Network)
	}
}

// Make sure info is correct (we connect properly)
func TestInfo(t *testing.T) {
	c := rpctest.GetClient()
	status, err := c.Status()
	require.Nil(t, err)
	info, err := c.ABCIInfo()
	require.Nil(t, err)
	assert.EqualValues(t, status.LatestBlockHeight, info.Response.LastBlockHeight)
	assert.True(t, strings.HasPrefix(info.Response.Data, "size:"))
}

// Make some app checks
func TestAppCalls(t *testing.T) {
	assert, require := assert.New(t), require.New(t)
	c := rpctest.GetClient()
	_, err := c.Block(1)
	assert.NotNil(err) // no block yet
	k, v, tx := MakeTxKV()
	_, err = c.BroadcastTxCommit(tx)
	require.Nil(err)
	// wait before querying
	time.Sleep(time.Second * 2)
	qres, err := c.ABCIQuery("/key", k, false)
	if assert.Nil(err) && assert.True(qres.Response.Code.IsOK()) {
		data := qres.Response
		// assert.Equal(k, data.GetKey())  // only returned for proofs
		assert.Equal(v, data.GetValue())
	}
	// and we can even check the block is added
	block, err := c.Block(3)
	assert.Nil(err) // now it's good :)
	appHash := block.BlockMeta.Header.AppHash
	assert.True(len(appHash) > 0)

	// and get the corresponding commit with the same apphash
	commit, err := c.Commit(3)
	assert.Nil(err) // now it's good :)
	cappHash := commit.Header.AppHash
	assert.Equal(appHash, cappHash)
	assert.NotNil(commit.Commit)

	// compare the commits (note Commit(2) has commit from Block(3))
	commit2, err := c.Commit(2)
	assert.Nil(err) // now it's good :)
	assert.Equal(block.Block.LastCommit, commit2.Commit)

	// and we got a proof that works!
	pres, err := c.ABCIQuery("/key", k, true)
	if assert.Nil(err) && assert.True(pres.Response.Code.IsOK()) {
		proof, err := merkle.ReadProof(pres.Response.GetProof())
		if assert.Nil(err) {
			key := pres.Response.GetKey()
			value := pres.Response.GetValue()
			assert.Equal(appHash, proof.RootHash)
			valid := proof.Verify(key, value, appHash)
			assert.True(valid)
		}
	}
}

// run most calls just to make sure no syntax errors
func TestNoErrors(t *testing.T) {
	assert := assert.New(t)
	c := rpctest.GetClient()
	_, err := c.NetInfo()
	assert.Nil(err)
	_, err = c.BlockchainInfo(0, 4)
	assert.Nil(err)
	// TODO: check with a valid height
	_, err = c.Block(1000)
	assert.NotNil(err)
	// maybe this is an error???
	// _, err = c.DialSeeds([]string{"one", "two"})
	// assert.Nil(err)
	gen, err := c.Genesis()
	if assert.Nil(err) {
		chainID := rpctest.GetConfig().GetString("chain_id")
		assert.Equal(chainID, gen.Genesis.ChainID)
	}
}

func TestSubscriptions(t *testing.T) {
	assert, require := assert.New(t), require.New(t)
	c := rpctest.GetClient()
	err := c.StartWebsocket()
	require.Nil(err)
	defer c.StopWebsocket()

	// subscribe to a transaction event
	_, _, tx := MakeTxKV()
	// this causes a panic in tendermint core!!!
	eventType := types.EventStringTx(types.Tx(tx))
	c.Subscribe(eventType)
	read := 0

	// set up a listener
	r, e := c.GetEventChannels()
	go func() {
		// read one event in the background
		select {
		case <-r:
			// TODO: actually parse this or something
			read += 1
		case err := <-e:
			panic(err)
		}
	}()

	// make sure nothing has happened yet.
	assert.Equal(0, read)

	// send a tx and wait for it to propogate
	_, err = c.BroadcastTxCommit(tx)
	assert.Nil(err, string(tx))
	// wait before querying
	time.Sleep(time.Second)

	// now make sure the event arrived
	assert.Equal(1, read)
}
