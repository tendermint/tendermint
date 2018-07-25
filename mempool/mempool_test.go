package mempool

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tendermint/tendermint/abci/example/counter"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"

	"github.com/stretchr/testify/require"
)

func newMempoolWithApp(cc proxy.ClientCreator) *Mempool {
	config := cfg.ResetTestRoot("mempool_test")

	appConnMem, _ := cc.NewABCIClient()
	appConnMem.SetLogger(log.TestingLogger().With("module", "abci-client", "connection", "mempool"))
	err := appConnMem.Start()
	if err != nil {
		panic(err)
	}
	mempool := NewMempool(config.Mempool, appConnMem, 0)
	mempool.SetLogger(log.TestingLogger())
	return mempool
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

func checkTxs(t *testing.T, mempool *Mempool, count int) types.Txs {
	txs := make(types.Txs, count)
	for i := 0; i < count; i++ {
		txBytes := make([]byte, 20)
		txs[i] = txBytes
		_, err := rand.Read(txBytes)
		if err != nil {
			t.Error(err)
		}
		if err := mempool.CheckTx(txBytes, nil); err != nil {
			t.Fatalf("Error after CheckTx: %v", err)
		}
	}
	return txs
}

func TestTxsAvailable(t *testing.T) {
	app := kvstore.NewKVStoreApplication()
	cc := proxy.NewLocalClientCreator(app)
	mempool := newMempoolWithApp(cc)
	mempool.EnableTxsAvailable()

	timeoutMS := 500

	// with no txs, it shouldnt fire
	ensureNoFire(t, mempool.TxsAvailable(), timeoutMS)

	// send a bunch of txs, it should only fire once
	txs := checkTxs(t, mempool, 100)
	ensureFire(t, mempool.TxsAvailable(), timeoutMS)
	ensureNoFire(t, mempool.TxsAvailable(), timeoutMS)

	// call update with half the txs.
	// it should fire once now for the new height
	// since there are still txs left
	committedTxs, txs := txs[:50], txs[50:]
	if err := mempool.Update(1, committedTxs); err != nil {
		t.Error(err)
	}
	ensureFire(t, mempool.TxsAvailable(), timeoutMS)
	ensureNoFire(t, mempool.TxsAvailable(), timeoutMS)

	// send a bunch more txs. we already fired for this height so it shouldnt fire again
	moreTxs := checkTxs(t, mempool, 50)
	ensureNoFire(t, mempool.TxsAvailable(), timeoutMS)

	// now call update with all the txs. it should not fire as there are no txs left
	committedTxs = append(txs, moreTxs...)
	if err := mempool.Update(2, committedTxs); err != nil {
		t.Error(err)
	}
	ensureNoFire(t, mempool.TxsAvailable(), timeoutMS)

	// send a bunch more txs, it should only fire once
	checkTxs(t, mempool, 100)
	ensureFire(t, mempool.TxsAvailable(), timeoutMS)
	ensureNoFire(t, mempool.TxsAvailable(), timeoutMS)
}

func TestSerialReap(t *testing.T) {
	app := counter.NewCounterApplication(true)
	app.SetOption(abci.RequestSetOption{Key: "serial", Value: "on"})
	cc := proxy.NewLocalClientCreator(app)

	mempool := newMempoolWithApp(cc)
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
			err := mempool.CheckTx(txBytes, nil)
			_, cached := cacheMap[string(txBytes)]
			if cached {
				require.NotNil(t, err, "expected error for cached tx")
			} else {
				require.Nil(t, err, "expected no err for uncached tx")
			}
			cacheMap[string(txBytes)] = struct{}{}

			// Duplicates are cached and should return error
			err = mempool.CheckTx(txBytes, nil)
			require.NotNil(t, err, "Expected error after CheckTx on duplicated tx")
		}
	}

	reapCheck := func(exp int) {
		txs := mempool.Reap(-1)
		require.Equal(t, len(txs), exp, cmn.Fmt("Expected to reap %v txs but got %v", exp, len(txs)))
	}

	updateRange := func(start, end int) {
		txs := make([]types.Tx, 0)
		for i := start; i < end; i++ {
			txBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(txBytes, uint64(i))
			txs = append(txs, txBytes)
		}
		if err := mempool.Update(0, txs); err != nil {
			t.Error(err)
		}
	}

	commitRange := func(start, end int) {
		// Deliver some txs.
		for i := start; i < end; i++ {
			txBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(txBytes, uint64(i))
			res, err := appConnCon.DeliverTxSync(txBytes)
			if err != nil {
				t.Errorf("Client error committing tx: %v", err)
			}
			if res.IsErr() {
				t.Errorf("Error committing tx. Code:%v result:%X log:%v",
					res.Code, res.Data, res.Log)
			}
		}
		res, err := appConnCon.CommitSync()
		if err != nil {
			t.Errorf("Client error committing: %v", err)
		}
		if len(res.Data) != 8 {
			t.Errorf("Error committing. Hash:%X", res.Data)
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
	defer os.RemoveAll(rootDir)

	// 2. Ensure that it doesn't contain any elements -- Sanity check
	m1, err := filepath.Glob(filepath.Join(rootDir, "*"))
	require.Nil(t, err, "successful globbing expected")
	require.Equal(t, 0, len(m1), "no matches yet")

	// 3. Create the mempool
	wcfg := cfg.DefaultMempoolConfig()
	wcfg.RootDir = rootDir
	app := kvstore.NewKVStoreApplication()
	cc := proxy.NewLocalClientCreator(app)
	appConnMem, _ := cc.NewABCIClient()
	mempool := NewMempool(wcfg, appConnMem, 10)
	mempool.InitWAL()

	// 4. Ensure that the directory contains the WAL file
	m2, err := filepath.Glob(filepath.Join(rootDir, "*"))
	require.Nil(t, err, "successful globbing expected")
	require.Equal(t, 1, len(m2), "expecting the wal match in")

	// 5. Write some contents to the WAL
	mempool.CheckTx(types.Tx([]byte("foo")), nil)
	walFilepath := mempool.wal.Path
	sum1 := checksumFile(walFilepath, t)

	// 6. Sanity check to ensure that the written TX matches the expectation.
	require.Equal(t, sum1, checksumIt([]byte("foo\n")), "foo with a newline should be written")

	// 7. Invoke CloseWAL() and ensure it discards the
	// WAL thus any other write won't go through.
	require.True(t, mempool.CloseWAL(), "CloseWAL should CloseWAL")
	mempool.CheckTx(types.Tx([]byte("bar")), nil)
	sum2 := checksumFile(walFilepath, t)
	require.Equal(t, sum1, sum2, "expected no change to the WAL after invoking CloseWAL() since it was discarded")

	// 8. Second CloseWAL should do nothing
	require.False(t, mempool.CloseWAL(), "CloseWAL should CloseWAL")

	// 9. Sanity check to ensure that the WAL file still exists
	m3, err := filepath.Glob(filepath.Join(rootDir, "*"))
	require.Nil(t, err, "successful globbing expected")
	require.Equal(t, 1, len(m3), "expecting the wal match in")
}

func checksumIt(data []byte) string {
	h := md5.New()
	h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func checksumFile(p string, t *testing.T) string {
	data, err := ioutil.ReadFile(p)
	require.Nil(t, err, "expecting successful read of %q", p)
	return checksumIt(data)
}
