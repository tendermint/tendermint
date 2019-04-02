package client_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"

	"github.com/tendermint/tendermint/rpc/client"
	rpctest "github.com/tendermint/tendermint/rpc/test"
	"github.com/tendermint/tendermint/types"
)

func getHTTPClient() *client.HTTP {
	rpcAddr := rpctest.GetConfig().RPC.ListenAddress
	return client.NewHTTP(rpcAddr, "/websocket")
}

func getLocalClient() *client.Local {
	return client.NewLocal(node)
}

// GetClients returns a slice of clients for table-driven tests
func GetClients() []client.Client {
	return []client.Client{
		getHTTPClient(),
		getLocalClient(),
	}
}

func TestCorsEnabled(t *testing.T) {
	origin := rpctest.GetConfig().RPC.CORSAllowedOrigins[0]
	remote := strings.Replace(rpctest.GetConfig().RPC.ListenAddress, "tcp", "http", -1)

	req, err := http.NewRequest("GET", remote, nil)
	require.Nil(t, err, "%+v", err)
	req.Header.Set("Origin", origin)
	c := &http.Client{}
	resp, err := c.Do(req)
	require.Nil(t, err, "%+v", err)
	defer resp.Body.Close()

	assert.Equal(t, resp.Header.Get("Access-Control-Allow-Origin"), origin)
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
		// assert.EqualValues(t, status.SyncInfo.LatestBlockHeight, info.Response.LastBlockHeight)
		assert.True(t, strings.Contains(info.Response.Data, "size"))
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
		assert.Empty(t, cons.Peers)
	}
}

func TestConsensusState(t *testing.T) {
	for i, c := range GetClients() {
		// FIXME: fix server so it doesn't panic on invalid input
		nc, ok := c.(client.NetworkClient)
		require.True(t, ok, "%d", i)
		cons, err := nc.ConsensusState()
		require.Nil(t, err, "%d: %+v", i, err)
		assert.NotEmpty(t, cons.RoundState)
	}
}

func TestHealth(t *testing.T) {
	for i, c := range GetClients() {
		nc, ok := c.(client.NetworkClient)
		require.True(t, ok, "%d", i)
		_, err := nc.Health()
		require.Nil(t, err, "%d: %+v", i, err)
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
		vals, err := c.Validators(nil)
		require.Nil(t, err, "%d: %+v", i, err)
		require.Equal(t, 1, len(vals.Validators))
		val := vals.Validators[0]

		// make sure the current set is also the genesis set
		assert.Equal(t, gval.Power, val.VotingPower)
		assert.Equal(t, gval.PubKey, val.PubKey)
	}
}

func TestABCIQuery(t *testing.T) {
	for i, c := range GetClients() {
		// write something
		k, v, tx := MakeTxKV()
		bres, err := c.BroadcastTxCommit(tx)
		require.Nil(t, err, "%d: %+v", i, err)
		apph := bres.Height + 1 // this is where the tx will be applied to the state

		// wait before querying
		client.WaitForHeight(c, apph, nil)
		res, err := c.ABCIQuery("/key", k)
		qres := res.Response
		if assert.Nil(t, err) && assert.True(t, qres.IsOK()) {
			assert.EqualValues(t, v, qres.Value)
		}
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
		sh := s.SyncInfo.LatestBlockHeight

		// look for the future
		h := sh + 2
		_, err = c.Block(&h)
		assert.NotNil(err) // no block yet

		// write something
		k, v, tx := MakeTxKV()
		bres, err := c.BroadcastTxCommit(tx)
		require.Nil(err, "%d: %+v", i, err)
		require.True(bres.DeliverTx.IsOK())
		txh := bres.Height
		apph := txh + 1 // this is where the tx will be applied to the state

		// wait before querying
		if err := client.WaitForHeight(c, apph, nil); err != nil {
			t.Error(err)
		}
		_qres, err := c.ABCIQueryWithOptions("/key", k, client.ABCIQueryOptions{Prove: false})
		qres := _qres.Response
		if assert.Nil(err) && assert.True(qres.IsOK()) {
			assert.Equal(k, qres.Key)
			assert.EqualValues(v, qres.Value)
		}

		// make sure we can lookup the tx with proof
		ptx, err := c.Tx(bres.Hash, true)
		require.Nil(err, "%d: %+v", i, err)
		assert.EqualValues(txh, ptx.Height)
		assert.EqualValues(tx, ptx.Tx)

		// and we can even check the block is added
		block, err := c.Block(&apph)
		require.Nil(err, "%d: %+v", i, err)
		appHash := block.BlockMeta.Header.AppHash
		assert.True(len(appHash) > 0)
		assert.EqualValues(apph, block.BlockMeta.Header.Height)

		// now check the results
		blockResults, err := c.BlockResults(&txh)
		require.Nil(err, "%d: %+v", i, err)
		assert.Equal(txh, blockResults.Height)
		if assert.Equal(1, len(blockResults.Results.DeliverTx)) {
			// check success code
			assert.EqualValues(0, blockResults.Results.DeliverTx[0].Code)
		}

		// check blockchain info, now that we know there is info
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
		commit, err := c.Commit(&apph)
		require.Nil(err, "%d: %+v", i, err)
		cappHash := commit.Header.AppHash
		assert.Equal(appHash, cappHash)
		assert.NotNil(commit.Commit)

		// compare the commits (note Commit(2) has commit from Block(3))
		h = apph - 1
		commit2, err := c.Commit(&h)
		require.Nil(err, "%d: %+v", i, err)
		assert.Equal(block.Block.LastCommit, commit2.Commit)

		// and we got a proof that works!
		_pres, err := c.ABCIQueryWithOptions("/key", k, client.ABCIQueryOptions{Prove: true})
		pres := _pres.Response
		assert.Nil(err)
		assert.True(pres.IsOK())

		// XXX Test proof
	}
}

func TestBroadcastTxSync(t *testing.T) {
	require := require.New(t)

	// TODO (melekes): use mempool which is set on RPC rather than getting it from node
	mempool := node.Mempool()
	initMempoolSize := mempool.Size()

	for i, c := range GetClients() {
		_, _, tx := MakeTxKV()
		bres, err := c.BroadcastTxSync(tx)
		require.Nil(err, "%d: %+v", i, err)
		require.Equal(bres.Code, abci.CodeTypeOK) // FIXME

		require.Equal(initMempoolSize+1, mempool.Size())

		txs := mempool.ReapMaxTxs(len(tx))
		require.EqualValues(tx, txs[0])
		mempool.Flush()
	}
}

func TestBroadcastTxCommit(t *testing.T) {
	require := require.New(t)

	mempool := node.Mempool()
	for i, c := range GetClients() {
		_, _, tx := MakeTxKV()
		bres, err := c.BroadcastTxCommit(tx)
		require.Nil(err, "%d: %+v", i, err)
		require.True(bres.CheckTx.IsOK())
		require.True(bres.DeliverTx.IsOK())

		require.Equal(0, mempool.Size())
	}
}

func TestUnconfirmedTxs(t *testing.T) {
	_, _, tx := MakeTxKV()

	mempool := node.Mempool()
	_ = mempool.CheckTx(tx, nil)

	for i, c := range GetClients() {
		mc, ok := c.(client.MempoolClient)
		require.True(t, ok, "%d", i)
		res, err := mc.UnconfirmedTxs(1)
		require.Nil(t, err, "%d: %+v", i, err)

		assert.Equal(t, 1, res.Count)
		assert.Equal(t, 1, res.Total)
		assert.Equal(t, mempool.TxsBytes(), res.TotalBytes)
		assert.Exactly(t, types.Txs{tx}, types.Txs(res.Txs))
	}

	mempool.Flush()
}

func TestNumUnconfirmedTxs(t *testing.T) {
	_, _, tx := MakeTxKV()

	mempool := node.Mempool()
	_ = mempool.CheckTx(tx, nil)
	mempoolSize := mempool.Size()

	for i, c := range GetClients() {
		mc, ok := c.(client.MempoolClient)
		require.True(t, ok, "%d", i)
		res, err := mc.NumUnconfirmedTxs()
		require.Nil(t, err, "%d: %+v", i, err)

		assert.Equal(t, mempoolSize, res.Count)
		assert.Equal(t, mempoolSize, res.Total)
		assert.Equal(t, mempool.TxsBytes(), res.TotalBytes)
	}

	mempool.Flush()
}

func TestTx(t *testing.T) {
	// first we broadcast a tx
	c := getHTTPClient()
	_, _, tx := MakeTxKV()
	bres, err := c.BroadcastTxCommit(tx)
	require.Nil(t, err, "%+v", err)

	txHeight := bres.Height
	txHash := bres.Hash

	anotherTxHash := types.Tx("a different tx").Hash()

	cases := []struct {
		valid bool
		hash  []byte
		prove bool
	}{
		// only valid if correct hash provided
		{true, txHash, false},
		{true, txHash, true},
		{false, anotherTxHash, false},
		{false, anotherTxHash, true},
		{false, nil, false},
		{false, nil, true},
	}

	for i, c := range GetClients() {
		for j, tc := range cases {
			t.Logf("client %d, case %d", i, j)

			// now we query for the tx.
			// since there's only one tx, we know index=0.
			ptx, err := c.Tx(tc.hash, tc.prove)

			if !tc.valid {
				require.NotNil(t, err)
			} else {
				require.Nil(t, err, "%+v", err)
				assert.EqualValues(t, txHeight, ptx.Height)
				assert.EqualValues(t, tx, ptx.Tx)
				assert.Zero(t, ptx.Index)
				assert.True(t, ptx.TxResult.IsOK())
				assert.EqualValues(t, txHash, ptx.Hash)

				// time to verify the proof
				proof := ptx.Proof
				if tc.prove && assert.EqualValues(t, tx, proof.Data) {
					assert.NoError(t, proof.Proof.Verify(proof.RootHash, txHash))
				}
			}
		}
	}
}

func TestTxSearch(t *testing.T) {
	// first we broadcast a tx
	c := getHTTPClient()
	_, _, tx := MakeTxKV()
	bres, err := c.BroadcastTxCommit(tx)
	require.Nil(t, err, "%+v", err)

	txHeight := bres.Height
	txHash := bres.Hash

	anotherTxHash := types.Tx("a different tx").Hash()

	for i, c := range GetClients() {
		t.Logf("client %d", i)

		// now we query for the tx.
		// since there's only one tx, we know index=0.
		result, err := c.TxSearch(fmt.Sprintf("tx.hash='%v'", txHash), true, 1, 30)
		require.Nil(t, err, "%+v", err)
		require.Len(t, result.Txs, 1)

		ptx := result.Txs[0]
		assert.EqualValues(t, txHeight, ptx.Height)
		assert.EqualValues(t, tx, ptx.Tx)
		assert.Zero(t, ptx.Index)
		assert.True(t, ptx.TxResult.IsOK())
		assert.EqualValues(t, txHash, ptx.Hash)

		// time to verify the proof
		proof := ptx.Proof
		if assert.EqualValues(t, tx, proof.Data) {
			assert.NoError(t, proof.Proof.Verify(proof.RootHash, txHash))
		}

		// query by height
		result, err = c.TxSearch(fmt.Sprintf("tx.height=%d", txHeight), true, 1, 30)
		require.Nil(t, err, "%+v", err)
		require.Len(t, result.Txs, 1)

		// query for non existing tx
		result, err = c.TxSearch(fmt.Sprintf("tx.hash='%X'", anotherTxHash), false, 1, 30)
		require.Nil(t, err, "%+v", err)
		require.Len(t, result.Txs, 0)

		// query using a tag (see kvstore application)
		result, err = c.TxSearch("app.creator='Cosmoshi Netowoko'", false, 1, 30)
		require.Nil(t, err, "%+v", err)
		if len(result.Txs) == 0 {
			t.Fatal("expected a lot of transactions")
		}

		// query using a tag (see kvstore application) and height
		result, err = c.TxSearch("app.creator='Cosmoshi Netowoko' AND tx.height<10000", true, 1, 30)
		require.Nil(t, err, "%+v", err)
		if len(result.Txs) == 0 {
			t.Fatal("expected a lot of transactions")
		}

		// query a non existing tx with page 1 and txsPerPage 1
		result, err = c.TxSearch("app.creator='Cosmoshi Neetowoko'", true, 1, 1)
		require.Nil(t, err, "%+v", err)
		require.Len(t, result.Txs, 0)
	}
}
