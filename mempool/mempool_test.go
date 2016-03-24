package mempool

import (
	"encoding/binary"
	"sync"
	"testing"

	"github.com/tendermint/tendermint/config/tendermint_test"
	"github.com/tendermint/tendermint/types"
	tmspcli "github.com/tendermint/tmsp/client"
	"github.com/tendermint/tmsp/example/counter"
)

func init() {
	tendermint_test.ResetConfig("mempool_mempool_test")
}

func TestSerialReap(t *testing.T) {

	app := counter.NewCounterApplication(true)
	app.SetOption("serial", "on")
	mtx := new(sync.Mutex)
	appConnMem := tmspcli.NewLocalClient(mtx, app)
	appConnCon := tmspcli.NewLocalClient(mtx, app)
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
		txs := mempool.Reap(0)
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
			res := appConnCon.AppendTx(txBytes)
			if !res.IsOK() {
				t.Errorf("Error committing tx. Code:%v result:%X log:%v",
					res.Code, res.Data, res.Log)
			}
		}
		res := appConnCon.Commit()
		if len(res.Data) != 8 {
			t.Errorf("Error committing. Hash:%X log:%v", res.Data, res.Log)
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
