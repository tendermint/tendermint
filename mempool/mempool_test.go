package mempool

import (
	"encoding/binary"
	"sync"
	"testing"

	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tmsp/example/counter"
	tmsp "github.com/tendermint/tmsp/types"
)

func TestSerialReap(t *testing.T) {

	app := counter.NewCounterApplication(true)
	app.SetOption("serial", "on")
	mtx := new(sync.Mutex)
	appConnMem := proxy.NewLocalAppConn(mtx, app)
	appConnCon := proxy.NewLocalAppConn(mtx, app)
	mempool := NewMempool(appConnMem)

	appendTxsRange := func(start, end int) {
		// Append some txs.
		for i := start; i < end; i++ {

			// This will succeed
			txBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(txBytes, uint64(i))
			err := mempool.CheckTx(txBytes, nil)
			if err != nil {
				t.Fatal("Error after CheckTx: %v", err)
			}

			// This will fail because not serial (incrementing)
			// However, error should still be nil.
			// It just won't show up on Reap().
			err = mempool.CheckTx(txBytes, nil)
			if err != nil {
				t.Fatal("Error after CheckTx: %v", err)
			}

		}
	}

	reapCheck := func(exp int) {
		txs := mempool.Reap()
		if len(txs) != exp {
			t.Fatalf("Expected to reap %v txs but got %v", exp, len(txs))
		}
	}

	updateRange := func(start, end int) {
		txs := make([]types.Tx, 0)
		for i := start; i < end; i++ {
			txBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(txBytes, uint64(i))
			txs = append(txs, txBytes)
		}
		mempool.Update(0, txs)
	}

	commitRange := func(start, end int) {
		// Append some txs.
		for i := start; i < end; i++ {
			txBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(txBytes, uint64(i))
			code, result, logStr := appConnCon.AppendTx(txBytes)
			if code != tmsp.CodeType_OK {
				t.Errorf("Error committing tx. Code:%v result:%X log:%v",
					code, result, logStr)
			}
		}
		hash, log := appConnCon.Commit()
		if len(hash) != 8 {
			t.Errorf("Error committing. Hash:%X log:%v", hash, log)
		}
	}

	//----------------------------------------

	// Append some txs.
	appendTxsRange(0, 100)

	// Reap the txs.
	reapCheck(100)

	// Reap again.  We should get the same amount
	reapCheck(100)

	// Append 0 to 999, we should reap 900 new txs
	// because 100 were already counted.
	appendTxsRange(0, 1000)

	// Reap the txs.
	reapCheck(1000)

	// Reap again.  We should get the same amount
	reapCheck(1000)

	// Commit from the conensus AppConn
	commitRange(0, 500)
	updateRange(0, 500)

	// We should have 500 left.
	reapCheck(500)

	// Append 100 invalid txs and 100 valid txs
	appendTxsRange(900, 1100)

	// We should have 600 now.
	reapCheck(600)
}
