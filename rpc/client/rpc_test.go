package client_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	mempl "github.com/tendermint/tendermint/internal/mempool"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/libs/log"
	tmmath "github.com/tendermint/tendermint/libs/math"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/rpc/client"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	rpclocal "github.com/tendermint/tendermint/rpc/client/local"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpcclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
	"github.com/tendermint/tendermint/types"
)

func getHTTPClient(t *testing.T, conf *config.Config) *rpchttp.HTTP {
	t.Helper()

	rpcAddr := conf.RPC.ListenAddress
	c, err := rpchttp.New(rpcAddr)
	require.NoError(t, err)

	c.SetLogger(log.TestingLogger())
	return c
}

func getHTTPClientWithTimeout(t *testing.T, conf *config.Config, timeout time.Duration) *rpchttp.HTTP {
	t.Helper()

	rpcAddr := conf.RPC.ListenAddress
	c, err := rpchttp.NewWithTimeout(rpcAddr, timeout)
	require.NoError(t, err)

	c.SetLogger(log.TestingLogger())

	return c
}

// GetClients returns a slice of clients for table-driven tests
func GetClients(t *testing.T, ns service.Service, conf *config.Config) []client.Client {
	t.Helper()

	node, ok := ns.(rpclocal.NodeService)
	require.True(t, ok)

	ncl, err := rpclocal.New(node)
	require.NoError(t, err)

	return []client.Client{
		getHTTPClient(t, conf),
		ncl,
	}
}

func TestNilCustomHTTPClient(t *testing.T) {
	require.Panics(t, func() {
		_, _ = rpchttp.NewWithClient("http://example.com", nil)
	})
	require.Panics(t, func() {
		_, _ = rpcclient.NewWithHTTPClient("http://example.com", nil)
	})
}

func TestParseInvalidAddress(t *testing.T) {
	_, conf := NodeSuite(t)
	// should remove trailing /
	invalidRemote := conf.RPC.ListenAddress + "/"
	_, err := rpchttp.New(invalidRemote)
	require.NoError(t, err)
}

func TestCustomHTTPClient(t *testing.T) {
	_, conf := NodeSuite(t)
	remote := conf.RPC.ListenAddress
	c, err := rpchttp.NewWithClient(remote, http.DefaultClient)
	require.Nil(t, err)
	status, err := c.Status(context.Background())
	require.NoError(t, err)
	require.NotNil(t, status)
}

func TestCorsEnabled(t *testing.T) {
	_, conf := NodeSuite(t)
	origin := conf.RPC.CORSAllowedOrigins[0]
	remote := strings.ReplaceAll(conf.RPC.ListenAddress, "tcp", "http")

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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n, conf := NodeSuite(t)
	for i, c := range GetClients(t, n, conf) {
		moniker := conf.Moniker
		status, err := c.Status(ctx)
		require.Nil(t, err, "%d: %+v", i, err)
		assert.Equal(t, moniker, status.NodeInfo.Moniker)
	}
}

// Make sure info is correct (we connect properly)
func TestInfo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	n, conf := NodeSuite(t)

	for i, c := range GetClients(t, n, conf) {
		// status, err := c.Status()
		// require.Nil(t, err, "%+v", err)
		info, err := c.ABCIInfo(ctx)
		require.Nil(t, err, "%d: %+v", i, err)
		// TODO: this is not correct - fix merkleeyes!
		// assert.EqualValues(t, status.SyncInfo.LatestBlockHeight, info.Response.LastBlockHeight)
		assert.True(t, strings.Contains(info.Response.Data, "size"))
	}
}

func TestNetInfo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n, conf := NodeSuite(t)
	for i, c := range GetClients(t, n, conf) {
		nc, ok := c.(client.NetworkClient)
		require.True(t, ok, "%d", i)
		netinfo, err := nc.NetInfo(ctx)
		require.Nil(t, err, "%d: %+v", i, err)
		assert.True(t, netinfo.Listening)
		assert.Equal(t, 0, len(netinfo.Peers))
	}
}

func TestDumpConsensusState(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n, conf := NodeSuite(t)
	for i, c := range GetClients(t, n, conf) {
		// FIXME: fix server so it doesn't panic on invalid input
		nc, ok := c.(client.NetworkClient)
		require.True(t, ok, "%d", i)
		cons, err := nc.DumpConsensusState(ctx)
		require.Nil(t, err, "%d: %+v", i, err)
		assert.NotEmpty(t, cons.RoundState)
		assert.Empty(t, cons.Peers)
	}
}

func TestConsensusState(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n, conf := NodeSuite(t)

	for i, c := range GetClients(t, n, conf) {
		// FIXME: fix server so it doesn't panic on invalid input
		nc, ok := c.(client.NetworkClient)
		require.True(t, ok, "%d", i)
		cons, err := nc.ConsensusState(ctx)
		require.Nil(t, err, "%d: %+v", i, err)
		assert.NotEmpty(t, cons.RoundState)
	}
}

func TestHealth(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n, conf := NodeSuite(t)

	for i, c := range GetClients(t, n, conf) {
		nc, ok := c.(client.NetworkClient)
		require.True(t, ok, "%d", i)
		_, err := nc.Health(ctx)
		require.Nil(t, err, "%d: %+v", i, err)
	}
}

func TestGenesisAndValidators(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n, conf := NodeSuite(t)
	for i, c := range GetClients(t, n, conf) {

		// make sure this is the right genesis file
		gen, err := c.Genesis(ctx)
		require.Nil(t, err, "%d: %+v", i, err)
		// get the genesis validator
		require.Equal(t, 1, len(gen.Genesis.Validators))
		gval := gen.Genesis.Validators[0]

		// get the current validators
		h := int64(1)
		vals, err := c.Validators(ctx, &h, nil, nil)
		require.Nil(t, err, "%d: %+v", i, err)
		require.Equal(t, 1, len(vals.Validators))
		require.Equal(t, 1, vals.Count)
		require.Equal(t, 1, vals.Total)
		val := vals.Validators[0]

		// make sure the current set is also the genesis set
		assert.Equal(t, gval.Power, val.VotingPower)
		assert.Equal(t, gval.PubKey, val.PubKey)
	}
}

func TestGenesisChunked(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n, conf := NodeSuite(t)

	for _, c := range GetClients(t, n, conf) {
		first, err := c.GenesisChunked(ctx, 0)
		require.NoError(t, err)

		decoded := make([]string, 0, first.TotalChunks)
		for i := 0; i < first.TotalChunks; i++ {
			chunk, err := c.GenesisChunked(ctx, uint(i))
			require.NoError(t, err)
			data, err := base64.StdEncoding.DecodeString(chunk.Data)
			require.NoError(t, err)
			decoded = append(decoded, string(data))

		}
		doc := []byte(strings.Join(decoded, ""))

		var out types.GenesisDoc
		require.NoError(t, tmjson.Unmarshal(doc, &out),
			"first: %+v, doc: %s", first, string(doc))
	}
}

func TestABCIQuery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n, conf := NodeSuite(t)

	for i, c := range GetClients(t, n, conf) {
		// write something
		k, v, tx := MakeTxKV()
		bres, err := c.BroadcastTxCommit(ctx, tx)
		require.Nil(t, err, "%d: %+v", i, err)
		apph := bres.Height + 1 // this is where the tx will be applied to the state

		// wait before querying
		err = client.WaitForHeight(c, apph, nil)
		require.NoError(t, err)
		res, err := c.ABCIQuery(ctx, "/key", k)
		qres := res.Response
		if assert.Nil(t, err) && assert.True(t, qres.IsOK()) {
			assert.EqualValues(t, v, qres.Value)
		}
	}
}

// Make some app checks
func TestAppCalls(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n, conf := NodeSuite(t)

	for i, c := range GetClients(t, n, conf) {

		// get an offset of height to avoid racing and guessing
		s, err := c.Status(ctx)
		require.NoError(t, err)
		// sh is start height or status height
		sh := s.SyncInfo.LatestBlockHeight

		// look for the future
		h := sh + 20
		_, err = c.Block(ctx, &h)
		require.Error(t, err) // no block yet

		// write something
		k, v, tx := MakeTxKV()
		bres, err := c.BroadcastTxCommit(ctx, tx)
		require.NoError(t, err)
		require.True(t, bres.DeliverTx.IsOK())
		txh := bres.Height
		apph := txh + 1 // this is where the tx will be applied to the state

		// wait before querying
		err = client.WaitForHeight(c, apph, nil)
		require.NoError(t, err)

		_qres, err := c.ABCIQueryWithOptions(ctx, "/key", k, client.ABCIQueryOptions{Prove: false})
		require.NoError(t, err)
		qres := _qres.Response
		if assert.True(t, qres.IsOK()) {
			assert.Equal(t, k, qres.Key)
			assert.EqualValues(t, v, qres.Value)
		}

		// make sure we can lookup the tx with proof
		ptx, err := c.Tx(ctx, bres.Hash, true)
		require.NoError(t, err)
		assert.EqualValues(t, txh, ptx.Height)
		assert.EqualValues(t, tx, ptx.Tx)

		// and we can even check the block is added
		block, err := c.Block(ctx, &apph)
		require.NoError(t, err)
		appHash := block.Block.Header.AppHash
		assert.True(t, len(appHash) > 0)
		assert.EqualValues(t, apph, block.Block.Header.Height)

		blockByHash, err := c.BlockByHash(ctx, block.BlockID.Hash)
		require.NoError(t, err)
		require.Equal(t, block, blockByHash)

		// now check the results
		blockResults, err := c.BlockResults(ctx, &txh)
		require.NoError(t, err, "%d: %+v", i, err)
		assert.Equal(t, txh, blockResults.Height)
		if assert.Equal(t, 1, len(blockResults.TxsResults)) {
			// check success code
			assert.EqualValues(t, 0, blockResults.TxsResults[0].Code)
		}

		// check blockchain info, now that we know there is info
		info, err := c.BlockchainInfo(ctx, apph, apph)
		require.NoError(t, err)
		assert.True(t, info.LastHeight >= apph)
		if assert.Equal(t, 1, len(info.BlockMetas)) {
			lastMeta := info.BlockMetas[0]
			assert.EqualValues(t, apph, lastMeta.Header.Height)
			blockData := block.Block
			assert.Equal(t, blockData.Header.AppHash, lastMeta.Header.AppHash)
			assert.Equal(t, block.BlockID, lastMeta.BlockID)
		}

		// and get the corresponding commit with the same apphash
		commit, err := c.Commit(ctx, &apph)
		require.NoError(t, err)
		cappHash := commit.Header.AppHash
		assert.Equal(t, appHash, cappHash)
		assert.NotNil(t, commit.Commit)

		// compare the commits (note Commit(2) has commit from Block(3))
		h = apph - 1
		commit2, err := c.Commit(ctx, &h)
		require.NoError(t, err)
		assert.Equal(t, block.Block.LastCommitHash, commit2.Commit.Hash())

		// and we got a proof that works!
		_pres, err := c.ABCIQueryWithOptions(ctx, "/key", k, client.ABCIQueryOptions{Prove: true})
		require.NoError(t, err)
		pres := _pres.Response
		assert.True(t, pres.IsOK())

		// XXX Test proof
	}
}

func TestBlockchainInfo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n, conf := NodeSuite(t)

	for i, c := range GetClients(t, n, conf) {
		err := client.WaitForHeight(c, 10, nil)
		require.NoError(t, err)

		res, err := c.BlockchainInfo(ctx, 0, 0)
		require.Nil(t, err, "%d: %+v", i, err)
		assert.True(t, res.LastHeight > 0)
		assert.True(t, len(res.BlockMetas) > 0)

		res, err = c.BlockchainInfo(ctx, 1, 1)
		require.Nil(t, err, "%d: %+v", i, err)
		assert.True(t, res.LastHeight > 0)
		assert.True(t, len(res.BlockMetas) == 1)

		res, err = c.BlockchainInfo(ctx, 1, 10000)
		require.Nil(t, err, "%d: %+v", i, err)
		assert.True(t, res.LastHeight > 0)
		assert.True(t, len(res.BlockMetas) < 100)
		for _, m := range res.BlockMetas {
			assert.NotNil(t, m)
		}

		res, err = c.BlockchainInfo(ctx, 10000, 1)
		require.NotNil(t, err)
		assert.Nil(t, res)
		assert.Contains(t, err.Error(), "can't be greater than max")
	}
}

func TestBroadcastTxSync(t *testing.T) {
	n, conf := NodeSuite(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// TODO (melekes): use mempool which is set on RPC rather than getting it from node
	mempool := getMempool(t, n)
	initMempoolSize := mempool.Size()

	for i, c := range GetClients(t, n, conf) {
		_, _, tx := MakeTxKV()
		bres, err := c.BroadcastTxSync(ctx, tx)
		require.Nil(t, err, "%d: %+v", i, err)
		require.Equal(t, bres.Code, abci.CodeTypeOK) // FIXME

		require.Equal(t, initMempoolSize+1, mempool.Size())

		txs := mempool.ReapMaxTxs(len(tx))
		require.EqualValues(t, tx, txs[0])
		mempool.Flush()
	}
}

func getMempool(t *testing.T, srv service.Service) mempl.Mempool {
	t.Helper()
	n, ok := srv.(interface {
		Mempool() mempl.Mempool
	})
	require.True(t, ok)
	return n.Mempool()
}

func TestBroadcastTxCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n, conf := NodeSuite(t)

	mempool := getMempool(t, n)
	for i, c := range GetClients(t, n, conf) {
		_, _, tx := MakeTxKV()
		bres, err := c.BroadcastTxCommit(ctx, tx)
		require.Nil(t, err, "%d: %+v", i, err)
		require.True(t, bres.CheckTx.IsOK())
		require.True(t, bres.DeliverTx.IsOK())

		require.Equal(t, 0, mempool.Size())
	}
}

func TestUnconfirmedTxs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, _, tx := MakeTxKV()
	ch := make(chan *abci.Response, 1)

	n, conf := NodeSuite(t)
	mempool := getMempool(t, n)
	err := mempool.CheckTx(ctx, tx, func(resp *abci.Response) { ch <- resp }, mempl.TxInfo{})

	require.NoError(t, err)

	// wait for tx to arrive in mempoool.
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Error("Timed out waiting for CheckTx callback")
	}

	for _, c := range GetClients(t, n, conf) {
		mc := c.(client.MempoolClient)
		limit := 1
		res, err := mc.UnconfirmedTxs(ctx, &limit)
		require.NoError(t, err)

		assert.Equal(t, 1, res.Count)
		assert.Equal(t, 1, res.Total)
		assert.Equal(t, mempool.SizeBytes(), res.TotalBytes)
		assert.Exactly(t, types.Txs{tx}, types.Txs(res.Txs))
	}

	mempool.Flush()
}

func TestNumUnconfirmedTxs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, _, tx := MakeTxKV()

	n, conf := NodeSuite(t)
	ch := make(chan *abci.Response, 1)
	mempool := getMempool(t, n)

	err := mempool.CheckTx(ctx, tx, func(resp *abci.Response) { ch <- resp }, mempl.TxInfo{})
	require.NoError(t, err)

	// wait for tx to arrive in mempoool.
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Error("Timed out waiting for CheckTx callback")
	}

	mempoolSize := mempool.Size()
	for i, c := range GetClients(t, n, conf) {
		mc, ok := c.(client.MempoolClient)
		require.True(t, ok, "%d", i)
		res, err := mc.NumUnconfirmedTxs(ctx)
		require.Nil(t, err, "%d: %+v", i, err)

		assert.Equal(t, mempoolSize, res.Count)
		assert.Equal(t, mempoolSize, res.Total)
		assert.Equal(t, mempool.SizeBytes(), res.TotalBytes)
	}

	mempool.Flush()
}

func TestCheckTx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	n, conf := NodeSuite(t)
	mempool := getMempool(t, n)

	for _, c := range GetClients(t, n, conf) {
		_, _, tx := MakeTxKV()

		res, err := c.CheckTx(ctx, tx)
		require.NoError(t, err)
		assert.Equal(t, abci.CodeTypeOK, res.Code)

		assert.Equal(t, 0, mempool.Size(), "mempool must be empty")
	}
}

func TestTx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	n, conf := NodeSuite(t)

	c := getHTTPClient(t, conf)

	// first we broadcast a tx
	_, _, tx := MakeTxKV()
	bres, err := c.BroadcastTxCommit(ctx, tx)
	require.Nil(t, err, "%+v", err)

	txHeight := bres.Height
	txHash := bres.Hash

	anotherTxHash := types.Tx("a different tx").Hash()

	cases := []struct {
		valid bool
		prove bool
		hash  []byte
	}{
		// only valid if correct hash provided
		{true, false, txHash},
		{true, true, txHash},
		{false, false, anotherTxHash},
		{false, true, anotherTxHash},
		{false, false, nil},
		{false, true, nil},
	}

	for i, c := range GetClients(t, n, conf) {
		for j, tc := range cases {
			t.Logf("client %d, case %d", i, j)

			// now we query for the tx.
			// since there's only one tx, we know index=0.
			ptx, err := c.Tx(ctx, tc.hash, tc.prove)

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

func TestTxSearchWithTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, conf := NodeSuite(t)
	timeoutClient := getHTTPClientWithTimeout(t, conf, 10*time.Second)

	_, _, tx := MakeTxKV()
	_, err := timeoutClient.BroadcastTxCommit(ctx, tx)
	require.NoError(t, err)

	// query using a compositeKey (see kvstore application)
	result, err := timeoutClient.TxSearch(ctx, "app.creator='Cosmoshi Netowoko'", false, nil, nil, "asc")
	require.Nil(t, err)
	require.Greater(t, len(result.Txs), 0, "expected a lot of transactions")
}

func TestTxSearch(t *testing.T) {
	n, conf := NodeSuite(t)
	c := getHTTPClient(t, conf)

	// first we broadcast a few txs
	for i := 0; i < 10; i++ {
		_, _, tx := MakeTxKV()
		_, err := c.BroadcastTxCommit(context.Background(), tx)
		require.NoError(t, err)
	}

	// since we're not using an isolated test server, we'll have lingering transactions
	// from other tests as well
	result, err := c.TxSearch(context.Background(), "tx.height >= 0", true, nil, nil, "asc")
	require.NoError(t, err)
	txCount := len(result.Txs)

	// pick out the last tx to have something to search for in tests
	find := result.Txs[len(result.Txs)-1]
	anotherTxHash := types.Tx("a different tx").Hash()

	for i, c := range GetClients(t, n, conf) {
		t.Logf("client %d", i)

		// now we query for the tx.
		result, err := c.TxSearch(context.Background(), fmt.Sprintf("tx.hash='%v'", find.Hash), true, nil, nil, "asc")
		require.Nil(t, err)
		require.Len(t, result.Txs, 1)
		require.Equal(t, find.Hash, result.Txs[0].Hash)

		ptx := result.Txs[0]
		assert.EqualValues(t, find.Height, ptx.Height)
		assert.EqualValues(t, find.Tx, ptx.Tx)
		assert.Zero(t, ptx.Index)
		assert.True(t, ptx.TxResult.IsOK())
		assert.EqualValues(t, find.Hash, ptx.Hash)

		// time to verify the proof
		if assert.EqualValues(t, find.Tx, ptx.Proof.Data) {
			assert.NoError(t, ptx.Proof.Proof.Verify(ptx.Proof.RootHash, find.Hash))
		}

		// query by height
		result, err = c.TxSearch(context.Background(), fmt.Sprintf("tx.height=%d", find.Height), true, nil, nil, "asc")
		require.Nil(t, err)
		require.Len(t, result.Txs, 1)

		// query for non existing tx
		result, err = c.TxSearch(context.Background(), fmt.Sprintf("tx.hash='%X'", anotherTxHash), false, nil, nil, "asc")
		require.Nil(t, err)
		require.Len(t, result.Txs, 0)

		// query using a compositeKey (see kvstore application)
		result, err = c.TxSearch(context.Background(), "app.creator='Cosmoshi Netowoko'", false, nil, nil, "asc")
		require.Nil(t, err)
		require.Greater(t, len(result.Txs), 0, "expected a lot of transactions")

		// query using an index key
		result, err = c.TxSearch(context.Background(), "app.index_key='index is working'", false, nil, nil, "asc")
		require.Nil(t, err)
		require.Greater(t, len(result.Txs), 0, "expected a lot of transactions")

		// query using an noindex key
		result, err = c.TxSearch(context.Background(), "app.noindex_key='index is working'", false, nil, nil, "asc")
		require.Nil(t, err)
		require.Equal(t, len(result.Txs), 0, "expected a lot of transactions")

		// query using a compositeKey (see kvstore application) and height
		result, err = c.TxSearch(context.Background(),
			"app.creator='Cosmoshi Netowoko' AND tx.height<10000", true, nil, nil, "asc")
		require.Nil(t, err)
		require.Greater(t, len(result.Txs), 0, "expected a lot of transactions")

		// query a non existing tx with page 1 and txsPerPage 1
		perPage := 1
		result, err = c.TxSearch(context.Background(), "app.creator='Cosmoshi Neetowoko'", true, nil, &perPage, "asc")
		require.Nil(t, err)
		require.Len(t, result.Txs, 0)

		// check sorting
		result, err = c.TxSearch(context.Background(), "tx.height >= 1", false, nil, nil, "asc")
		require.Nil(t, err)
		for k := 0; k < len(result.Txs)-1; k++ {
			require.LessOrEqual(t, result.Txs[k].Height, result.Txs[k+1].Height)
			require.LessOrEqual(t, result.Txs[k].Index, result.Txs[k+1].Index)
		}

		result, err = c.TxSearch(context.Background(), "tx.height >= 1", false, nil, nil, "desc")
		require.Nil(t, err)
		for k := 0; k < len(result.Txs)-1; k++ {
			require.GreaterOrEqual(t, result.Txs[k].Height, result.Txs[k+1].Height)
			require.GreaterOrEqual(t, result.Txs[k].Index, result.Txs[k+1].Index)
		}
		// check pagination
		perPage = 3
		var (
			seen      = map[int64]bool{}
			maxHeight int64
			pages     = int(math.Ceil(float64(txCount) / float64(perPage)))
		)

		for page := 1; page <= pages; page++ {
			page := page
			result, err := c.TxSearch(context.Background(), "tx.height >= 1", false, &page, &perPage, "asc")
			require.NoError(t, err)
			if page < pages {
				require.Len(t, result.Txs, perPage)
			} else {
				require.LessOrEqual(t, len(result.Txs), perPage)
			}
			require.Equal(t, txCount, result.TotalCount)
			for _, tx := range result.Txs {
				require.False(t, seen[tx.Height],
					"Found duplicate height %v in page %v", tx.Height, page)
				require.Greater(t, tx.Height, maxHeight,
					"Found decreasing height %v (max seen %v) in page %v", tx.Height, maxHeight, page)
				seen[tx.Height] = true
				maxHeight = tx.Height
			}
		}
		require.Len(t, seen, txCount)
	}
}

func TestBatchedJSONRPCCalls(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, conf := NodeSuite(t)
	c := getHTTPClient(t, conf)
	testBatchedJSONRPCCalls(ctx, t, c)
}

func testBatchedJSONRPCCalls(ctx context.Context, t *testing.T, c *rpchttp.HTTP) {
	k1, v1, tx1 := MakeTxKV()
	k2, v2, tx2 := MakeTxKV()

	batch := c.NewBatch()
	r1, err := batch.BroadcastTxCommit(ctx, tx1)
	require.NoError(t, err)
	r2, err := batch.BroadcastTxCommit(ctx, tx2)
	require.NoError(t, err)
	require.Equal(t, 2, batch.Count())
	bresults, err := batch.Send(ctx)
	require.NoError(t, err)
	require.Len(t, bresults, 2)
	require.Equal(t, 0, batch.Count())

	bresult1, ok := bresults[0].(*ctypes.ResultBroadcastTxCommit)
	require.True(t, ok)
	require.Equal(t, *bresult1, *r1)
	bresult2, ok := bresults[1].(*ctypes.ResultBroadcastTxCommit)
	require.True(t, ok)
	require.Equal(t, *bresult2, *r2)
	apph := tmmath.MaxInt64(bresult1.Height, bresult2.Height) + 1

	err = client.WaitForHeight(c, apph, nil)
	require.NoError(t, err)

	q1, err := batch.ABCIQuery(ctx, "/key", k1)
	require.NoError(t, err)
	q2, err := batch.ABCIQuery(ctx, "/key", k2)
	require.NoError(t, err)
	require.Equal(t, 2, batch.Count())
	qresults, err := batch.Send(ctx)
	require.NoError(t, err)
	require.Len(t, qresults, 2)
	require.Equal(t, 0, batch.Count())

	qresult1, ok := qresults[0].(*ctypes.ResultABCIQuery)
	require.True(t, ok)
	require.Equal(t, *qresult1, *q1)
	qresult2, ok := qresults[1].(*ctypes.ResultABCIQuery)
	require.True(t, ok)
	require.Equal(t, *qresult2, *q2)

	require.Equal(t, qresult1.Response.Key, k1)
	require.Equal(t, qresult2.Response.Key, k2)
	require.Equal(t, qresult1.Response.Value, v1)
	require.Equal(t, qresult2.Response.Value, v2)
}

func TestBatchedJSONRPCCallsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, conf := NodeSuite(t)
	c := getHTTPClient(t, conf)
	_, _, tx1 := MakeTxKV()
	_, _, tx2 := MakeTxKV()

	batch := c.NewBatch()
	_, err := batch.BroadcastTxCommit(ctx, tx1)
	require.NoError(t, err)
	_, err = batch.BroadcastTxCommit(ctx, tx2)
	require.NoError(t, err)
	// we should have 2 requests waiting
	require.Equal(t, 2, batch.Count())
	// we want to make sure we cleared 2 pending requests
	require.Equal(t, 2, batch.Clear())
	// now there should be no batched requests
	require.Equal(t, 0, batch.Count())
}

func TestSendingEmptyRequestBatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, conf := NodeSuite(t)
	c := getHTTPClient(t, conf)
	batch := c.NewBatch()
	_, err := batch.Send(ctx)
	require.Error(t, err, "sending an empty batch of JSON RPC requests should result in an error")
}

func TestClearingEmptyRequestBatch(t *testing.T) {
	_, conf := NodeSuite(t)
	c := getHTTPClient(t, conf)
	batch := c.NewBatch()
	require.Zero(t, batch.Clear(), "clearing an empty batch of JSON RPC requests should result in a 0 result")
}

func TestConcurrentJSONRPCBatching(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, conf := NodeSuite(t)
	var wg sync.WaitGroup
	c := getHTTPClient(t, conf)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			testBatchedJSONRPCCalls(ctx, t, c)
		}()
	}
	wg.Wait()
}
