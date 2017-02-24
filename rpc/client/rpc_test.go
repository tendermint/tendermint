package client_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	merkle "github.com/tendermint/go-merkle"
	merktest "github.com/tendermint/merkleeyes/testutil"
	"github.com/tendermint/tendermint/rpc/client"
	rpctest "github.com/tendermint/tendermint/rpc/test"
)

func getHTTPClient() *client.HTTP {
	rpcAddr := rpctest.GetConfig().GetString("rpc_laddr")
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
		chainID := rpctest.GetConfig().GetString("chain_id")
		status, err := c.Status()
		require.Nil(t, err, "%d: %+v", i, err)
		assert.Equal(t, chainID, status.NodeInfo.Network)
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

// FIXME: This seems to trigger a race condition with client.Local
// go test -v -race . -run=DumpCons
// func TestDumpConsensusState(t *testing.T) {
// 	for i, c := range GetClients() {
// 		// FIXME: fix server so it doesn't panic on invalid input
// 		nc, ok := c.(client.NetworkClient)
// 		require.True(t, ok, "%d", i)
// 		cons, err := nc.DumpConsensusState()
// 		require.Nil(t, err, "%d: %+v", i, err)
// 		assert.NotEmpty(t, cons.RoundState)
// 		assert.Empty(t, cons.PeerRoundStates)
// 	}
// }

func TestGenesisAndValidators(t *testing.T) {
	for i, c := range GetClients() {
		chainID := rpctest.GetConfig().GetString("chain_id")

		// make sure this is the right genesis file
		gen, err := c.Genesis()
		require.Nil(t, err, "%d: %+v", i, err)
		assert.Equal(t, chainID, gen.Genesis.ChainID)
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
		_, err = c.BroadcastTxCommit(tx)
		require.Nil(err, "%d: %+v", i, err)
		// wait before querying
		time.Sleep(time.Second * 1)
		qres, err := c.ABCIQuery("/key", k, false)
		if assert.Nil(err) && assert.True(qres.Response.Code.IsOK()) {
			data := qres.Response
			// assert.Equal(k, data.GetKey())  // only returned for proofs
			assert.Equal(v, data.GetValue())
		}
		// +/- 1 making my head hurt
		h := int(qres.Response.Height) - 1

		// and we can even check the block is added
		block, err := c.Block(h)
		require.Nil(err, "%d: %+v", i, err)
		appHash := block.BlockMeta.Header.AppHash
		assert.True(len(appHash) > 0)
		assert.EqualValues(h, block.BlockMeta.Header.Height)

		// check blockchain info, now that we know there is info
		// TODO: is this commented somewhere that they are returned
		// in order of descending height???
		info, err := c.BlockchainInfo(h-2, h)
		require.Nil(err, "%d: %+v", i, err)
		assert.True(info.LastHeight > 2)
		if assert.Equal(3, len(info.BlockMetas)) {
			lastMeta := info.BlockMetas[0]
			assert.EqualValues(h, lastMeta.Header.Height)
			bMeta := block.BlockMeta
			assert.Equal(bMeta.Header.AppHash, lastMeta.Header.AppHash)
			assert.Equal(bMeta.BlockID, lastMeta.BlockID)
		}

		// and get the corresponding commit with the same apphash
		commit, err := c.Commit(h)
		require.Nil(err, "%d: %+v", i, err)
		cappHash := commit.Header.AppHash
		assert.Equal(appHash, cappHash)
		assert.NotNil(commit.Commit)

		// compare the commits (note Commit(2) has commit from Block(3))
		commit2, err := c.Commit(h - 1)
		require.Nil(err, "%d: %+v", i, err)
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
}

// TestSubscriptions only works for HTTPClient
//
// TODO: generalize this functionality -> Local and Client
// func TestSubscriptions(t *testing.T) {
// 	require := require.New(t)
// 	c := getHTTPClient()
// 	err := c.StartWebsocket()
// 	require.Nil(err)
// 	defer c.StopWebsocket()

// 	// subscribe to a transaction event
// 	_, _, tx := merktest.MakeTxKV()
// 	eventType := types.EventStringTx(types.Tx(tx))
// 	c.Subscribe(eventType)

// 	// set up a listener
// 	r, e := c.GetEventChannels()
// 	go func() {
// 		// send a tx and wait for it to propogate
// 		_, err = c.BroadcastTxCommit(tx)
// 		require.Nil(err, string(tx))
// 	}()

// 	checkData := func(data []byte, kind byte) {
// 		x := []interface{}{}
// 		err := json.Unmarshal(data, &x)
// 		require.Nil(err)
// 		// gotta love wire's json format
// 		require.EqualValues(kind, x[0])
// 	}

// 	res := <-r
// 	checkData(res, ctypes.ResultTypeSubscribe)

// 	// read one event, must be success
// 	select {
// 	case res := <-r:
// 		checkData(res, ctypes.ResultTypeEvent)
// 		// this is good.. let's get the data... ugh...
// 		// result := new(ctypes.TMResult)
// 		// wire.ReadJSON(result, res, &err)
// 		// require.Nil(err, "%+v", err)
// 		// event, ok := (*result).(*ctypes.ResultEvent)
// 		// require.True(ok)
// 		// assert.Equal("foo", event.Name)
// 		// data, ok := event.Data.(types.EventDataTx)
// 		// require.True(ok)
// 		// assert.EqualValues(0, data.Code)
// 		// assert.EqualValues(tx, data.Tx)
// 	case err := <-e:
// 		// this is a failure
// 		require.Nil(err)
// 	}

// }
