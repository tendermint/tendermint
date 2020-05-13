package client_test

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/libs/log"
	tmmath "github.com/tendermint/tendermint/libs/math"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/rpc/client"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	rpclocal "github.com/tendermint/tendermint/rpc/client/local"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpcclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
	rpctest "github.com/tendermint/tendermint/rpc/test"
	"github.com/tendermint/tendermint/types"
)

func getHTTPClient() *rpchttp.HTTP {
	rpcAddr := rpctest.GetConfig().RPC.ListenAddress
	c, err := rpchttp.New(rpcAddr, "/websocket")
	if err != nil {
		panic(err)
	}
	c.SetLogger(log.TestingLogger())
	return c
}

func getHTTPClientWithTimeout(timeout uint) *rpchttp.HTTP {
	rpcAddr := rpctest.GetConfig().RPC.ListenAddress
	c, err := rpchttp.NewWithTimeout(rpcAddr, "/websocket", timeout)
	if err != nil {
		panic(err)
	}
	c.SetLogger(log.TestingLogger())
	return c
}

func getLocalClient() *rpclocal.Local {
	return rpclocal.New(node)
}

// GetClients returns a slice of clients for table-driven tests
func GetClients() []client.Client {
	return []client.Client{
		getHTTPClient(),
		getLocalClient(),
	}
}

func TestNilCustomHTTPClient(t *testing.T) {
	require.Panics(t, func() {
		_, _ = rpchttp.NewWithClient("http://example.com", "/websocket", nil)
	})
	require.Panics(t, func() {
		_, _ = rpcclient.NewWithHTTPClient("http://example.com", nil)
	})
}

func TestCustomHTTPClient(t *testing.T) {
	remote := rpctest.GetConfig().RPC.ListenAddress
	c, err := rpchttp.NewWithClient(remote, "/websocket", http.DefaultClient)
	require.Nil(t, err)
	status, err := c.Status()
	require.NoError(t, err)
	require.NotNil(t, status)
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
		vals, err := c.Validators(nil, 0, 0)
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
		appHash := block.Block.Header.AppHash
		assert.True(len(appHash) > 0)
		assert.EqualValues(apph, block.Block.Header.Height)

		// now check the results
		blockResults, err := c.BlockResults(&txh)
		require.Nil(err, "%d: %+v", i, err)
		assert.Equal(txh, blockResults.Height)
		if assert.Equal(1, len(blockResults.TxsResults)) {
			// check success code
			assert.EqualValues(0, blockResults.TxsResults[0].Code)
		}

		// check blockchain info, now that we know there is info
		info, err := c.BlockchainInfo(apph, apph)
		require.Nil(err, "%d: %+v", i, err)
		assert.True(info.LastHeight >= apph)
		if assert.Equal(1, len(info.BlockMetas)) {
			lastMeta := info.BlockMetas[0]
			assert.EqualValues(apph, lastMeta.Header.Height)
			blockData := block.Block
			assert.Equal(blockData.Header.AppHash, lastMeta.Header.AppHash)
			assert.Equal(block.BlockID, lastMeta.BlockID)
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
	_ = mempool.CheckTx(tx, nil, mempl.TxInfo{})

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
	_ = mempool.CheckTx(tx, nil, mempl.TxInfo{})
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

func TestTxSearchWithTimeout(t *testing.T) {
	// Get a client with a time-out of 10 secs.
	timeoutClient := getHTTPClientWithTimeout(10)

	// query using a compositeKey (see kvstore application)
	result, err := timeoutClient.TxSearch("app.creator='Cosmoshi Netowoko'", false, 1, 30, "asc")
	require.Nil(t, err)
	if len(result.Txs) == 0 {
		t.Fatal("expected a lot of transactions")
	}
}

func TestTxSearch(t *testing.T) {
	c := getHTTPClient()

	// first we broadcast a few txs
	for i := 0; i < 10; i++ {
		_, _, tx := MakeTxKV()
		_, err := c.BroadcastTxCommit(tx)
		require.NoError(t, err)
	}

	// since we're not using an isolated test server, we'll have lingering transactions
	// from other tests as well
	result, err := c.TxSearch("tx.height >= 0", true, 1, 100, "asc")
	require.NoError(t, err)
	txCount := len(result.Txs)

	// pick out the last tx to have something to search for in tests
	find := result.Txs[len(result.Txs)-1]
	anotherTxHash := types.Tx("a different tx").Hash()

	for i, c := range GetClients() {
		t.Logf("client %d", i)

		// now we query for the tx.
		result, err := c.TxSearch(fmt.Sprintf("tx.hash='%v'", find.Hash), true, 1, 30, "asc")
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
		result, err = c.TxSearch(fmt.Sprintf("tx.height=%d", find.Height), true, 1, 30, "asc")
		require.Nil(t, err)
		require.Len(t, result.Txs, 1)

		// query for non existing tx
		result, err = c.TxSearch(fmt.Sprintf("tx.hash='%X'", anotherTxHash), false, 1, 30, "asc")
		require.Nil(t, err)
		require.Len(t, result.Txs, 0)

		// query using a compositeKey (see kvstore application)
		result, err = c.TxSearch("app.creator='Cosmoshi Netowoko'", false, 1, 30, "asc")
		require.Nil(t, err)
		if len(result.Txs) == 0 {
			t.Fatal("expected a lot of transactions")
		}

		// query using a compositeKey (see kvstore application) and height
		result, err = c.TxSearch("app.creator='Cosmoshi Netowoko' AND tx.height<10000", true, 1, 30, "asc")
		require.Nil(t, err)
		if len(result.Txs) == 0 {
			t.Fatal("expected a lot of transactions")
		}

		// query a non existing tx with page 1 and txsPerPage 1
		result, err = c.TxSearch("app.creator='Cosmoshi Neetowoko'", true, 1, 1, "asc")
		require.Nil(t, err)
		require.Len(t, result.Txs, 0)

		// check sorting
		result, err = c.TxSearch(fmt.Sprintf("tx.height >= 1"), false, 1, 30, "asc")
		require.Nil(t, err)
		for k := 0; k < len(result.Txs)-1; k++ {
			require.LessOrEqual(t, result.Txs[k].Height, result.Txs[k+1].Height)
			require.LessOrEqual(t, result.Txs[k].Index, result.Txs[k+1].Index)
		}

		result, err = c.TxSearch(fmt.Sprintf("tx.height >= 1"), false, 1, 30, "desc")
		require.Nil(t, err)
		for k := 0; k < len(result.Txs)-1; k++ {
			require.GreaterOrEqual(t, result.Txs[k].Height, result.Txs[k+1].Height)
			require.GreaterOrEqual(t, result.Txs[k].Index, result.Txs[k+1].Index)
		}

		// check pagination
		var (
			seen      = map[int64]bool{}
			maxHeight int64
			perPage   = 3
			pages     = int(math.Ceil(float64(txCount) / float64(perPage)))
		)
		for page := 1; page <= pages; page++ {
			result, err = c.TxSearch("tx.height >= 1", false, page, perPage, "asc")
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

func deepcpVote(vote *types.Vote) (res *types.Vote) {
	res = &types.Vote{
		ValidatorAddress: make([]byte, len(vote.ValidatorAddress)),
		ValidatorIndex:   vote.ValidatorIndex,
		Height:           vote.Height,
		Round:            vote.Round,
		Type:             vote.Type,
		Timestamp:        vote.Timestamp,
		BlockID: types.BlockID{
			Hash:        make([]byte, len(vote.BlockID.Hash)),
			PartsHeader: vote.BlockID.PartsHeader,
		},
		Signature: make([]byte, len(vote.Signature)),
	}
	copy(res.ValidatorAddress, vote.ValidatorAddress)
	copy(res.BlockID.Hash, vote.BlockID.Hash)
	copy(res.Signature, vote.Signature)
	return
}

func newEvidence(
	t *testing.T,
	val *privval.FilePV,
	vote *types.Vote,
	vote2 *types.Vote,
	chainID string,
) types.DuplicateVoteEvidence {
	var err error
	deepcpVote2 := deepcpVote(vote2)
	deepcpVote2.Signature, err = val.Key.PrivKey.Sign(deepcpVote2.SignBytes(chainID))
	require.NoError(t, err)

	return *types.NewDuplicateVoteEvidence(val.Key.PubKey, vote, deepcpVote2)
}

func makeEvidences(
	t *testing.T,
	val *privval.FilePV,
	chainID string,
) (ev types.DuplicateVoteEvidence, fakes []types.DuplicateVoteEvidence) {
	vote := &types.Vote{
		ValidatorAddress: val.Key.Address,
		ValidatorIndex:   0,
		Height:           1,
		Round:            0,
		Type:             types.PrevoteType,
		Timestamp:        time.Now().UTC(),
		BlockID: types.BlockID{
			Hash: tmhash.Sum([]byte("blockhash")),
			PartsHeader: types.PartSetHeader{
				Total: 1000,
				Hash:  tmhash.Sum([]byte("partset")),
			},
		},
	}

	var err error
	vote.Signature, err = val.Key.PrivKey.Sign(vote.SignBytes(chainID))
	require.NoError(t, err)

	vote2 := deepcpVote(vote)
	vote2.BlockID.Hash = tmhash.Sum([]byte("blockhash2"))

	ev = newEvidence(t, val, vote, vote2, chainID)

	fakes = make([]types.DuplicateVoteEvidence, 42)

	// different address
	vote2 = deepcpVote(vote)
	for i := 0; i < 10; i++ {
		rand.Read(vote2.ValidatorAddress) // nolint: gosec
		fakes[i] = newEvidence(t, val, vote, vote2, chainID)
	}
	// different index
	vote2 = deepcpVote(vote)
	for i := 10; i < 20; i++ {
		vote2.ValidatorIndex = rand.Int()%100 + 1 // nolint: gosec
		fakes[i] = newEvidence(t, val, vote, vote2, chainID)
	}
	// different height
	vote2 = deepcpVote(vote)
	for i := 20; i < 30; i++ {
		vote2.Height = rand.Int63()%1000 + 100 // nolint: gosec
		fakes[i] = newEvidence(t, val, vote, vote2, chainID)
	}
	// different round
	vote2 = deepcpVote(vote)
	for i := 30; i < 40; i++ {
		vote2.Round = rand.Int()%10 + 1 // nolint: gosec
		fakes[i] = newEvidence(t, val, vote, vote2, chainID)
	}
	// different type
	vote2 = deepcpVote(vote)
	vote2.Type = types.PrecommitType
	fakes[40] = newEvidence(t, val, vote, vote2, chainID)
	// exactly same vote
	vote2 = deepcpVote(vote)
	fakes[41] = newEvidence(t, val, vote, vote2, chainID)
	return ev, fakes
}

func TestBroadcastEvidenceDuplicateVote(t *testing.T) {
	config := rpctest.GetConfig()
	chainID := config.ChainID()
	pvKeyFile := config.PrivValidatorKeyFile()
	pvKeyStateFile := config.PrivValidatorStateFile()
	pv := privval.LoadOrGenFilePV(pvKeyFile, pvKeyStateFile)

	ev, fakes := makeEvidences(t, pv, chainID)
	t.Logf("evidence %v", ev)

	for i, c := range GetClients() {
		t.Logf("client %d", i)

		result, err := c.BroadcastEvidence(&ev)
		require.Nil(t, err)
		require.Equal(t, ev.Hash(), result.Hash, "Invalid response, result %+v", result)

		status, err := c.Status()
		require.NoError(t, err)
		client.WaitForHeight(c, status.SyncInfo.LatestBlockHeight+2, nil)

		ed25519pub := ev.PubKey.(ed25519.PubKeyEd25519)
		rawpub := ed25519pub[:]
		result2, err := c.ABCIQuery("/val", rawpub)
		require.Nil(t, err, "Error querying evidence, err %v", err)
		qres := result2.Response
		require.True(t, qres.IsOK(), "Response not OK")

		var v abci.ValidatorUpdate
		err = abci.ReadMessage(bytes.NewReader(qres.Value), &v)
		require.NoError(t, err, "Error reading query result, value %v", qres.Value)

		require.EqualValues(t, rawpub, v.PubKey.Data, "Stored PubKey not equal with expected, value %v", string(qres.Value))
		require.Equal(t, int64(9), v.Power, "Stored Power not equal with expected, value %v", string(qres.Value))

		for _, fake := range fakes {
			_, err := c.BroadcastEvidence(&types.DuplicateVoteEvidence{
				PubKey: fake.PubKey,
				VoteA:  fake.VoteA,
				VoteB:  fake.VoteB})
			require.Error(t, err, "Broadcasting fake evidence succeed: %s", fake.String())
		}
	}
}

func TestBatchedJSONRPCCalls(t *testing.T) {
	c := getHTTPClient()
	testBatchedJSONRPCCalls(t, c)
}

func testBatchedJSONRPCCalls(t *testing.T, c *rpchttp.HTTP) {
	k1, v1, tx1 := MakeTxKV()
	k2, v2, tx2 := MakeTxKV()

	batch := c.NewBatch()
	r1, err := batch.BroadcastTxCommit(tx1)
	require.NoError(t, err)
	r2, err := batch.BroadcastTxCommit(tx2)
	require.NoError(t, err)
	require.Equal(t, 2, batch.Count())
	bresults, err := batch.Send()
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

	client.WaitForHeight(c, apph, nil)

	q1, err := batch.ABCIQuery("/key", k1)
	require.NoError(t, err)
	q2, err := batch.ABCIQuery("/key", k2)
	require.NoError(t, err)
	require.Equal(t, 2, batch.Count())
	qresults, err := batch.Send()
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
	c := getHTTPClient()
	_, _, tx1 := MakeTxKV()
	_, _, tx2 := MakeTxKV()

	batch := c.NewBatch()
	_, err := batch.BroadcastTxCommit(tx1)
	require.NoError(t, err)
	_, err = batch.BroadcastTxCommit(tx2)
	require.NoError(t, err)
	// we should have 2 requests waiting
	require.Equal(t, 2, batch.Count())
	// we want to make sure we cleared 2 pending requests
	require.Equal(t, 2, batch.Clear())
	// now there should be no batched requests
	require.Equal(t, 0, batch.Count())
}

func TestSendingEmptyRequestBatch(t *testing.T) {
	c := getHTTPClient()
	batch := c.NewBatch()
	_, err := batch.Send()
	require.Error(t, err, "sending an empty batch of JSON RPC requests should result in an error")
}

func TestClearingEmptyRequestBatch(t *testing.T) {
	c := getHTTPClient()
	batch := c.NewBatch()
	require.Zero(t, batch.Clear(), "clearing an empty batch of JSON RPC requests should result in a 0 result")
}

func TestConcurrentJSONRPCBatching(t *testing.T) {
	var wg sync.WaitGroup
	c := getHTTPClient()
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			testBatchedJSONRPCCalls(t, c)
		}()
	}
	wg.Wait()
}
