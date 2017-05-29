package client_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/merkleeyes/iavl"
	merktest "github.com/tendermint/merkleeyes/testutil"
	"github.com/tendermint/tendermint/rpc/client"
	rpctest "github.com/tendermint/tendermint/rpc/test"
)

func getHTTPClient() *client.HTTP {
	rpcAddr := rpctest.GetConfig().RPC.ListenAddress
	return client.NewHTTP(rpcAddr, "/websocket")
}

func getLocalClient() client.Local {
	return client.NewLocal(node)
}

// GetClients returns a slice of clients for table-driven tests
func GetClients() []client.Client {
	return []client.Client{
		getHTTPClient(),
		getLocalClient(),
	}
}

// Make sure status is correct (we connect properly)
func TestStatus(t *testing.T) {
	for i, c := range GetClients() {
		moniker := rpctest.GetConfig().Moniker
		status, err := c.Status()
		require.Nil(t, err, "%d: %+v", i, err)
		assert.Equal(t, moniker, status.NodeInfo.Moniker)
	}
}

// Make sure info is correct (we connect properly)
func TestInfo(t *testing.T) {
	for i, c := range GetClients() {
		// status, err := c.Status()
		// require.Nil(t, err, "%+v", err)
		info, err := c.ABCIInfo()
		require.Nil(t, err, "%d: %+v", i, err)
		// TODO: this is not correct - fix merkleeyes!
		// assert.EqualValues(t, status.LatestBlockHeight, info.Response.LastBlockHeight)
		assert.True(t, strings.HasPrefix(info.Response.Data, "size"))
	}
}

func TestNetInfo(t *testing.T) {
	for i, c := range GetClients() {
		nc, ok := c.(client.NetworkClient)
		require.True(t, ok, "%d", i)
		netinfo, err := nc.NetInfo()
		require.Nil(t, err, "%d: %+v", i, err)
		assert.True(t, netinfo.Listening)
		assert.Equal(t, 0, len(netinfo.Peers))
	}
}

func TestDumpConsensusState(t *testing.T) {
	for i, c := range GetClients() {
		// FIXME: fix server so it doesn't panic on invalid input
		nc, ok := c.(client.NetworkClient)
		require.True(t, ok, "%d", i)
		cons, err := nc.DumpConsensusState()
		require.Nil(t, err, "%d: %+v", i, err)
		assert.NotEmpty(t, cons.RoundState)
		assert.Empty(t, cons.PeerRoundStates)
	}
}

func TestGenesisAndValidators(t *testing.T) {
	for i, c := range GetClients() {

		// make sure this is the right genesis file
		gen, err := c.Genesis()
		require.Nil(t, err, "%d: %+v", i, err)
		// get the genesis validator
		require.Equal(t, 1, len(gen.Genesis.Validators))
		gval := gen.Genesis.Validators[0]

		// get the current validators
		vals, err := c.Validators()
		require.Nil(t, err, "%d: %+v", i, err)
		require.Equal(t, 1, len(vals.Validators))
		val := vals.Validators[0]

		// make sure the current set is also the genesis set
		assert.Equal(t, gval.Amount, val.VotingPower)
		assert.Equal(t, gval.PubKey, val.PubKey)
	}
}

// Make some app checks
func TestAppCalls(t *testing.T) {
	assert, require := assert.New(t), require.New(t)
	for i, c := range GetClients() {

		// get an offset of height to avoid racing and guessing
		s, err := c.Status()
		require.Nil(err, "%d: %+v", i, err)
		// sh is start height or status height
		sh := s.LatestBlockHeight

		// look for the future
		_, err = c.Block(sh + 2)
		assert.NotNil(err) // no block yet

		// write something
		k, v, tx := merktest.MakeTxKV()
		bres, err := c.BroadcastTxCommit(tx)
		require.Nil(err, "%d: %+v", i, err)
		require.True(bres.DeliverTx.Code.IsOK())
		txh := bres.Height
		apph := txh + 1 // this is where the tx will be applied to the state

		// wait before querying
		client.WaitForHeight(c, apph, nil)
		qres, err := c.ABCIQuery("/key", k, false)
		if assert.Nil(err) && assert.True(qres.Code.IsOK()) {
			// assert.Equal(k, data.GetKey())  // only returned for proofs
			assert.EqualValues(v, qres.Value)
		}

		// make sure we can lookup the tx with proof
		// ptx, err := c.Tx(bres.Hash, true)
		ptx, err := c.Tx(bres.Hash, true)
		require.Nil(err, "%d: %+v", i, err)
		assert.Equal(txh, ptx.Height)
		assert.EqualValues(tx, ptx.Tx)

		// and we can even check the block is added
		block, err := c.Block(apph)
		require.Nil(err, "%d: %+v", i, err)
		appHash := block.BlockMeta.Header.AppHash
		assert.True(len(appHash) > 0)
		assert.EqualValues(apph, block.BlockMeta.Header.Height)

		// check blockchain info, now that we know there is info
		// TODO: is this commented somewhere that they are returned
		// in order of descending height???
		info, err := c.BlockchainInfo(apph, apph)
		require.Nil(err, "%d: %+v", i, err)
		assert.True(info.LastHeight >= apph)
		if assert.Equal(1, len(info.BlockMetas)) {
			lastMeta := info.BlockMetas[0]
			assert.EqualValues(apph, lastMeta.Header.Height)
			bMeta := block.BlockMeta
			assert.Equal(bMeta.Header.AppHash, lastMeta.Header.AppHash)
			assert.Equal(bMeta.BlockID, lastMeta.BlockID)
		}

		// and get the corresponding commit with the same apphash
		commit, err := c.Commit(apph)
		require.Nil(err, "%d: %+v", i, err)
		cappHash := commit.Header.AppHash
		assert.Equal(appHash, cappHash)
		assert.NotNil(commit.Commit)

		// compare the commits (note Commit(2) has commit from Block(3))
		commit2, err := c.Commit(apph - 1)
		require.Nil(err, "%d: %+v", i, err)
		assert.Equal(block.Block.LastCommit, commit2.Commit)

		// and we got a proof that works!
		pres, err := c.ABCIQuery("/key", k, true)
		if assert.Nil(err) && assert.True(pres.Code.IsOK()) {
			proof, err := iavl.ReadProof(pres.Proof)
			if assert.Nil(err) {
				key := pres.Key
				value := pres.Value
				assert.EqualValues(appHash, proof.RootHash)
				valid := proof.Verify(key, value, appHash)
				assert.True(valid)
			}
		}
	}
}
