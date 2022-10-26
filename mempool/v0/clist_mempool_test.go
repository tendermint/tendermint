package v0

import (
	"context"
	"encoding/binary"
	"fmt"
	mrand "math/rand"
	"os"
	"testing"
	"time"

	"github.com/cosmos/gogoproto/proto"
	gogotypes "github.com/cosmos/gogoproto/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	abciclient "github.com/tendermint/tendermint/abci/client"
	abciclimocks "github.com/tendermint/tendermint/abci/client/mocks"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abciserver "github.com/tendermint/tendermint/abci/server"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/test"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

// A cleanupFunc cleans up any config / test files created for a particular
// test.
type cleanupFunc func()

func newMempoolWithAppMock(cc proxy.ClientCreator, client abciclient.Client) (*CListMempool, cleanupFunc, error) {
	conf := test.ResetTestRoot("mempool_test")

	mp, cu := newMempoolWithAppAndConfigMock(cc, conf, client)
	return mp, cu, nil
}

func newMempoolWithAppAndConfigMock(cc proxy.ClientCreator,
	cfg *config.Config,
	client abciclient.Client) (*CListMempool, cleanupFunc) {
	appConnMem := client
	appConnMem.SetLogger(log.TestingLogger().With("module", "abci-client", "connection", "mempool"))
	err := appConnMem.Start()
	if err != nil {
		panic(err)
	}

	mp := NewCListMempool(cfg.Mempool, appConnMem, 0)
	mp.SetLogger(log.TestingLogger())

	return mp, func() { os.RemoveAll(cfg.RootDir) }
}

func newMempoolWithApp(cc proxy.ClientCreator) (*CListMempool, cleanupFunc) {
	conf := test.ResetTestRoot("mempool_test")

	mp, cu := newMempoolWithAppAndConfig(cc, conf)
	return mp, cu
}

func newMempoolWithAppAndConfig(cc proxy.ClientCreator, cfg *config.Config) (*CListMempool, cleanupFunc) {
	appConnMem, _ := cc.NewABCIClient()
	appConnMem.SetLogger(log.TestingLogger().With("module", "abci-client", "connection", "mempool"))
	err := appConnMem.Start()
	if err != nil {
		panic(err)
	}

	mp := NewCListMempool(cfg.Mempool, appConnMem, 0)
	mp.SetLogger(log.TestingLogger())

	return mp, func() { os.RemoveAll(cfg.RootDir) }
}

func ensureNoFire(t *testing.T, ch <-chan struct{}, timeoutMS int) {
	timer := time.NewTimer(time.Duration(timeoutMS) * time.Millisecond)
	select {
	case <-ch:
		t.Fatal("Expected not to fire")
	case <-timer.C:
	}
}

func ensureFire(t *testing.T, ch <-chan struct{}, timeoutMS int) {
	timer := time.NewTimer(time.Duration(timeoutMS) * time.Millisecond)
	select {
	case <-ch:
	case <-timer.C:
		t.Fatal("Expected to fire")
	}
}

func checkTxs(t *testing.T, mp mempool.Mempool, count int, peerID uint16) types.Txs {
	txs := make(types.Txs, count)
	txInfo := mempool.TxInfo{SenderID: peerID}
	for i := 0; i < count; i++ {
		txBytes := kvstore.NewRandomTx(20)
		txs[i] = txBytes
		if err := mp.CheckTx(txBytes, nil, txInfo); err != nil {
			// Skip invalid txs.
			// TestMempoolFilters will fail otherwise. It asserts a number of txs
			// returned.
			if mempool.IsPreCheckError(err) {
				continue
			}
			t.Fatalf("CheckTx failed: %v while checking #%d tx", err, i)
		}
	}
	return txs
}

func TestReapMaxBytesMaxGas(t *testing.T) {
	app := kvstore.NewInMemoryApplication()
	cc := proxy.NewLocalClientCreator(app)
	mp, cleanup := newMempoolWithApp(cc)
	defer cleanup()

	// Ensure gas calculation behaves as expected
	checkTxs(t, mp, 1, mempool.UnknownPeerID)
	tx0 := mp.TxsFront().Value.(*mempoolTx)
	require.Equal(t, tx0.gasWanted, int64(1), "transactions gas was set incorrectly")
	// ensure each tx is 20 bytes long
	require.Equal(t, len(tx0.tx), 20, "Tx is longer than 20 bytes")
	mp.Flush()

	// each table driven test creates numTxsToCreate txs with checkTx, and at the end clears all remaining txs.
	// each tx has 20 bytes
	tests := []struct {
		numTxsToCreate int
		maxBytes       int64
		maxGas         int64
		expectedNumTxs int
	}{
		{20, -1, -1, 20},
		{20, -1, 0, 0},
		{20, -1, 10, 10},
		{20, -1, 30, 20},
		{20, 0, -1, 0},
		{20, 0, 10, 0},
		{20, 10, 10, 0},
		{20, 24, 10, 1},
		{20, 240, 5, 5},
		{20, 240, -1, 10},
		{20, 240, 10, 10},
		{20, 240, 15, 10},
		{20, 20000, -1, 20},
		{20, 20000, 5, 5},
		{20, 20000, 30, 20},
	}
	for tcIndex, tt := range tests {
		checkTxs(t, mp, tt.numTxsToCreate, mempool.UnknownPeerID)
		got := mp.ReapMaxBytesMaxGas(tt.maxBytes, tt.maxGas)
		assert.Equal(t, tt.expectedNumTxs, len(got), "Got %d txs, expected %d, tc #%d",
			len(got), tt.expectedNumTxs, tcIndex)
		mp.Flush()
	}
}

func TestMempoolFilters(t *testing.T) {
	app := kvstore.NewInMemoryApplication()
	cc := proxy.NewLocalClientCreator(app)
	mp, cleanup := newMempoolWithApp(cc)
	defer cleanup()
	emptyTxArr := []types.Tx{[]byte{}}

	nopPreFilter := func(tx types.Tx) error { return nil }
	nopPostFilter := func(tx types.Tx, res *abci.ResponseCheckTx) error { return nil }

	// each table driven test creates numTxsToCreate txs with checkTx, and at the end clears all remaining txs.
	// each tx has 20 bytes
	tests := []struct {
		numTxsToCreate int
		preFilter      mempool.PreCheckFunc
		postFilter     mempool.PostCheckFunc
		expectedNumTxs int
	}{
		{10, nopPreFilter, nopPostFilter, 10},
		{10, mempool.PreCheckMaxBytes(10), nopPostFilter, 0},
		{10, mempool.PreCheckMaxBytes(22), nopPostFilter, 10},
		{10, nopPreFilter, mempool.PostCheckMaxGas(-1), 10},
		{10, nopPreFilter, mempool.PostCheckMaxGas(0), 0},
		{10, nopPreFilter, mempool.PostCheckMaxGas(1), 10},
		{10, nopPreFilter, mempool.PostCheckMaxGas(3000), 10},
		{10, mempool.PreCheckMaxBytes(10), mempool.PostCheckMaxGas(20), 0},
		{10, mempool.PreCheckMaxBytes(30), mempool.PostCheckMaxGas(20), 10},
		{10, mempool.PreCheckMaxBytes(22), mempool.PostCheckMaxGas(1), 10},
		{10, mempool.PreCheckMaxBytes(22), mempool.PostCheckMaxGas(0), 0},
	}
	for tcIndex, tt := range tests {
		err := mp.Update(1, emptyTxArr, abciResponses(len(emptyTxArr), abci.CodeTypeOK), tt.preFilter, tt.postFilter)
		require.NoError(t, err)
		checkTxs(t, mp, tt.numTxsToCreate, mempool.UnknownPeerID)
		require.Equal(t, tt.expectedNumTxs, mp.Size(), "mempool had the incorrect size, on test case %d", tcIndex)
		mp.Flush()
	}
}

func TestMempoolUpdate(t *testing.T) {
	app := kvstore.NewInMemoryApplication()
	cc := proxy.NewLocalClientCreator(app)
	mp, cleanup := newMempoolWithApp(cc)
	defer cleanup()

	// 1. Adds valid txs to the cache
	{
		tx1 := kvstore.NewTxFromID(1)
		err := mp.Update(1, []types.Tx{tx1}, abciResponses(1, abci.CodeTypeOK), nil, nil)
		require.NoError(t, err)
		err = mp.CheckTx(tx1, nil, mempool.TxInfo{})
		if assert.Error(t, err) {
			assert.Equal(t, mempool.ErrTxInCache, err)
		}
	}

	// 2. Removes valid txs from the mempool
	{
		tx2 := kvstore.NewTxFromID(2)
		err := mp.CheckTx(tx2, nil, mempool.TxInfo{})
		require.NoError(t, err)
		err = mp.Update(1, []types.Tx{tx2}, abciResponses(1, abci.CodeTypeOK), nil, nil)
		require.NoError(t, err)
		assert.Zero(t, mp.Size())
	}

	// 3. Removes invalid transactions from the cache and the mempool (if present)
	{
		tx3 := kvstore.NewTxFromID(3)
		err := mp.CheckTx(tx3, nil, mempool.TxInfo{})
		require.NoError(t, err)
		err = mp.Update(1, []types.Tx{tx3}, abciResponses(1, 1), nil, nil)
		require.NoError(t, err)
		assert.Zero(t, mp.Size())

		err = mp.CheckTx(tx3, nil, mempool.TxInfo{})
		require.NoError(t, err)
	}
}

func TestMempoolUpdateDoesNotPanicWhenApplicationMissedTx(t *testing.T) {
	mockClient := new(abciclimocks.Client)
	mockClient.On("Start").Return(nil)
	mockClient.On("SetLogger", mock.Anything)

	mockClient.On("Error").Return(nil).Times(4)
	mockClient.On("Flush", mock.Anything).Return(nil)

	app := kvstore.NewInMemoryApplication()
	cc := proxy.NewLocalClientCreator(app)
	mp, cleanup, err := newMempoolWithAppMock(cc, mockClient)
	require.NoError(t, err)
	defer cleanup()

	// Add 4 transactions to the mempool by calling the mempool's `CheckTx` on each of them.
	txs := []types.Tx{[]byte{0x01}, []byte{0x02}, []byte{0x03}, []byte{0x04}}
	for idx, tx := range txs {
		mockClient.On("CheckTx", mock.Anything, &abci.RequestCheckTx{Tx: tx}).Return(&abci.ResponseCheckTx{Code: abci.CodeTypeOK}, nil)
		if idx != 0 {
			// for all other txs we expect them to be rechecked
			mockClient.On("CheckTx", mock.Anything, &abci.RequestCheckTx{Tx: tx, Type: 1}).Return(&abci.ResponseCheckTx{Code: abci.CodeTypeOK}, nil)
		}
		err := mp.CheckTx(tx, nil, mempool.TxInfo{})
		require.NoError(t, err)
	}

	// Calling update to remove the first transaction from the mempool.
	// This call also triggers the mempool to recheck its remaining transactions.
	err = mp.Update(0, []types.Tx{txs[0]}, abciResponses(1, abci.CodeTypeOK), nil, nil)
	require.Nil(t, err)

	mockClient.AssertExpectations(t)
}

func TestMempool_KeepInvalidTxsInCache(t *testing.T) {
	app := kvstore.NewInMemoryApplication()
	cc := proxy.NewLocalClientCreator(app)
	wcfg := config.DefaultConfig()
	wcfg.Mempool.KeepInvalidTxsInCache = true
	mp, cleanup := newMempoolWithAppAndConfig(cc, wcfg)
	defer cleanup()

	// 1. An invalid transaction must remain in the cache after Update
	{
		a := make([]byte, 8)
		binary.BigEndian.PutUint64(a, 0)

		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, 1)

		err := mp.CheckTx(b, nil, mempool.TxInfo{})
		require.NoError(t, err)

		// simulate new block
		_, err = app.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
			Txs: [][]byte{a, b},
		})
		require.NoError(t, err)
		err = mp.Update(1, []types.Tx{a, b},
			[]*abci.ExecTxResult{{Code: abci.CodeTypeOK}, {Code: 2}}, nil, nil)
		require.NoError(t, err)

		// a must be added to the cache
		err = mp.CheckTx(a, nil, mempool.TxInfo{})
		if assert.Error(t, err) {
			assert.Equal(t, mempool.ErrTxInCache, err)
		}

		// b must remain in the cache
		err = mp.CheckTx(b, nil, mempool.TxInfo{})
		if assert.Error(t, err) {
			assert.Equal(t, mempool.ErrTxInCache, err)
		}
	}

	// 2. An invalid transaction must remain in the cache
	{
		a := make([]byte, 8)
		binary.BigEndian.PutUint64(a, 0)

		// remove a from the cache to test (2)
		mp.cache.Remove(a)

		err := mp.CheckTx(a, nil, mempool.TxInfo{})
		require.NoError(t, err)
	}
}

func TestTxsAvailable(t *testing.T) {
	app := kvstore.NewInMemoryApplication()
	cc := proxy.NewLocalClientCreator(app)
	mp, cleanup := newMempoolWithApp(cc)
	defer cleanup()
	mp.EnableTxsAvailable()

	timeoutMS := 500

	// with no txs, it shouldnt fire
	ensureNoFire(t, mp.TxsAvailable(), timeoutMS)

	// send a bunch of txs, it should only fire once
	txs := checkTxs(t, mp, 100, mempool.UnknownPeerID)
	ensureFire(t, mp.TxsAvailable(), timeoutMS)
	ensureNoFire(t, mp.TxsAvailable(), timeoutMS)

	// call update with half the txs.
	// it should fire once now for the new height
	// since there are still txs left
	committedTxs, remainingTxs := txs[:50], txs[50:]
	if err := mp.Update(1, committedTxs, abciResponses(len(committedTxs), abci.CodeTypeOK), nil, nil); err != nil {
		t.Error(err)
	}
	ensureFire(t, mp.TxsAvailable(), timeoutMS)
	ensureNoFire(t, mp.TxsAvailable(), timeoutMS)

	// send a bunch more txs. we already fired for this height so it shouldnt fire again
	moreTxs := checkTxs(t, mp, 50, mempool.UnknownPeerID)
	ensureNoFire(t, mp.TxsAvailable(), timeoutMS)

	// now call update with all the txs. it should not fire as there are no txs left
	committedTxs = append(remainingTxs, moreTxs...)
	if err := mp.Update(2, committedTxs, abciResponses(len(committedTxs), abci.CodeTypeOK), nil, nil); err != nil {
		t.Error(err)
	}
	ensureNoFire(t, mp.TxsAvailable(), timeoutMS)

	// send a bunch more txs, it should only fire once
	checkTxs(t, mp, 100, mempool.UnknownPeerID)
	ensureFire(t, mp.TxsAvailable(), timeoutMS)
	ensureNoFire(t, mp.TxsAvailable(), timeoutMS)
}

func TestSerialReap(t *testing.T) {
	app := kvstore.NewInMemoryApplication()
	cc := proxy.NewLocalClientCreator(app)

	mp, cleanup := newMempoolWithApp(cc)
	defer cleanup()

	appConnCon, _ := cc.NewABCIClient()
	appConnCon.SetLogger(log.TestingLogger().With("module", "abci-client", "connection", "consensus"))
	err := appConnCon.Start()
	require.Nil(t, err)

	cacheMap := make(map[string]struct{})
	deliverTxsRange := func(start, end int) {
		// Deliver some txs.
		for i := start; i < end; i++ {
			txBytes := kvstore.NewTx(fmt.Sprintf("%d", i), "true")
			err := mp.CheckTx(txBytes, nil, mempool.TxInfo{})
			_, cached := cacheMap[string(txBytes)]
			if cached {
				require.NotNil(t, err, "expected error for cached tx")
			} else {
				require.Nil(t, err, "expected no err for uncached tx")
			}
			cacheMap[string(txBytes)] = struct{}{}

			// Duplicates are cached and should return error
			err = mp.CheckTx(txBytes, nil, mempool.TxInfo{})
			require.NotNil(t, err, "Expected error after CheckTx on duplicated tx")
		}
	}

	reapCheck := func(exp int) {
		txs := mp.ReapMaxBytesMaxGas(-1, -1)
		require.Equal(t, len(txs), exp, fmt.Sprintf("Expected to reap %v txs but got %v", exp, len(txs)))
	}

	updateRange := func(start, end int) {
		txs := make(types.Txs, end-start)
		for i := start; i < end; i++ {
			txs[i-start] = kvstore.NewTx(fmt.Sprintf("%d", i), "true")
		}
		if err := mp.Update(0, txs, abciResponses(len(txs), abci.CodeTypeOK), nil, nil); err != nil {
			t.Error(err)
		}
	}

	commitRange := func(start, end int) {
		// Deliver some txs in a block
		txs := make([][]byte, end-start)
		for i := start; i < end; i++ {
			txs[i-start] = kvstore.NewTx(fmt.Sprintf("%d", i), "true")
		}

		res, err := appConnCon.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{Txs: txs})
		if err != nil {
			t.Errorf("client error committing tx: %v", err)
		}
		for _, txResult := range res.TxResults {
			if txResult.IsErr() {
				t.Errorf("error committing tx. Code:%v result:%X log:%v",
					txResult.Code, txResult.Data, txResult.Log)
			}
		}
		if len(res.AgreedAppData) != 8 {
			t.Errorf("error committing. Hash:%X", res.AgreedAppData)
		}

		_, err = appConnCon.Commit(context.Background(), &abci.RequestCommit{})
		if err != nil {
			t.Errorf("client error committing: %v", err)
		}
	}

	//----------------------------------------

	// Deliver some txs.
	deliverTxsRange(0, 100)

	// Reap the txs.
	reapCheck(100)

	// Reap again.  We should get the same amount
	reapCheck(100)

	// Deliver 0 to 999, we should reap 900 new txs
	// because 100 were already counted.
	deliverTxsRange(0, 1000)

	// Reap the txs.
	reapCheck(1000)

	// Reap again.  We should get the same amount
	reapCheck(1000)

	// Commit from the conensus AppConn
	commitRange(0, 500)
	updateRange(0, 500)

	// We should have 500 left.
	reapCheck(500)

	// Deliver 100 invalid txs and 100 valid txs
	deliverTxsRange(900, 1100)

	// We should have 600 now.
	reapCheck(600)
}

func TestMempool_CheckTxChecksTxSize(t *testing.T) {
	app := kvstore.NewInMemoryApplication()
	cc := proxy.NewLocalClientCreator(app)

	mempl, cleanup := newMempoolWithApp(cc)
	defer cleanup()

	maxTxSize := mempl.config.MaxTxBytes

	testCases := []struct {
		len int
		err bool
	}{
		// check small txs. no error
		0: {10, false},
		1: {1000, false},
		2: {1000000, false},

		// check around maxTxSize
		3: {maxTxSize - 1, false},
		4: {maxTxSize, false},
		5: {maxTxSize + 1, true},
	}

	for i, testCase := range testCases {
		caseString := fmt.Sprintf("case %d, len %d", i, testCase.len)

		tx := tmrand.Bytes(testCase.len)

		err := mempl.CheckTx(tx, nil, mempool.TxInfo{})
		bv := gogotypes.BytesValue{Value: tx}
		bz, err2 := bv.Marshal()
		require.NoError(t, err2)
		require.Equal(t, len(bz), proto.Size(&bv), caseString)

		if !testCase.err {
			require.NoError(t, err, caseString)
		} else {
			require.Equal(t, err, mempool.ErrTxTooLarge{
				Max:    maxTxSize,
				Actual: testCase.len,
			}, caseString)
		}
	}
}

func TestMempoolTxsBytes(t *testing.T) {
	app := kvstore.NewInMemoryApplication()
	cc := proxy.NewLocalClientCreator(app)

	cfg := test.ResetTestRoot("mempool_test")

	cfg.Mempool.MaxTxsBytes = 100
	mp, cleanup := newMempoolWithAppAndConfig(cc, cfg)
	defer cleanup()

	// 1. zero by default
	assert.EqualValues(t, 0, mp.SizeBytes())

	// 2. len(tx) after CheckTx
	tx1 := kvstore.NewRandomTx(10)
	err := mp.CheckTx(tx1, nil, mempool.TxInfo{})
	require.NoError(t, err)
	assert.EqualValues(t, 10, mp.SizeBytes())

	// 3. zero again after tx is removed by Update
	err = mp.Update(1, []types.Tx{tx1}, abciResponses(1, abci.CodeTypeOK), nil, nil)
	require.NoError(t, err)
	assert.EqualValues(t, 0, mp.SizeBytes())

	// 4. zero after Flush
	tx2 := kvstore.NewRandomTx(20)
	err = mp.CheckTx(tx2, nil, mempool.TxInfo{})
	require.NoError(t, err)
	assert.EqualValues(t, 20, mp.SizeBytes())

	mp.Flush()
	assert.EqualValues(t, 0, mp.SizeBytes())

	// 5. ErrMempoolIsFull is returned when/if MaxTxsBytes limit is reached.
	tx3 := kvstore.NewRandomTx(100)
	err = mp.CheckTx(tx3, nil, mempool.TxInfo{})
	require.NoError(t, err)

	tx4 := kvstore.NewRandomTx(10)
	err = mp.CheckTx(tx4, nil, mempool.TxInfo{})
	if assert.Error(t, err) {
		assert.IsType(t, mempool.ErrMempoolIsFull{}, err)
	}

	// 6. zero after tx is rechecked and removed due to not being valid anymore
	app2 := kvstore.NewInMemoryApplication()
	cc = proxy.NewLocalClientCreator(app2)

	mp, cleanup = newMempoolWithApp(cc)
	defer cleanup()

	txBytes := kvstore.NewRandomTx(10)

	err = mp.CheckTx(txBytes, nil, mempool.TxInfo{})
	require.NoError(t, err)
	assert.EqualValues(t, 10, mp.SizeBytes())

	appConnCon, _ := cc.NewABCIClient()
	appConnCon.SetLogger(log.TestingLogger().With("module", "abci-client", "connection", "consensus"))
	err = appConnCon.Start()
	require.Nil(t, err)
	t.Cleanup(func() {
		if err := appConnCon.Stop(); err != nil {
			t.Error(err)
		}
	})

	res, err := appConnCon.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{Txs: [][]byte{txBytes}})
	require.NoError(t, err)
	require.EqualValues(t, 0, res.TxResults[0].Code)
	require.NotEmpty(t, res.AgreedAppData)

	_, err = appConnCon.Commit(context.Background(), &abci.RequestCommit{})
	require.NoError(t, err)

	// Pretend like we committed nothing so txBytes gets rechecked and removed.
	err = mp.Update(1, []types.Tx{}, abciResponses(0, abci.CodeTypeOK), nil, nil)
	require.NoError(t, err)
	assert.EqualValues(t, 10, mp.SizeBytes())

	// 7. Test RemoveTxByKey function
	err = mp.CheckTx(tx1, nil, mempool.TxInfo{})
	require.NoError(t, err)
	assert.EqualValues(t, 20, mp.SizeBytes())
	assert.Error(t, mp.RemoveTxByKey(types.Tx([]byte{0x07}).Key()))
	assert.EqualValues(t, 20, mp.SizeBytes())
	assert.NoError(t, mp.RemoveTxByKey(types.Tx(tx1).Key()))
	assert.EqualValues(t, 10, mp.SizeBytes())

}

// This will non-deterministically catch some concurrency failures like
// https://github.com/tendermint/tendermint/issues/3509
// TODO: all of the tests should probably also run using the remote proxy app
// since otherwise we're not actually testing the concurrency of the mempool here!
func TestMempoolRemoteAppConcurrency(t *testing.T) {
	sockPath := fmt.Sprintf("unix:///tmp/echo_%v.sock", tmrand.Str(6))
	app := kvstore.NewInMemoryApplication()
	_, server := newRemoteApp(t, sockPath, app)
	t.Cleanup(func() {
		if err := server.Stop(); err != nil {
			t.Error(err)
		}
	})

	cfg := test.ResetTestRoot("mempool_test")

	mp, cleanup := newMempoolWithAppAndConfig(proxy.NewRemoteClientCreator(sockPath, "socket", true), cfg)
	defer cleanup()

	// generate small number of txs
	nTxs := 10
	txLen := 200
	txs := make([]types.Tx, nTxs)
	for i := 0; i < nTxs; i++ {
		txs[i] = kvstore.NewRandomTx(txLen)
	}

	// simulate a group of peers sending them over and over
	N := cfg.Mempool.Size
	maxPeers := 5
	for i := 0; i < N; i++ {
		peerID := mrand.Intn(maxPeers)
		txNum := mrand.Intn(nTxs)
		tx := txs[txNum]

		// this will err with ErrTxInCache many times ...
		mp.CheckTx(tx, nil, mempool.TxInfo{SenderID: uint16(peerID)}) //nolint: errcheck // will error
	}

	require.NoError(t, mp.FlushAppConn())
}

// caller must close server
func newRemoteApp(t *testing.T, addr string, app abci.Application) (abciclient.Client, service.Service) {
	clientCreator, err := abciclient.NewClient(addr, "socket", true)
	require.NoError(t, err)

	// Start server
	server := abciserver.NewSocketServer(addr, app)
	server.SetLogger(log.TestingLogger().With("module", "abci-server"))
	if err := server.Start(); err != nil {
		t.Fatalf("Error starting socket server: %v", err.Error())
	}

	return clientCreator, server
}

func abciResponses(n int, code uint32) []*abci.ExecTxResult {
	responses := make([]*abci.ExecTxResult, 0, n)
	for i := 0; i < n; i++ {
		responses = append(responses, &abci.ExecTxResult{Code: code})
	}
	return responses
}
