package mempool

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"math/big"
	mrand "math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/tendermint/tendermint/libs/clist"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	amino "github.com/tendermint/go-amino"

	"github.com/tendermint/tendermint/abci/example/counter"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abciserver "github.com/tendermint/tendermint/abci/server"
	abci "github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

// A cleanupFunc cleans up any config / test files created for a particular
// test.
type cleanupFunc func()

func newMempoolWithApp(cc proxy.ClientCreator) (*CListMempool, cleanupFunc) {
	return newMempoolWithAppAndConfig(cc, cfg.ResetTestRoot("mempool_test"))
}

func newMempoolWithAppAndConfig(cc proxy.ClientCreator, config *cfg.Config) (*CListMempool, cleanupFunc) {
	appConnMem, _ := cc.NewABCIClient()
	appConnMem.SetLogger(log.TestingLogger().With("module", "abci-client", "connection", "mempool"))
	err := appConnMem.Start()
	if err != nil {
		panic(err)
	}
	mempool := NewCListMempool(config.Mempool, appConnMem, 0)
	mempool.SetLogger(log.TestingLogger())
	return mempool, func() { os.RemoveAll(config.RootDir) }
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

func checkTxs(t *testing.T, mempool Mempool, count int, peerID uint16) types.Txs {
	txs := make(types.Txs, count)
	txInfo := TxInfo{SenderID: peerID}
	for i := 0; i < count; i++ {
		txBytes := make([]byte, 20)
		txs[i] = txBytes
		_, err := rand.Read(txBytes)
		if err != nil {
			t.Error(err)
		}
		if err := mempool.CheckTx(txBytes, nil, txInfo); err != nil {
			// Skip invalid txs.
			// TestMempoolFilters will fail otherwise. It asserts a number of txs
			// returned.
			if IsPreCheckError(err) {
				continue
			}
			t.Fatalf("CheckTx failed: %v while checking #%d tx", err, i)
		}
	}
	return txs
}

func TestReapMaxBytesMaxGas(t *testing.T) {
	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	mempool, cleanup := newMempoolWithApp(cc)
	defer cleanup()

	// Ensure gas calculation behaves as expected
	checkTxs(t, mempool, 1, UnknownPeerID)
	tx0 := mempool.TxsFront().Value.(*mempoolTx)
	// assert that kv store has gas wanted = 1.
	require.Equal(t, app.CheckTx(abci.RequestCheckTx{Tx: tx0.tx}).GasWanted, int64(1), "KVStore had a gas value neq to 1")
	require.Equal(t, tx0.gasWanted, int64(1), "transactions gas was set incorrectly")
	// ensure each tx is 20 bytes long
	require.Equal(t, len(tx0.tx), 20, "Tx is longer than 20 bytes")
	mempool.Flush()

	// each table driven test creates numTxsToCreate txs with checkTx, and at the end clears all remaining txs.
	// each tx has 20 bytes + amino overhead = 21 bytes, 1 gas
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
		{20, 22, 10, 1},
		{20, 220, -1, 10},
		{20, 220, 5, 5},
		{20, 220, 10, 10},
		{20, 220, 15, 10},
		{20, 20000, -1, 20},
		{20, 20000, 5, 5},
		{20, 20000, 30, 20},
		{2000, -1, -1, 300},
	}
	for tcIndex, tt := range tests {
		checkTxs(t, mempool, tt.numTxsToCreate, UnknownPeerID)
		got := mempool.ReapMaxBytesMaxGas(tt.maxBytes, tt.maxGas)
		assert.Equal(t, tt.expectedNumTxs, len(got), "Got %d txs, expected %d, tc #%d",
			len(got), tt.expectedNumTxs, tcIndex)
		mempool.Flush()
	}
}

func TestMempoolFilters(t *testing.T) {
	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	mempool, cleanup := newMempoolWithApp(cc)
	defer cleanup()
	emptyTxArr := []types.Tx{[]byte{}}

	nopPreFilter := func(tx types.Tx) error { return nil }
	nopPostFilter := func(tx types.Tx, res *abci.ResponseCheckTx) error { return nil }

	// each table driven test creates numTxsToCreate txs with checkTx, and at the end clears all remaining txs.
	// each tx has 20 bytes + amino overhead = 21 bytes, 1 gas
	tests := []struct {
		numTxsToCreate int
		preFilter      PreCheckFunc
		postFilter     PostCheckFunc
		expectedNumTxs int
	}{
		{10, nopPreFilter, nopPostFilter, 10},
		{10, PreCheckAminoMaxBytes(10), nopPostFilter, 0},
		{10, PreCheckAminoMaxBytes(20), nopPostFilter, 0},
		{10, PreCheckAminoMaxBytes(22), nopPostFilter, 10},
		{10, nopPreFilter, PostCheckMaxGas(-1), 10},
		{10, nopPreFilter, PostCheckMaxGas(0), 0},
		{10, nopPreFilter, PostCheckMaxGas(1), 10},
		{10, nopPreFilter, PostCheckMaxGas(3000), 10},
		{10, PreCheckAminoMaxBytes(10), PostCheckMaxGas(20), 0},
		{10, PreCheckAminoMaxBytes(30), PostCheckMaxGas(20), 10},
		{10, PreCheckAminoMaxBytes(22), PostCheckMaxGas(1), 10},
		{10, PreCheckAminoMaxBytes(22), PostCheckMaxGas(0), 0},
	}
	for tcIndex, tt := range tests {
		mempool.Update(1, emptyTxArr, abciResponses(len(emptyTxArr), abci.CodeTypeOK), tt.preFilter, tt.postFilter)
		checkTxs(t, mempool, tt.numTxsToCreate, UnknownPeerID)
		require.Equal(t, tt.expectedNumTxs, mempool.Size(), "mempool had the incorrect size, on test case %d", tcIndex)
		mempool.Flush()
	}
}

func TestMempoolUpdate(t *testing.T) {
	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	mempool, cleanup := newMempoolWithApp(cc)
	defer cleanup()

	// 1. Adds valid txs to the cache
	{
		mempool.Update(1, []types.Tx{[]byte{0x01}}, abciResponses(1, abci.CodeTypeOK), nil, nil)
		err := mempool.CheckTx([]byte{0x01}, nil, TxInfo{})
		if assert.Error(t, err) {
			assert.Equal(t, ErrTxInCache, err)
		}
	}

	// 2. Removes valid txs from the mempool
	{
		err := mempool.CheckTx([]byte{0x02}, nil, TxInfo{})
		require.NoError(t, err)
		mempool.Update(1, []types.Tx{[]byte{0x02}}, abciResponses(1, abci.CodeTypeOK), nil, nil)
		assert.Zero(t, mempool.Size())
	}

	// 3. Removes invalid transactions from the cache and the mempool (if present)
	{
		err := mempool.CheckTx([]byte{0x03}, nil, TxInfo{})
		require.NoError(t, err)
		mempool.Update(1, []types.Tx{[]byte{0x03}}, abciResponses(1, 1), nil, nil)
		assert.Zero(t, mempool.Size())

		err = mempool.CheckTx([]byte{0x03}, nil, TxInfo{})
		assert.NoError(t, err)
	}
}

func TestTxsAvailable(t *testing.T) {
	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	mempool, cleanup := newMempoolWithApp(cc)
	defer cleanup()
	mempool.EnableTxsAvailable()

	timeoutMS := 500

	// with no txs, it shouldnt fire
	ensureNoFire(t, mempool.TxsAvailable(), timeoutMS)

	// send a bunch of txs, it should only fire once
	txs := checkTxs(t, mempool, 100, UnknownPeerID)
	ensureFire(t, mempool.TxsAvailable(), timeoutMS)
	ensureNoFire(t, mempool.TxsAvailable(), timeoutMS)

	// call update with half the txs.
	// it should fire once now for the new height
	// since there are still txs left
	committedTxs, txs := txs[:50], txs[50:]
	if err := mempool.Update(1, committedTxs, abciResponses(len(committedTxs), abci.CodeTypeOK), nil, nil); err != nil {
		t.Error(err)
	}
	ensureFire(t, mempool.TxsAvailable(), timeoutMS)
	ensureNoFire(t, mempool.TxsAvailable(), timeoutMS)

	// send a bunch more txs. we already fired for this height so it shouldnt fire again
	moreTxs := checkTxs(t, mempool, 50, UnknownPeerID)
	ensureNoFire(t, mempool.TxsAvailable(), timeoutMS)

	// now call update with all the txs. it should not fire as there are no txs left
	committedTxs = append(txs, moreTxs...) //nolint: gocritic
	if err := mempool.Update(2, committedTxs, abciResponses(len(committedTxs), abci.CodeTypeOK), nil, nil); err != nil {
		t.Error(err)
	}
	ensureNoFire(t, mempool.TxsAvailable(), timeoutMS)

	// send a bunch more txs, it should only fire once
	checkTxs(t, mempool, 100, UnknownPeerID)
	ensureFire(t, mempool.TxsAvailable(), timeoutMS)
	ensureNoFire(t, mempool.TxsAvailable(), timeoutMS)
}

func TestSerialReap(t *testing.T) {
	app := counter.NewApplication(true)
	app.SetOption(abci.RequestSetOption{Key: "serial", Value: "on"})
	cc := proxy.NewLocalClientCreator(app)

	mempool, cleanup := newMempoolWithApp(cc)
	defer cleanup()
	mempool.config.MaxTxNumPerBlock = 10000

	appConnCon, _ := cc.NewABCIClient()
	appConnCon.SetLogger(log.TestingLogger().With("module", "abci-client", "connection", "consensus"))
	err := appConnCon.Start()
	require.Nil(t, err)

	cacheMap := make(map[string]struct{})
	deliverTxsRange := func(start, end int) {
		// Deliver some txs.
		for i := start; i < end; i++ {

			// This will succeed
			txBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(txBytes, uint64(i))
			err := mempool.CheckTx(txBytes, nil, TxInfo{})
			_, cached := cacheMap[string(txBytes)]
			if cached {
				require.NotNil(t, err, "expected error for cached tx")
			} else {
				require.Nil(t, err, "expected no err for uncached tx")
			}
			cacheMap[string(txBytes)] = struct{}{}

			// Duplicates are cached and should return error
			err = mempool.CheckTx(txBytes, nil, TxInfo{})
			require.NotNil(t, err, "Expected error after CheckTx on duplicated tx")
		}
	}

	reapCheck := func(exp int) {
		txs := mempool.ReapMaxBytesMaxGas(-1, -1)
		require.Equal(t, len(txs), exp, fmt.Sprintf("Expected to reap %v txs but got %v", exp, len(txs)))
	}

	updateRange := func(start, end int) {
		txs := make([]types.Tx, 0)
		for i := start; i < end; i++ {
			txBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(txBytes, uint64(i))
			txs = append(txs, txBytes)
		}
		if err := mempool.Update(0, txs, abciResponses(len(txs), abci.CodeTypeOK), nil, nil); err != nil {
			t.Error(err)
		}
	}

	commitRange := func(start, end int) {
		// Deliver some txs.
		for i := start; i < end; i++ {
			txBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(txBytes, uint64(i))
			res, err := appConnCon.DeliverTxSync(abci.RequestDeliverTx{Tx: txBytes})
			if err != nil {
				t.Errorf("client error committing tx: %v", err)
			}
			if res.IsErr() {
				t.Errorf("error committing tx. Code:%v result:%X log:%v",
					res.Code, res.Data, res.Log)
			}
		}
		res, err := appConnCon.CommitSync()
		if err != nil {
			t.Errorf("client error committing: %v", err)
		}
		if len(res.Data) != 8 {
			t.Errorf("error committing. Hash:%X", res.Data)
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

func TestMempoolCloseWAL(t *testing.T) {
	// 1. Create the temporary directory for mempool and WAL testing.
	rootDir, err := ioutil.TempDir("", "mempool-test")
	require.Nil(t, err, "expecting successful tmpdir creation")

	// 2. Ensure that it doesn't contain any elements -- Sanity check
	m1, err := filepath.Glob(filepath.Join(rootDir, "*"))
	require.Nil(t, err, "successful globbing expected")
	require.Equal(t, 0, len(m1), "no matches yet")

	// 3. Create the mempool
	wcfg := cfg.DefaultConfig()
	wcfg.Mempool.RootDir = rootDir
	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	mempool, cleanup := newMempoolWithAppAndConfig(cc, wcfg)
	defer cleanup()
	mempool.height = 10
	mempool.InitWAL()

	// 4. Ensure that the directory contains the WAL file
	m2, err := filepath.Glob(filepath.Join(rootDir, "*"))
	require.Nil(t, err, "successful globbing expected")
	require.Equal(t, 1, len(m2), "expecting the wal match in")

	// 5. Write some contents to the WAL
	mempool.CheckTx(types.Tx([]byte("foo")), nil, TxInfo{})
	walFilepath := mempool.wal.Path
	sum1 := checksumFile(walFilepath, t)

	// 6. Sanity check to ensure that the written TX matches the expectation.
	require.Equal(t, sum1, checksumIt([]byte("foo\n")), "foo with a newline should be written")

	// 7. Invoke CloseWAL() and ensure it discards the
	// WAL thus any other write won't go through.
	mempool.CloseWAL()
	mempool.CheckTx(types.Tx([]byte("bar")), nil, TxInfo{})
	sum2 := checksumFile(walFilepath, t)
	require.Equal(t, sum1, sum2, "expected no change to the WAL after invoking CloseWAL() since it was discarded")

	// 8. Sanity check to ensure that the WAL file still exists
	m3, err := filepath.Glob(filepath.Join(rootDir, "*"))
	require.Nil(t, err, "successful globbing expected")
	require.Equal(t, 1, len(m3), "expecting the wal match in")
}

// Size of the amino encoded TxMessage is the length of the
// encoded byte array, plus 1 for the struct field, plus 4
// for the amino prefix.
func txMessageSize(tx types.Tx) int {
	return amino.ByteSliceSize(tx) + 1 + 4
}

func TestMempoolMaxMsgSize(t *testing.T) {
	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	mempl, cleanup := newMempoolWithApp(cc)
	defer cleanup()

	maxTxSize := mempl.config.MaxTxBytes
	maxMsgSize := calcMaxMsgSize(maxTxSize)

	testCases := []struct {
		len int
		err bool
	}{
		// check small txs. no error
		{10, false},
		{1000, false},
		{1000000, false},

		// check around maxTxSize
		// changes from no error to error
		{maxTxSize - 2, false},
		{maxTxSize - 1, false},
		{maxTxSize, false},
		{maxTxSize + 1, true},
		{maxTxSize + 2, true},

		// check around maxMsgSize. all error
		{maxMsgSize - 1, true},
		{maxMsgSize, true},
		{maxMsgSize + 1, true},
	}

	for i, testCase := range testCases {
		caseString := fmt.Sprintf("case %d, len %d", i, testCase.len)

		tx := tmrand.Bytes(testCase.len)
		err := mempl.CheckTx(tx, nil, TxInfo{})
		msg := &TxMessage{tx}
		encoded := cdc.MustMarshalBinaryBare(msg)
		require.Equal(t, len(encoded), txMessageSize(tx), caseString)
		if !testCase.err {
			require.True(t, len(encoded) <= maxMsgSize, caseString)
			require.NoError(t, err, caseString)
		} else {
			require.True(t, len(encoded) > maxMsgSize, caseString)
			require.Equal(t, err, ErrTxTooLarge{maxTxSize, testCase.len}, caseString)
		}
	}

}

func TestMempoolTxsBytes(t *testing.T) {
	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	config := cfg.ResetTestRoot("mempool_test")
	config.Mempool.MaxTxsBytes = 10
	mempool, cleanup := newMempoolWithAppAndConfig(cc, config)
	defer cleanup()

	// 1. zero by default
	assert.EqualValues(t, 0, mempool.TxsBytes())

	// 2. len(tx) after CheckTx
	err := mempool.CheckTx([]byte{0x01}, nil, TxInfo{})
	require.NoError(t, err)
	assert.EqualValues(t, 1, mempool.TxsBytes())

	// 3. zero again after tx is removed by Update
	mempool.Update(1, []types.Tx{[]byte{0x01}}, abciResponses(1, abci.CodeTypeOK), nil, nil)
	assert.EqualValues(t, 0, mempool.TxsBytes())

	// 4. zero after Flush
	err = mempool.CheckTx([]byte{0x02, 0x03}, nil, TxInfo{})
	require.NoError(t, err)
	assert.EqualValues(t, 2, mempool.TxsBytes())

	mempool.Flush()
	assert.EqualValues(t, 0, mempool.TxsBytes())

	// 5. ErrMempoolIsFull is returned when/if MaxTxsBytes limit is reached.
	err = mempool.CheckTx([]byte{0x04, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04}, nil, TxInfo{})
	require.NoError(t, err)
	err = mempool.CheckTx([]byte{0x05}, nil, TxInfo{})
	if assert.Error(t, err) {
		assert.IsType(t, ErrMempoolIsFull{}, err)
	}

	// 6. zero after tx is rechecked and removed due to not being valid anymore
	app2 := counter.NewApplication(true)
	cc = proxy.NewLocalClientCreator(app2)
	mempool, cleanup = newMempoolWithApp(cc)
	defer cleanup()

	txBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(txBytes, uint64(0))

	err = mempool.CheckTx(txBytes, nil, TxInfo{})
	require.NoError(t, err)
	assert.EqualValues(t, 8, mempool.TxsBytes())

	appConnCon, _ := cc.NewABCIClient()
	appConnCon.SetLogger(log.TestingLogger().With("module", "abci-client", "connection", "consensus"))
	err = appConnCon.Start()
	require.Nil(t, err)
	defer appConnCon.Stop()
	res, err := appConnCon.DeliverTxSync(abci.RequestDeliverTx{Tx: txBytes})
	require.NoError(t, err)
	require.EqualValues(t, 0, res.Code)
	res2, err := appConnCon.CommitSync()
	require.NoError(t, err)
	require.NotEmpty(t, res2.Data)

	// Pretend like we committed nothing so txBytes gets rechecked and removed.
	mempool.Update(1, []types.Tx{}, abciResponses(0, abci.CodeTypeOK), nil, nil)
	assert.EqualValues(t, 0, mempool.TxsBytes())
}

// This will non-deterministically catch some concurrency failures like
// https://github.com/tendermint/tendermint/issues/3509
// TODO: all of the tests should probably also run using the remote proxy app
// since otherwise we're not actually testing the concurrency of the mempool here!
func TestMempoolRemoteAppConcurrency(t *testing.T) {
	sockPath := fmt.Sprintf("unix:///tmp/echo_%v.sock", tmrand.Str(6))
	app := kvstore.NewApplication()
	cc, server := newRemoteApp(t, sockPath, app)
	defer server.Stop()
	config := cfg.ResetTestRoot("mempool_test")
	mempool, cleanup := newMempoolWithAppAndConfig(cc, config)
	defer cleanup()

	// generate small number of txs
	nTxs := 10
	txLen := 200
	txs := make([]types.Tx, nTxs)
	for i := 0; i < nTxs; i++ {
		txs[i] = tmrand.Bytes(txLen)
	}

	// simulate a group of peers sending them over and over
	N := config.Mempool.Size
	maxPeers := 5
	for i := 0; i < N; i++ {
		peerID := mrand.Intn(maxPeers)
		txNum := mrand.Intn(nTxs)
		tx := txs[txNum]

		// this will err with ErrTxInCache many times ...
		mempool.CheckTx(tx, nil, TxInfo{SenderID: uint16(peerID)})
	}
	err := mempool.FlushAppConn()
	require.NoError(t, err)
}

// caller must close server
func newRemoteApp(
	t *testing.T,
	addr string,
	app abci.Application,
) (
	clientCreator proxy.ClientCreator,
	server service.Service,
) {
	clientCreator = proxy.NewRemoteClientCreator(addr, "socket", true)

	// Start server
	server = abciserver.NewSocketServer(addr, app)
	server.SetLogger(log.TestingLogger().With("module", "abci-server"))
	if err := server.Start(); err != nil {
		t.Fatalf("Error starting socket server: %v", err.Error())
	}
	return clientCreator, server
}
func checksumIt(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func checksumFile(p string, t *testing.T) string {
	data, err := ioutil.ReadFile(p)
	require.Nil(t, err, "expecting successful read of %q", p)
	return checksumIt(data)
}

func abciResponses(n int, code uint32) []*abci.ResponseDeliverTx {
	responses := make([]*abci.ResponseDeliverTx, 0, n)
	for i := 0; i < n; i++ {
		responses = append(responses, &abci.ResponseDeliverTx{Code: code})
	}
	return responses
}

func TestAddAndSortTx(t *testing.T) {
	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	config := cfg.ResetTestRoot("mempool_test")
	config.Mempool.SortTxByGp = true
	mempool, cleanup := newMempoolWithAppAndConfig(cc, config)
	defer cleanup()

	//tx := &mempoolTx{height: 1, gasWanted: 1, tx:[]byte{0x01}}
	testCases := []struct {
		Tx   *mempoolTx
		Info ExTxInfo
	}{
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("1")}, ExTxInfo{"18", 0, big.NewInt(3780), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("2")}, ExTxInfo{"6", 0, big.NewInt(5853), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("3")}, ExTxInfo{"7", 0, big.NewInt(8315), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("4")}, ExTxInfo{"10", 0, big.NewInt(9526), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("5")}, ExTxInfo{"15", 0, big.NewInt(9140), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("6")}, ExTxInfo{"9", 0, big.NewInt(9227), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("7")}, ExTxInfo{"3", 0, big.NewInt(761), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("8")}, ExTxInfo{"18", 0, big.NewInt(9740), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("9")}, ExTxInfo{"1", 0, big.NewInt(6574), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("10")}, ExTxInfo{"8", 0, big.NewInt(9656), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("11")}, ExTxInfo{"12", 0, big.NewInt(6554), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("12")}, ExTxInfo{"16", 0, big.NewInt(5609), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("13")}, ExTxInfo{"6", 0, big.NewInt(2791), 1}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("14")}, ExTxInfo{"18", 0, big.NewInt(2698), 1}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("15")}, ExTxInfo{"1", 0, big.NewInt(6925), 1}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("16")}, ExTxInfo{"3", 0, big.NewInt(3171), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("17")}, ExTxInfo{"1", 0, big.NewInt(2965), 2}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("18")}, ExTxInfo{"19", 0, big.NewInt(2484), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("19")}, ExTxInfo{"13", 0, big.NewInt(9722), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("20")}, ExTxInfo{"7", 0, big.NewInt(4236), 1}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("21")}, ExTxInfo{"18", 0, big.NewInt(1780), 0}},
	}

	for _, exInfo := range testCases {
		mempool.addAndSortTx(exInfo.Tx, exInfo.Info)
	}
	require.Equal(t, 18, mempool.txs.Len(), fmt.Sprintf("Expected to txs length %v but got %v", 18, mempool.txs.Len()))

	// The txs in mempool should sorted, the output should be (head -> tail):
	//
	//Address:  18  , GasPrice:  9740  , Nonce:  0
	//Address:  13  , GasPrice:  9722  , Nonce:  0
	//Address:  8  , GasPrice:  9656  , Nonce:  0
	//Address:  10  , GasPrice:  9526  , Nonce:  0
	//Address:  9  , GasPrice:  9227  , Nonce:  0
	//Address:  15  , GasPrice:  9140  , Nonce:  0
	//Address:  7  , GasPrice:  8315  , Nonce:  0
	//Address:  1  , GasPrice:  6574  , Nonce:  0
	//Address:  1  , GasPrice:  6925  , Nonce:  1
	//Address:  12  , GasPrice:  6554  , Nonce:  0
	//Address:  6  , GasPrice:  5853  , Nonce:  0
	//Address:  16  , GasPrice:  5609  , Nonce:  0
	//Address:  7  , GasPrice:  4236  , Nonce:  1
	//Address:  3  , GasPrice:  3171  , Nonce:  0
	//Address:  1  , GasPrice:  2965  , Nonce:  2
	//Address:  6  , GasPrice:  2791  , Nonce:  1
	//Address:  18  , GasPrice:  2698  , Nonce:  1
	//Address:  19  , GasPrice:  2484  , Nonce:  0

	require.Equal(t, 3, len(mempool.addressRecord["1"]))
	require.Equal(t, 1, len(mempool.addressRecord["15"]))
	require.Equal(t, 2, len(mempool.addressRecord["18"]))

	require.Equal(t, "18", mempool.txs.Front().Address)
	require.Equal(t, big.NewInt(9740), mempool.txs.Front().GasPrice)
	require.Equal(t, uint64(0), mempool.txs.Front().Nonce)

	require.Equal(t, "19", mempool.txs.Back().Address)
	require.Equal(t, big.NewInt(2484), mempool.txs.Back().GasPrice)
	require.Equal(t, uint64(0), mempool.txs.Back().Nonce)

	require.Equal(t, true, checkTx(mempool.txs.Front()))

	for addr := range mempool.addressRecord {
		require.Equal(t, true, checkAccNonce(addr, mempool.txs.Front()))
	}

	txs := mempool.ReapMaxBytesMaxGas(-1, -1)
	require.Equal(t, 18, len(txs), fmt.Sprintf("Expected to reap %v txs but got %v", 18, len(txs)))

	mempool.Flush()
	require.Equal(t, 0, mempool.txs.Len())
	require.Equal(t, 0, mempool.bcTxsList.Len())
	require.Equal(t, 0, len(mempool.addressRecord))
}

func TestReplaceTx(t *testing.T) {
	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	config := cfg.ResetTestRoot("mempool_test")
	mempool, cleanup := newMempoolWithAppAndConfig(cc, config)
	defer cleanup()

	//tx := &mempoolTx{height: 1, gasWanted: 1, tx:[]byte{0x01}}
	testCases := []struct {
		Tx   *mempoolTx
		Info ExTxInfo
	}{
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("10000")}, ExTxInfo{"1", 0, big.NewInt(9740), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("10001")}, ExTxInfo{"1", 0, big.NewInt(5853), 1}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("10002")}, ExTxInfo{"1", 0, big.NewInt(8315), 2}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("10003")}, ExTxInfo{"1", 0, big.NewInt(9526), 3}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("10004")}, ExTxInfo{"1", 0, big.NewInt(9140), 4}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("10002")}, ExTxInfo{"1", 0, big.NewInt(9227), 2}},
	}

	for _, exInfo := range testCases {
		mempool.addAndSortTx(exInfo.Tx, exInfo.Info)
	}
	require.Equal(t, 5, mempool.txs.Len(), fmt.Sprintf("Expected to txs length %v but got %v", 5, mempool.txs.Len()))
}

func TestAddAndSortTxByRandom(t *testing.T) {
	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	config := cfg.ResetTestRoot("mempool_test")
	mempool, cleanup := newMempoolWithAppAndConfig(cc, config)
	defer cleanup()

	AddrNonce := make(map[string]int)
	for i := 0; i < 1000; i++ {
		mempool.addAndSortTx(generateNode(AddrNonce, i))
	}

	require.Equal(t, true, checkTx(mempool.txs.Front()))
	for addr := range mempool.addressRecord {
		require.Equal(t, true, checkAccNonce(addr, mempool.txs.Front()))
	}
}

func TestReapUserTxs(t *testing.T) {
	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	config := cfg.ResetTestRoot("mempool_test")
	mempool, cleanup := newMempoolWithAppAndConfig(cc, config)
	defer cleanup()

	//tx := &mempoolTx{height: 1, gasWanted: 1, tx:[]byte{0x01}}
	testCases := []struct {
		Tx   *mempoolTx
		Info ExTxInfo
	}{
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("1")}, ExTxInfo{"18", 0, big.NewInt(9740), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("2")}, ExTxInfo{"6", 0, big.NewInt(5853), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("3")}, ExTxInfo{"7", 0, big.NewInt(8315), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("4")}, ExTxInfo{"10", 0, big.NewInt(9526), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("5")}, ExTxInfo{"15", 0, big.NewInt(9140), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("6")}, ExTxInfo{"9", 0, big.NewInt(9227), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("7")}, ExTxInfo{"3", 0, big.NewInt(761), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("8")}, ExTxInfo{"18", 0, big.NewInt(3780), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("9")}, ExTxInfo{"1", 0, big.NewInt(6574), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("10")}, ExTxInfo{"8", 0, big.NewInt(9656), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("11")}, ExTxInfo{"12", 0, big.NewInt(6554), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("12")}, ExTxInfo{"16", 0, big.NewInt(5609), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("13")}, ExTxInfo{"6", 0, big.NewInt(2791), 1}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("14")}, ExTxInfo{"18", 0, big.NewInt(2698), 1}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("15")}, ExTxInfo{"1", 0, big.NewInt(6925), 1}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("16")}, ExTxInfo{"3", 0, big.NewInt(3171), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("17")}, ExTxInfo{"1", 0, big.NewInt(2965), 2}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("18")}, ExTxInfo{"19", 0, big.NewInt(2484), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("19")}, ExTxInfo{"13", 0, big.NewInt(9722), 0}},
		{&mempoolTx{height: 1, gasWanted: 1, tx: []byte("20")}, ExTxInfo{"7", 0, big.NewInt(4236), 1}},
	}

	for _, exInfo := range testCases {
		mempool.addAndSortTx(exInfo.Tx, exInfo.Info)
	}
	require.Equal(t, 18, mempool.txs.Len(), fmt.Sprintf("Expected to txs length %v but got %v", 18,
		mempool.txs.Len()))

	require.Equal(t, 3, mempool.ReapUserTxsCnt("1"), fmt.Sprintf("Expected to txs length of %s "+
		"is %v but got %v", "1", 3, mempool.ReapUserTxsCnt("1")))

	require.Equal(t, 0, mempool.ReapUserTxsCnt("111"), fmt.Sprintf("Expected to txs length of %s "+
		"is %v but got %v", "111", 0, mempool.ReapUserTxsCnt("111")))

	require.Equal(t, 3, len(mempool.ReapUserTxs("1", -1)), fmt.Sprintf("Expected to txs length "+
		"of %s is %v but got %v", "1", 3, len(mempool.ReapUserTxs("1", -1))))

	require.Equal(t, 3, len(mempool.ReapUserTxs("1", 100)), fmt.Sprintf("Expected to txs length "+
		"of %s is %v but got %v", "1", 3, len(mempool.ReapUserTxs("1", 100))))

	require.Equal(t, 0, len(mempool.ReapUserTxs("111", -1)), fmt.Sprintf("Expected to txs length "+
		"of %s is %v but got %v", "111", 0, len(mempool.ReapUserTxs("111", -1))))

	require.Equal(t, 0, len(mempool.ReapUserTxs("111", 100)), fmt.Sprintf("Expected to txs length "+
		"of %s is %v but got %v", "111", 0, len(mempool.ReapUserTxs("111", 100))))
}

func generateNode(addrNonce map[string]int, idx int) (*mempoolTx, ExTxInfo) {
	mrand.Seed(time.Now().UnixNano())
	addr := strconv.Itoa(mrand.Int()%1000 + 1)
	gasPrice := mrand.Int()%100000 + 1

	nonce := 0
	if n, ok := addrNonce[addr]; ok {
		if gasPrice%177 == 0 {
			nonce = n - 1
		} else {
			nonce = n
		}
	}
	addrNonce[addr] = nonce + 1

	tx := &mempoolTx{
		height:    1,
		gasWanted: int64(idx),
		tx:        []byte(strconv.Itoa(idx)),
	}

	exInfo := ExTxInfo{
		Sender:   addr,
		GasPrice: big.NewInt(int64(gasPrice)),
		Nonce:    uint64(nonce),
	}

	return tx, exInfo
}

func checkAccNonce(addr string, head *clist.CElement) bool {
	nonce := uint64(0)

	for head != nil {
		if head.Address == addr {
			if head.Nonce != nonce {
				return false
			}
			nonce++
		}

		head = head.Next()
	}

	return true
}

func checkTx(head *clist.CElement) bool {
	for head != nil {
		next := head.Next()
		if next == nil {
			break
		}

		if head.Address == next.Address {
			if head.Nonce >= next.Nonce {
				return false
			}
		} else {
			if head.GasPrice.Cmp(next.GasPrice) < 0 {
				return false
			}
		}

		head = head.Next()
	}

	return true
}

func TestMultiPriceBump(t *testing.T) {
	tests := []struct {
		rawPrice    *big.Int
		priceBump   uint64
		targetPrice *big.Int
	}{
		{big.NewInt(1), 0, big.NewInt(1)},
		{big.NewInt(10), 1, big.NewInt(10)},
		{big.NewInt(100), 0, big.NewInt(100)},
		{big.NewInt(100), 5, big.NewInt(105)},
		{big.NewInt(100), 50, big.NewInt(150)},
		{big.NewInt(100), 150, big.NewInt(250)},
	}
	for _, tt := range tests {
		require.True(t, tt.targetPrice.Cmp(MultiPriceBump(tt.rawPrice, int64(tt.priceBump))) == 0)
	}
}
