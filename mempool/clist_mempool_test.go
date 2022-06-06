package mempool

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

// application extends the KV store application by overriding CheckTx to provide
// transaction priority based on the value in the key/value pair.
type application struct {
	*kvstore.Application
}

type testTx struct {
	tx       types.Tx
	priority int64
}

func (app *application) CheckTx(req abci.RequestCheckTx) abci.ResponseCheckTx {
	var (
		priority int64
		sender   string
	)

	// infer the priority from the raw transaction value (sender=key=value)
	parts := bytes.Split(req.Tx, []byte("="))
	if len(parts) == 3 {
		v, err := strconv.ParseInt(string(parts[2]), 10, 64)
		if err != nil {
			return abci.ResponseCheckTx{
				Priority:  priority,
				Code:      100,
				GasWanted: 1,
			}
		}

		priority = v
		sender = string(parts[0])
	} else {
		return abci.ResponseCheckTx{
			Priority:  priority,
			Code:      101,
			GasWanted: 1,
		}
	}

	return abci.ResponseCheckTx{
		Priority:  priority,
		Sender:    sender,
		Code:      code.CodeTypeOK,
		GasWanted: 1,
	}
}

func setup(t testing.TB, cacheSize int, options ...TxMempoolOption) *TxMempool {
	t.Helper()

	app := &application{kvstore.NewApplication()}
	cc := proxy.NewLocalClientCreator(app)

	cfg := config.ResetTestRoot(strings.ReplaceAll(t.Name(), "/", "|"))
	cfg.Mempool.CacheSize = cacheSize

	appConnMem, err := cc.NewABCIClient()
	require.NoError(t, err)
	require.NoError(t, appConnMem.Start())

	t.Cleanup(func() {
		os.RemoveAll(cfg.RootDir)
		require.NoError(t, appConnMem.Stop())
	})

	return NewTxMempool(log.TestingLogger().With("test", t.Name()), cfg.Mempool, appConnMem, 0, options...)
}

func checkTxs(t *testing.T, txmp *TxMempool, numTxs int, peerID uint16) []testTx {
	txs := make([]testTx, numTxs)
	txInfo := TxInfo{SenderID: peerID}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < numTxs; i++ {
		prefix := make([]byte, 20)
		_, err := rng.Read(prefix)
		require.NoError(t, err)

		priority := int64(rng.Intn(9999-1000) + 1000)

		txs[i] = testTx{
			tx:       []byte(fmt.Sprintf("sender-%d-%d=%X=%d", i, peerID, prefix, priority)),
			priority: priority,
		}
		require.NoError(t, txmp.CheckTx(txs[i].tx, nil, txInfo))
	}

	return txs
}

func TestTxMempool_TxsAvailable(t *testing.T) {
	txmp := setup(t, 0)
	txmp.EnableTxsAvailable()

	ensureNoTxFire := func() {
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-txmp.TxsAvailable():
			require.Fail(t, "unexpected transactions event")
		case <-timer.C:
		}
	}

	ensureTxFire := func() {
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-txmp.TxsAvailable():
		case <-timer.C:
			require.Fail(t, "expected transactions event")
		}
	}

	// ensure no event as we have not executed any transactions yet
	ensureNoTxFire()

	// Execute CheckTx for some transactions and ensure TxsAvailable only fires
	// once.
	txs := checkTxs(t, txmp, 100, 0)
	ensureTxFire()
	ensureNoTxFire()

	rawTxs := make([]types.Tx, len(txs))
	for i, tx := range txs {
		rawTxs[i] = tx.tx
	}

	responses := make([]*abci.ResponseDeliverTx, len(rawTxs[:50]))
	for i := 0; i < len(responses); i++ {
		responses[i] = &abci.ResponseDeliverTx{Code: abci.CodeTypeOK}
	}

	// commit half the transactions and ensure we fire an event
	txmp.Lock()
	require.NoError(t, txmp.Update(1, rawTxs[:50], responses, nil, nil))
	txmp.Unlock()
	ensureTxFire()
	ensureNoTxFire()

	// Execute CheckTx for more transactions and ensure we do not fire another
	// event as we're still on the same height (1).
	_ = checkTxs(t, txmp, 100, 0)
	ensureNoTxFire()
}

func TestTxMempool_Size(t *testing.T) {
	txmp := setup(t, 0)
	txs := checkTxs(t, txmp, 100, 0)
	require.Equal(t, len(txs), txmp.Size())
	require.Equal(t, int64(5690), txmp.SizeBytes())

	rawTxs := make([]types.Tx, len(txs))
	for i, tx := range txs {
		rawTxs[i] = tx.tx
	}

	responses := make([]*abci.ResponseDeliverTx, len(rawTxs[:50]))
	for i := 0; i < len(responses); i++ {
		responses[i] = &abci.ResponseDeliverTx{Code: abci.CodeTypeOK}
	}

	txmp.Lock()
	require.NoError(t, txmp.Update(1, rawTxs[:50], responses, nil, nil))
	txmp.Unlock()

	require.Equal(t, len(rawTxs)/2, txmp.Size())
	require.Equal(t, int64(2850), txmp.SizeBytes())
}

func TestTxMempool_Flush(t *testing.T) {
	txmp := setup(t, 0)
	txs := checkTxs(t, txmp, 100, 0)
	require.Equal(t, len(txs), txmp.Size())
	require.Equal(t, int64(5690), txmp.SizeBytes())

	rawTxs := make([]types.Tx, len(txs))
	for i, tx := range txs {
		rawTxs[i] = tx.tx
	}

	responses := make([]*abci.ResponseDeliverTx, len(rawTxs[:50]))
	for i := 0; i < len(responses); i++ {
		responses[i] = &abci.ResponseDeliverTx{Code: abci.CodeTypeOK}
	}

	txmp.Lock()
	require.NoError(t, txmp.Update(1, rawTxs[:50], responses, nil, nil))
	txmp.Unlock()

	txmp.Flush()
	require.Zero(t, txmp.Size())
	require.Equal(t, int64(0), txmp.SizeBytes())
}

func TestTxMempool_ReapMaxBytesMaxGas(t *testing.T) {
	txmp := setup(t, 0)
	tTxs := checkTxs(t, txmp, 100, 0) // all txs request 1 gas unit
	require.Equal(t, len(tTxs), txmp.Size())
	require.Equal(t, int64(5690), txmp.SizeBytes())

	txMap := make(map[types.TxKey]testTx)
	priorities := make([]int64, len(tTxs))
	for i, tTx := range tTxs {
		txMap[tTx.tx.Key()] = tTx
		priorities[i] = tTx.priority
	}

	sort.Slice(priorities, func(i, j int) bool {
		// sort by priority, i.e. decreasing order
		return priorities[i] > priorities[j]
	})

	ensurePrioritized := func(reapedTxs types.Txs) {
		reapedPriorities := make([]int64, len(reapedTxs))
		for i, rTx := range reapedTxs {
			reapedPriorities[i] = txMap[rTx.Key()].priority
		}

		require.Equal(t, priorities[:len(reapedPriorities)], reapedPriorities)
	}

	// reap by gas capacity only
	reapedTxs := txmp.ReapMaxBytesMaxGas(-1, 50)
	ensurePrioritized(reapedTxs)
	require.Equal(t, len(tTxs), txmp.Size())
	require.Equal(t, int64(5690), txmp.SizeBytes())
	require.Len(t, reapedTxs, 50)

	// reap by transaction bytes only
	reapedTxs = txmp.ReapMaxBytesMaxGas(1000, -1)
	ensurePrioritized(reapedTxs)
	require.Equal(t, len(tTxs), txmp.Size())
	require.Equal(t, int64(5690), txmp.SizeBytes())
	require.GreaterOrEqual(t, len(reapedTxs), 16)

	// Reap by both transaction bytes and gas, where the size yields 31 reaped
	// transactions and the gas limit reaps 25 transactions.
	reapedTxs = txmp.ReapMaxBytesMaxGas(1500, 30)
	ensurePrioritized(reapedTxs)
	require.Equal(t, len(tTxs), txmp.Size())
	require.Equal(t, int64(5690), txmp.SizeBytes())
	require.Len(t, reapedTxs, 25)
}

func TestTxMempool_ReapMaxTxs(t *testing.T) {
	txmp := setup(t, 0)
	tTxs := checkTxs(t, txmp, 100, 0)
	require.Equal(t, len(tTxs), txmp.Size())
	require.Equal(t, int64(5690), txmp.SizeBytes())

	txMap := make(map[types.TxKey]testTx)
	priorities := make([]int64, len(tTxs))
	for i, tTx := range tTxs {
		txMap[tTx.tx.Key()] = tTx
		priorities[i] = tTx.priority
	}

	sort.Slice(priorities, func(i, j int) bool {
		// sort by priority, i.e. decreasing order
		return priorities[i] > priorities[j]
	})

	ensurePrioritized := func(reapedTxs types.Txs) {
		reapedPriorities := make([]int64, len(reapedTxs))
		for i, rTx := range reapedTxs {
			reapedPriorities[i] = txMap[rTx.Key()].priority
		}

		require.Equal(t, priorities[:len(reapedPriorities)], reapedPriorities)
	}

	// reap all transactions
	reapedTxs := txmp.ReapMaxTxs(-1)
	ensurePrioritized(reapedTxs)
	require.Equal(t, len(tTxs), txmp.Size())
	require.Equal(t, int64(5690), txmp.SizeBytes())
	require.Len(t, reapedTxs, len(tTxs))

	// reap a single transaction
	reapedTxs = txmp.ReapMaxTxs(1)
	ensurePrioritized(reapedTxs)
	require.Equal(t, len(tTxs), txmp.Size())
	require.Equal(t, int64(5690), txmp.SizeBytes())
	require.Len(t, reapedTxs, 1)

	// reap half of the transactions
	reapedTxs = txmp.ReapMaxTxs(len(tTxs) / 2)
	ensurePrioritized(reapedTxs)
	require.Equal(t, len(tTxs), txmp.Size())
	require.Equal(t, int64(5690), txmp.SizeBytes())
	require.Len(t, reapedTxs, len(tTxs)/2)
}

func TestTxMempool_CheckTxExceedsMaxSize(t *testing.T) {
	txmp := setup(t, 0)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	tx := make([]byte, txmp.config.MaxTxBytes+1)
	_, err := rng.Read(tx)
	require.NoError(t, err)

	require.Error(t, txmp.CheckTx(tx, nil, TxInfo{SenderID: 0}))

	tx = make([]byte, txmp.config.MaxTxBytes-1)
	_, err = rng.Read(tx)
	require.NoError(t, err)

	require.NoError(t, txmp.CheckTx(tx, nil, TxInfo{SenderID: 0}))
}

func TestTxMempool_CheckTxSamePeer(t *testing.T) {
	txmp := setup(t, 100)
	peerID := uint16(1)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	prefix := make([]byte, 20)
	_, err := rng.Read(prefix)
	require.NoError(t, err)

	tx := []byte(fmt.Sprintf("sender-0=%X=%d", prefix, 50))

	require.NoError(t, txmp.CheckTx(tx, nil, TxInfo{SenderID: peerID}))
	require.Error(t, txmp.CheckTx(tx, nil, TxInfo{SenderID: peerID}))
}

func TestTxMempool_CheckTxSameSender(t *testing.T) {
	txmp := setup(t, 100)
	peerID := uint16(1)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	prefix1 := make([]byte, 20)
	_, err := rng.Read(prefix1)
	require.NoError(t, err)

	prefix2 := make([]byte, 20)
	_, err = rng.Read(prefix2)
	require.NoError(t, err)

	tx1 := []byte(fmt.Sprintf("sender-0=%X=%d", prefix1, 50))
	tx2 := []byte(fmt.Sprintf("sender-0=%X=%d", prefix2, 50))

	require.NoError(t, txmp.CheckTx(tx1, nil, TxInfo{SenderID: peerID}))
	require.Equal(t, 1, txmp.Size())
	require.NoError(t, txmp.CheckTx(tx2, nil, TxInfo{SenderID: peerID}))
	require.Equal(t, 1, txmp.Size())
}

func TestTxMempool_ConcurrentTxs(t *testing.T) {
	txmp := setup(t, 100)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	checkTxDone := make(chan struct{})

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		for i := 0; i < 20; i++ {
			_ = checkTxs(t, txmp, 100, 0)
			dur := rng.Intn(1000-500) + 500
			time.Sleep(time.Duration(dur) * time.Millisecond)
		}

		wg.Done()
		close(checkTxDone)
	}()

	wg.Add(1)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		defer wg.Done()

		var height int64 = 1

		for range ticker.C {
			reapedTxs := txmp.ReapMaxTxs(200)
			if len(reapedTxs) > 0 {
				responses := make([]*abci.ResponseDeliverTx, len(reapedTxs))
				for i := 0; i < len(responses); i++ {
					var code uint32

					if i%10 == 0 {
						code = 100
					} else {
						code = abci.CodeTypeOK
					}

					responses[i] = &abci.ResponseDeliverTx{Code: code}
				}

				txmp.Lock()
				require.NoError(t, txmp.Update(height, reapedTxs, responses, nil, nil))
				txmp.Unlock()

				height++
			} else {
				// only return once we know we finished the CheckTx loop
				select {
				case <-checkTxDone:
					return
				default:
				}
			}
		}
	}()

	wg.Wait()
	require.Zero(t, txmp.Size())
	require.Zero(t, txmp.SizeBytes())
}

func TestTxMempool_ExpiredTxs_NumBlocks(t *testing.T) {
	txmp := setup(t, 500)
	txmp.height = 100
	txmp.config.TTLNumBlocks = 10

	tTxs := checkTxs(t, txmp, 100, 0)
	require.Equal(t, len(tTxs), txmp.Size())
	require.Equal(t, 100, txmp.heightIndex.Size())

	// reap 5 txs at the next height -- no txs should expire
	reapedTxs := txmp.ReapMaxTxs(5)
	responses := make([]*abci.ResponseDeliverTx, len(reapedTxs))
	for i := 0; i < len(responses); i++ {
		responses[i] = &abci.ResponseDeliverTx{Code: abci.CodeTypeOK}
	}

	txmp.Lock()
	require.NoError(t, txmp.Update(txmp.height+1, reapedTxs, responses, nil, nil))
	txmp.Unlock()

	require.Equal(t, 95, txmp.Size())
	require.Equal(t, 95, txmp.heightIndex.Size())

	// check more txs at height 101
	_ = checkTxs(t, txmp, 50, 1)
	require.Equal(t, 145, txmp.Size())
	require.Equal(t, 145, txmp.heightIndex.Size())

	// Reap 5 txs at a height that would expire all the transactions from before
	// the previous Update (height 100).
	//
	// NOTE: When we reap txs below, we do not know if we're picking txs from the
	// initial CheckTx calls or from the second round of CheckTx calls. Thus, we
	// cannot guarantee that all 95 txs are remaining that should be expired and
	// removed. However, we do know that that at most 95 txs can be expired and
	// removed.
	reapedTxs = txmp.ReapMaxTxs(5)
	responses = make([]*abci.ResponseDeliverTx, len(reapedTxs))
	for i := 0; i < len(responses); i++ {
		responses[i] = &abci.ResponseDeliverTx{Code: abci.CodeTypeOK}
	}

	txmp.Lock()
	require.NoError(t, txmp.Update(txmp.height+10, reapedTxs, responses, nil, nil))
	txmp.Unlock()

	require.GreaterOrEqual(t, txmp.Size(), 45)
	require.GreaterOrEqual(t, txmp.heightIndex.Size(), 45)
}

func TestTxMempool_CheckTxPostCheckError(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{
			name: "error",
			err:  errors.New("test error"),
		},
		{
			name: "no error",
			err:  nil,
		},
	}
	for _, tc := range cases {
		testCase := tc
		t.Run(testCase.name, func(t *testing.T) {
			postCheckFn := func(_ types.Tx, _ *abci.ResponseCheckTx) error {
				return testCase.err
			}
			txmp := setup(t, 0, WithPostCheck(postCheckFn))
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			tx := make([]byte, txmp.config.MaxTxBytes-1)
			_, err := rng.Read(tx)
			require.NoError(t, err)

			callback := func(res *abci.Response) {
				checkTxRes, ok := res.Value.(*abci.Response_CheckTx)
				require.True(t, ok)
				expectedErrString := ""
				if testCase.err != nil {
					expectedErrString = testCase.err.Error()
				}
				require.Equal(t, expectedErrString, checkTxRes.CheckTx.MempoolError)
			}
			require.NoError(t, txmp.CheckTx(tx, callback, TxInfo{SenderID: 0}))
		})
	}
}

// // A cleanupFunc cleans up any config / test files created for a particular
// // test.
// type cleanupFunc func()

// func newMempoolWithApp(cc proxy.ClientCreator) (*CListMempool, cleanupFunc) {
// 	return newMempoolWithAppAndConfig(cc, cfg.ResetTestRoot("mempool_test"))
// }

// func newMempoolWithAppAndConfig(cc proxy.ClientCreator, config *cfg.Config) (*CListMempool, cleanupFunc) {
// 	appConnMem, _ := cc.NewABCIClient()
// 	appConnMem.SetLogger(log.TestingLogger().With("module", "abci-client", "connection", "mempool"))
// 	err := appConnMem.Start()
// 	if err != nil {
// 		panic(err)
// 	}
// 	mempool := NewCListMempool(config.Mempool, appConnMem, 0)
// 	mempool.SetLogger(log.TestingLogger())
// 	return mempool, func() { os.RemoveAll(config.RootDir) }
// }

// func ensureNoFire(t *testing.T, ch <-chan struct{}, timeoutMS int) {
// 	timer := time.NewTimer(time.Duration(timeoutMS) * time.Millisecond)
// 	select {
// 	case <-ch:
// 		t.Fatal("Expected not to fire")
// 	case <-timer.C:
// 	}
// }

// func ensureFire(t *testing.T, ch <-chan struct{}, timeoutMS int) {
// 	timer := time.NewTimer(time.Duration(timeoutMS) * time.Millisecond)
// 	select {
// 	case <-ch:
// 	case <-timer.C:
// 		t.Fatal("Expected to fire")
// 	}
// }

// func checkTxs(t *testing.T, mempool Mempool, count int, peerID uint16) types.Txs {
// 	txs := make(types.Txs, count)
// 	txInfo := TxInfo{SenderID: peerID}
// 	for i := 0; i < count; i++ {
// 		txBytes := make([]byte, 20)
// 		txs[i] = txBytes
// 		_, err := rand.Read(txBytes)
// 		if err != nil {
// 			t.Error(err)
// 		}
// 		if err := mempool.CheckTx(txBytes, nil, txInfo); err != nil {
// 			// Skip invalid txs.
// 			// TestMempoolFilters will fail otherwise. It asserts a number of txs
// 			// returned.
// 			if IsPreCheckError(err) {
// 				continue
// 			}
// 			t.Fatalf("CheckTx failed: %v while checking #%d tx", err, i)
// 		}
// 	}
// 	return txs
// }

// func TestReapMaxBytesMaxGas(t *testing.T) {
// 	app := kvstore.NewApplication()
// 	cc := proxy.NewLocalClientCreator(app)
// 	mempool, cleanup := newMempoolWithApp(cc)
// 	defer cleanup()

// 	// Ensure gas calculation behaves as expected
// 	checkTxs(t, mempool, 1, UnknownPeerID)
// 	tx0 := mempool.TxsFront().Value.(*mempoolTx)
// 	// assert that kv store has gas wanted = 1.
// 	require.Equal(t, app.CheckTx(abci.RequestCheckTx{Tx: tx0.tx}).GasWanted, int64(1), "KVStore had a gas value neq to 1")
// 	require.Equal(t, tx0.gasWanted, int64(1), "transactions gas was set incorrectly")
// 	// ensure each tx is 20 bytes long
// 	require.Equal(t, len(tx0.tx), 20, "Tx is longer than 20 bytes")
// 	mempool.Flush()

// 	// each table driven test creates numTxsToCreate txs with checkTx, and at the end clears all remaining txs.
// 	// each tx has 20 bytes
// 	tests := []struct {
// 		numTxsToCreate int
// 		maxBytes       int64
// 		maxGas         int64
// 		expectedNumTxs int
// 	}{
// 		{20, -1, -1, 20},
// 		{20, -1, 0, 0},
// 		{20, -1, 10, 10},
// 		{20, -1, 30, 20},
// 		{20, 0, -1, 0},
// 		{20, 0, 10, 0},
// 		{20, 10, 10, 0},
// 		{20, 24, 10, 1},
// 		{20, 240, 5, 5},
// 		{20, 240, -1, 10},
// 		{20, 240, 10, 10},
// 		{20, 240, 15, 10},
// 		{20, 20000, -1, 20},
// 		{20, 20000, 5, 5},
// 		{20, 20000, 30, 20},
// 	}
// 	for tcIndex, tt := range tests {
// 		checkTxs(t, mempool, tt.numTxsToCreate, UnknownPeerID)
// 		got := mempool.ReapMaxBytesMaxGas(tt.maxBytes, tt.maxGas)
// 		assert.Equal(t, tt.expectedNumTxs, len(got), "Got %d txs, expected %d, tc #%d",
// 			len(got), tt.expectedNumTxs, tcIndex)
// 		mempool.Flush()
// 	}
// }

// func TestMempoolFilters(t *testing.T) {
// 	app := kvstore.NewApplication()
// 	cc := proxy.NewLocalClientCreator(app)
// 	mempool, cleanup := newMempoolWithApp(cc)
// 	defer cleanup()
// 	emptyTxArr := []types.Tx{[]byte{}}

// 	nopPreFilter := func(tx types.Tx) error { return nil }
// 	nopPostFilter := func(tx types.Tx, res *abci.ResponseCheckTx) error { return nil }

// 	// each table driven test creates numTxsToCreate txs with checkTx, and at the end clears all remaining txs.
// 	// each tx has 20 bytes
// 	tests := []struct {
// 		numTxsToCreate int
// 		preFilter      PreCheckFunc
// 		postFilter     PostCheckFunc
// 		expectedNumTxs int
// 	}{
// 		{10, nopPreFilter, nopPostFilter, 10},
// 		{10, PreCheckMaxBytes(10), nopPostFilter, 0},
// 		{10, PreCheckMaxBytes(22), nopPostFilter, 10},
// 		{10, nopPreFilter, PostCheckMaxGas(-1), 10},
// 		{10, nopPreFilter, PostCheckMaxGas(0), 0},
// 		{10, nopPreFilter, PostCheckMaxGas(1), 10},
// 		{10, nopPreFilter, PostCheckMaxGas(3000), 10},
// 		{10, PreCheckMaxBytes(10), PostCheckMaxGas(20), 0},
// 		{10, PreCheckMaxBytes(30), PostCheckMaxGas(20), 10},
// 		{10, PreCheckMaxBytes(22), PostCheckMaxGas(1), 10},
// 		{10, PreCheckMaxBytes(22), PostCheckMaxGas(0), 0},
// 	}
// 	for tcIndex, tt := range tests {
// 		err := mempool.Update(1, emptyTxArr, abciResponses(len(emptyTxArr), abci.CodeTypeOK), tt.preFilter, tt.postFilter)
// 		require.NoError(t, err)
// 		checkTxs(t, mempool, tt.numTxsToCreate, UnknownPeerID)
// 		require.Equal(t, tt.expectedNumTxs, mempool.Size(), "mempool had the incorrect size, on test case %d", tcIndex)
// 		mempool.Flush()
// 	}
// }

// func TestMempoolUpdate(t *testing.T) {
// 	app := kvstore.NewApplication()
// 	cc := proxy.NewLocalClientCreator(app)
// 	mempool, cleanup := newMempoolWithApp(cc)
// 	defer cleanup()

// 	// 1. Adds valid txs to the cache
// 	{
// 		err := mempool.Update(1, []types.Tx{[]byte{0x01}}, abciResponses(1, abci.CodeTypeOK), nil, nil)
// 		require.NoError(t, err)
// 		err = mempool.CheckTx([]byte{0x01}, nil, TxInfo{})
// 		if assert.Error(t, err) {
// 			assert.Equal(t, ErrTxInCache, err)
// 		}
// 	}

// 	// 2. Removes valid txs from the mempool
// 	{
// 		err := mempool.CheckTx([]byte{0x02}, nil, TxInfo{})
// 		require.NoError(t, err)
// 		err = mempool.Update(1, []types.Tx{[]byte{0x02}}, abciResponses(1, abci.CodeTypeOK), nil, nil)
// 		require.NoError(t, err)
// 		assert.Zero(t, mempool.Size())
// 	}

// 	// 3. Removes invalid transactions from the cache and the mempool (if present)
// 	{
// 		err := mempool.CheckTx([]byte{0x03}, nil, TxInfo{})
// 		require.NoError(t, err)
// 		err = mempool.Update(1, []types.Tx{[]byte{0x03}}, abciResponses(1, 1), nil, nil)
// 		require.NoError(t, err)
// 		assert.Zero(t, mempool.Size())

// 		err = mempool.CheckTx([]byte{0x03}, nil, TxInfo{})
// 		require.NoError(t, err)
// 	}
// }

// func TestMempool_KeepInvalidTxsInCache(t *testing.T) {
// 	app := counter.NewApplication(true)
// 	cc := proxy.NewLocalClientCreator(app)
// 	wcfg := cfg.DefaultConfig()
// 	wcfg.Mempool.KeepInvalidTxsInCache = true
// 	mempool, cleanup := newMempoolWithAppAndConfig(cc, wcfg)
// 	defer cleanup()

// 	// 1. An invalid transaction must remain in the cache after Update
// 	{
// 		a := make([]byte, 8)
// 		binary.BigEndian.PutUint64(a, 0)

// 		b := make([]byte, 8)
// 		binary.BigEndian.PutUint64(b, 1)

// 		err := mempool.CheckTx(b, nil, TxInfo{})
// 		require.NoError(t, err)

// 		// simulate new block
// 		_ = app.DeliverTx(abci.RequestDeliverTx{Tx: a})
// 		_ = app.DeliverTx(abci.RequestDeliverTx{Tx: b})
// 		err = mempool.Update(1, []types.Tx{a, b},
// 			[]*abci.ResponseDeliverTx{{Code: abci.CodeTypeOK}, {Code: 2}}, nil, nil)
// 		require.NoError(t, err)

// 		// a must be added to the cache
// 		err = mempool.CheckTx(a, nil, TxInfo{})
// 		if assert.Error(t, err) {
// 			assert.Equal(t, ErrTxInCache, err)
// 		}

// 		// b must remain in the cache
// 		err = mempool.CheckTx(b, nil, TxInfo{})
// 		if assert.Error(t, err) {
// 			assert.Equal(t, ErrTxInCache, err)
// 		}
// 	}

// 	// 2. An invalid transaction must remain in the cache
// 	{
// 		a := make([]byte, 8)
// 		binary.BigEndian.PutUint64(a, 0)

// 		// remove a from the cache to test (2)
// 		mempool.cache.Remove(a)

// 		err := mempool.CheckTx(a, nil, TxInfo{})
// 		require.NoError(t, err)

// 		err = mempool.CheckTx(a, nil, TxInfo{})
// 		if assert.Error(t, err) {
// 			assert.Equal(t, ErrTxInCache, err)
// 		}
// 	}
// }

// func TestTxsAvailable(t *testing.T) {
// 	app := kvstore.NewApplication()
// 	cc := proxy.NewLocalClientCreator(app)
// 	mempool, cleanup := newMempoolWithApp(cc)
// 	defer cleanup()
// 	mempool.EnableTxsAvailable()

// 	timeoutMS := 500

// 	// with no txs, it shouldnt fire
// 	ensureNoFire(t, mempool.TxsAvailable(), timeoutMS)

// 	// send a bunch of txs, it should only fire once
// 	txs := checkTxs(t, mempool, 100, UnknownPeerID)
// 	ensureFire(t, mempool.TxsAvailable(), timeoutMS)
// 	ensureNoFire(t, mempool.TxsAvailable(), timeoutMS)

// 	// call update with half the txs.
// 	// it should fire once now for the new height
// 	// since there are still txs left
// 	committedTxs, txs := txs[:50], txs[50:]
// 	if err := mempool.Update(1, committedTxs, abciResponses(len(committedTxs), abci.CodeTypeOK), nil, nil); err != nil {
// 		t.Error(err)
// 	}
// 	ensureFire(t, mempool.TxsAvailable(), timeoutMS)
// 	ensureNoFire(t, mempool.TxsAvailable(), timeoutMS)

// 	// send a bunch more txs. we already fired for this height so it shouldnt fire again
// 	moreTxs := checkTxs(t, mempool, 50, UnknownPeerID)
// 	ensureNoFire(t, mempool.TxsAvailable(), timeoutMS)

// 	// now call update with all the txs. it should not fire as there are no txs left
// 	committedTxs = append(txs, moreTxs...) //nolint: gocritic
// 	if err := mempool.Update(2, committedTxs, abciResponses(len(committedTxs), abci.CodeTypeOK), nil, nil); err != nil {
// 		t.Error(err)
// 	}
// 	ensureNoFire(t, mempool.TxsAvailable(), timeoutMS)

// 	// send a bunch more txs, it should only fire once
// 	checkTxs(t, mempool, 100, UnknownPeerID)
// 	ensureFire(t, mempool.TxsAvailable(), timeoutMS)
// 	ensureNoFire(t, mempool.TxsAvailable(), timeoutMS)
// }

// func TestSerialReap(t *testing.T) {
// 	app := counter.NewApplication(true)
// 	app.SetOption(abci.RequestSetOption{Key: "serial", Value: "on"})
// 	cc := proxy.NewLocalClientCreator(app)

// 	mempool, cleanup := newMempoolWithApp(cc)
// 	defer cleanup()

// 	appConnCon, _ := cc.NewABCIClient()
// 	appConnCon.SetLogger(log.TestingLogger().With("module", "abci-client", "connection", "consensus"))
// 	err := appConnCon.Start()
// 	require.Nil(t, err)

// 	cacheMap := make(map[string]struct{})
// 	deliverTxsRange := func(start, end int) {
// 		// Deliver some txs.
// 		for i := start; i < end; i++ {

// 			// This will succeed
// 			txBytes := make([]byte, 8)
// 			binary.BigEndian.PutUint64(txBytes, uint64(i))
// 			err := mempool.CheckTx(txBytes, nil, TxInfo{})
// 			_, cached := cacheMap[string(txBytes)]
// 			if cached {
// 				require.NotNil(t, err, "expected error for cached tx")
// 			} else {
// 				require.Nil(t, err, "expected no err for uncached tx")
// 			}
// 			cacheMap[string(txBytes)] = struct{}{}

// 			// Duplicates are cached and should return error
// 			err = mempool.CheckTx(txBytes, nil, TxInfo{})
// 			require.NotNil(t, err, "Expected error after CheckTx on duplicated tx")
// 		}
// 	}

// 	reapCheck := func(exp int) {
// 		txs := mempool.ReapMaxBytesMaxGas(-1, -1)
// 		require.Equal(t, len(txs), exp, fmt.Sprintf("Expected to reap %v txs but got %v", exp, len(txs)))
// 	}

// 	updateRange := func(start, end int) {
// 		txs := make([]types.Tx, 0)
// 		for i := start; i < end; i++ {
// 			txBytes := make([]byte, 8)
// 			binary.BigEndian.PutUint64(txBytes, uint64(i))
// 			txs = append(txs, txBytes)
// 		}
// 		if err := mempool.Update(0, txs, abciResponses(len(txs), abci.CodeTypeOK), nil, nil); err != nil {
// 			t.Error(err)
// 		}
// 	}

// 	commitRange := func(start, end int) {
// 		// Deliver some txs.
// 		for i := start; i < end; i++ {
// 			txBytes := make([]byte, 8)
// 			binary.BigEndian.PutUint64(txBytes, uint64(i))
// 			res, err := appConnCon.DeliverTxSync(abci.RequestDeliverTx{Tx: txBytes})
// 			if err != nil {
// 				t.Errorf("client error committing tx: %v", err)
// 			}
// 			if res.IsErr() {
// 				t.Errorf("error committing tx. Code:%v result:%X log:%v",
// 					res.Code, res.Data, res.Log)
// 			}
// 		}
// 		res, err := appConnCon.CommitSync()
// 		if err != nil {
// 			t.Errorf("client error committing: %v", err)
// 		}
// 		if len(res.Data) != 8 {
// 			t.Errorf("error committing. Hash:%X", res.Data)
// 		}
// 	}

// 	//----------------------------------------

// 	// Deliver some txs.
// 	deliverTxsRange(0, 100)

// 	// Reap the txs.
// 	reapCheck(100)

// 	// Reap again.  We should get the same amount
// 	reapCheck(100)

// 	// Deliver 0 to 999, we should reap 900 new txs
// 	// because 100 were already counted.
// 	deliverTxsRange(0, 1000)

// 	// Reap the txs.
// 	reapCheck(1000)

// 	// Reap again.  We should get the same amount
// 	reapCheck(1000)

// 	// Commit from the conensus AppConn
// 	commitRange(0, 500)
// 	updateRange(0, 500)

// 	// We should have 500 left.
// 	reapCheck(500)

// 	// Deliver 100 invalid txs and 100 valid txs
// 	deliverTxsRange(900, 1100)

// 	// We should have 600 now.
// 	reapCheck(600)
// }

// func TestMempoolCloseWAL(t *testing.T) {
// 	// 1. Create the temporary directory for mempool and WAL testing.
// 	rootDir, err := ioutil.TempDir("", "mempool-test")
// 	require.Nil(t, err, "expecting successful tmpdir creation")

// 	// 2. Ensure that it doesn't contain any elements -- Sanity check
// 	m1, err := filepath.Glob(filepath.Join(rootDir, "*"))
// 	require.Nil(t, err, "successful globbing expected")
// 	require.Equal(t, 0, len(m1), "no matches yet")

// 	// 3. Create the mempool
// 	wcfg := cfg.DefaultConfig()
// 	wcfg.Mempool.RootDir = rootDir
// 	app := kvstore.NewApplication()
// 	cc := proxy.NewLocalClientCreator(app)
// 	mempool, cleanup := newMempoolWithAppAndConfig(cc, wcfg)
// 	defer cleanup()
// 	mempool.height = 10
// 	err = mempool.InitWAL()
// 	require.NoError(t, err)

// 	// 4. Ensure that the directory contains the WAL file
// 	m2, err := filepath.Glob(filepath.Join(rootDir, "*"))
// 	require.Nil(t, err, "successful globbing expected")
// 	require.Equal(t, 1, len(m2), "expecting the wal match in")

// 	// 5. Write some contents to the WAL
// 	err = mempool.CheckTx(types.Tx([]byte("foo")), nil, TxInfo{})
// 	require.NoError(t, err)
// 	walFilepath := mempool.wal.Path
// 	sum1 := checksumFile(walFilepath, t)

// 	// 6. Sanity check to ensure that the written TX matches the expectation.
// 	require.Equal(t, sum1, checksumIt([]byte("foo\n")), "foo with a newline should be written")

// 	// 7. Invoke CloseWAL() and ensure it discards the
// 	// WAL thus any other write won't go through.
// 	mempool.CloseWAL()
// 	err = mempool.CheckTx(types.Tx([]byte("bar")), nil, TxInfo{})
// 	require.NoError(t, err)
// 	sum2 := checksumFile(walFilepath, t)
// 	require.Equal(t, sum1, sum2, "expected no change to the WAL after invoking CloseWAL() since it was discarded")

// 	// 8. Sanity check to ensure that the WAL file still exists
// 	m3, err := filepath.Glob(filepath.Join(rootDir, "*"))
// 	require.Nil(t, err, "successful globbing expected")
// 	require.Equal(t, 1, len(m3), "expecting the wal match in")
// }

// func TestMempool_CheckTxChecksTxSize(t *testing.T) {
// 	app := kvstore.NewApplication()
// 	cc := proxy.NewLocalClientCreator(app)
// 	mempl, cleanup := newMempoolWithApp(cc)
// 	defer cleanup()

// 	maxTxSize := mempl.config.MaxTxBytes

// 	testCases := []struct {
// 		len int
// 		err bool
// 	}{
// 		// check small txs. no error
// 		0: {10, false},
// 		1: {1000, false},
// 		2: {1000000, false},

// 		// check around maxTxSize
// 		3: {maxTxSize - 1, false},
// 		4: {maxTxSize, false},
// 		5: {maxTxSize + 1, true},
// 	}

// 	for i, testCase := range testCases {
// 		caseString := fmt.Sprintf("case %d, len %d", i, testCase.len)

// 		tx := tmrand.Bytes(testCase.len)

// 		err := mempl.CheckTx(tx, nil, TxInfo{})
// 		bv := gogotypes.BytesValue{Value: tx}
// 		bz, err2 := bv.Marshal()
// 		require.NoError(t, err2)
// 		require.Equal(t, len(bz), proto.Size(&bv), caseString)

// 		if !testCase.err {
// 			require.NoError(t, err, caseString)
// 		} else {
// 			require.Equal(t, err, ErrTxTooLarge{maxTxSize, testCase.len}, caseString)
// 		}
// 	}
// }

// func TestMempoolTxsBytes(t *testing.T) {
// 	app := kvstore.NewApplication()
// 	cc := proxy.NewLocalClientCreator(app)
// 	config := cfg.ResetTestRoot("mempool_test")
// 	config.Mempool.MaxTxsBytes = 10
// 	mempool, cleanup := newMempoolWithAppAndConfig(cc, config)
// 	defer cleanup()

// 	// 1. zero by default
// 	assert.EqualValues(t, 0, mempool.TxsBytes())

// 	// 2. len(tx) after CheckTx
// 	err := mempool.CheckTx([]byte{0x01}, nil, TxInfo{})
// 	require.NoError(t, err)
// 	assert.EqualValues(t, 1, mempool.TxsBytes())

// 	// 3. zero again after tx is removed by Update
// 	err = mempool.Update(1, []types.Tx{[]byte{0x01}}, abciResponses(1, abci.CodeTypeOK), nil, nil)
// 	require.NoError(t, err)
// 	assert.EqualValues(t, 0, mempool.TxsBytes())

// 	// 4. zero after Flush
// 	err = mempool.CheckTx([]byte{0x02, 0x03}, nil, TxInfo{})
// 	require.NoError(t, err)
// 	assert.EqualValues(t, 2, mempool.TxsBytes())

// 	mempool.Flush()
// 	assert.EqualValues(t, 0, mempool.TxsBytes())

// 	// 5. ErrMempoolIsFull is returned when/if MaxTxsBytes limit is reached.
// 	err = mempool.CheckTx([]byte{0x04, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04}, nil, TxInfo{})
// 	require.NoError(t, err)
// 	err = mempool.CheckTx([]byte{0x05}, nil, TxInfo{})
// 	if assert.Error(t, err) {
// 		assert.IsType(t, ErrMempoolIsFull{}, err)
// 	}

// 	// 6. zero after tx is rechecked and removed due to not being valid anymore
// 	app2 := counter.NewApplication(true)
// 	cc = proxy.NewLocalClientCreator(app2)
// 	mempool, cleanup = newMempoolWithApp(cc)
// 	defer cleanup()

// 	txBytes := make([]byte, 8)
// 	binary.BigEndian.PutUint64(txBytes, uint64(0))

// 	err = mempool.CheckTx(txBytes, nil, TxInfo{})
// 	require.NoError(t, err)
// 	assert.EqualValues(t, 8, mempool.TxsBytes())

// 	appConnCon, _ := cc.NewABCIClient()
// 	appConnCon.SetLogger(log.TestingLogger().With("module", "abci-client", "connection", "consensus"))
// 	err = appConnCon.Start()
// 	require.Nil(t, err)
// 	t.Cleanup(func() {
// 		if err := appConnCon.Stop(); err != nil {
// 			t.Error(err)
// 		}
// 	})
// 	res, err := appConnCon.DeliverTxSync(abci.RequestDeliverTx{Tx: txBytes})
// 	require.NoError(t, err)
// 	require.EqualValues(t, 0, res.Code)
// 	res2, err := appConnCon.CommitSync()
// 	require.NoError(t, err)
// 	require.NotEmpty(t, res2.Data)

// 	// Pretend like we committed nothing so txBytes gets rechecked and removed.
// 	err = mempool.Update(1, []types.Tx{}, abciResponses(0, abci.CodeTypeOK), nil, nil)
// 	require.NoError(t, err)
// 	assert.EqualValues(t, 0, mempool.TxsBytes())

// 	// 7. Test RemoveTxByKey function
// 	err = mempool.CheckTx([]byte{0x06}, nil, TxInfo{})
// 	require.NoError(t, err)
// 	assert.EqualValues(t, 1, mempool.TxsBytes())
// 	mempool.RemoveTxByKey(TxKey([]byte{0x07}), true)
// 	assert.EqualValues(t, 1, mempool.TxsBytes())
// 	mempool.RemoveTxByKey(TxKey([]byte{0x06}), true)
// 	assert.EqualValues(t, 0, mempool.TxsBytes())

// }

// // This will non-deterministically catch some concurrency failures like
// // https://github.com/tendermint/tendermint/issues/3509
// // TODO: all of the tests should probably also run using the remote proxy app
// // since otherwise we're not actually testing the concurrency of the mempool here!
// func TestMempoolRemoteAppConcurrency(t *testing.T) {
// 	sockPath := fmt.Sprintf("unix:///tmp/echo_%v.sock", tmrand.Str(6))
// 	app := kvstore.NewApplication()
// 	cc, server := newRemoteApp(t, sockPath, app)
// 	t.Cleanup(func() {
// 		if err := server.Stop(); err != nil {
// 			t.Error(err)
// 		}
// 	})
// 	config := cfg.ResetTestRoot("mempool_test")
// 	mempool, cleanup := newMempoolWithAppAndConfig(cc, config)
// 	defer cleanup()

// 	// generate small number of txs
// 	nTxs := 10
// 	txLen := 200
// 	txs := make([]types.Tx, nTxs)
// 	for i := 0; i < nTxs; i++ {
// 		txs[i] = tmrand.Bytes(txLen)
// 	}

// 	// simulate a group of peers sending them over and over
// 	N := config.Mempool.Size
// 	maxPeers := 5
// 	for i := 0; i < N; i++ {
// 		peerID := mrand.Intn(maxPeers)
// 		txNum := mrand.Intn(nTxs)
// 		tx := txs[txNum]

// 		// this will err with ErrTxInCache many times ...
// 		mempool.CheckTx(tx, nil, TxInfo{SenderID: uint16(peerID)}) //nolint: errcheck // will error
// 	}
// 	err := mempool.FlushAppConn()
// 	require.NoError(t, err)
// }

// // caller must close server
// func newRemoteApp(
// 	t *testing.T,
// 	addr string,
// 	app abci.Application,
// ) (
// 	clientCreator proxy.ClientCreator,
// 	server service.Service,
// ) {
// 	clientCreator = proxy.NewRemoteClientCreator(addr, "socket", true)

// 	// Start server
// 	server = abciserver.NewSocketServer(addr, app)
// 	server.SetLogger(log.TestingLogger().With("module", "abci-server"))
// 	if err := server.Start(); err != nil {
// 		t.Fatalf("Error starting socket server: %v", err.Error())
// 	}
// 	return clientCreator, server
// }
// func checksumIt(data []byte) string {
// 	h := sha256.New()
// 	h.Write(data)
// 	return fmt.Sprintf("%x", h.Sum(nil))
// }

// func checksumFile(p string, t *testing.T) string {
// 	data, err := ioutil.ReadFile(p)
// 	require.Nil(t, err, "expecting successful read of %q", p)
// 	return checksumIt(data)
// }

// func abciResponses(n int, code uint32) []*abci.ResponseDeliverTx {
// 	responses := make([]*abci.ResponseDeliverTx, 0, n)
// 	for i := 0; i < n; i++ {
// 		responses = append(responses, &abci.ResponseDeliverTx{Code: code})
// 	}
// 	return responses
// }
