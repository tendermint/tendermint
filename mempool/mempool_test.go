package mempool

import (
	"encoding/binary"
	"testing"

	"github.com/tendermint/tendermint/config/tendermint_test"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/abci/example/counter"
)

func TestSerialReap(t *testing.T) {
	config := tendermint_test.ResetConfig("mempool_mempool_test")

	app := counter.NewCounterApplication(true)
	app.SetOption("serial", "on")
	cc := proxy.NewLocalClientCreator(app)
	appConnMem, _ := cc.NewABCIClient()
	appConnCon, _ := cc.NewABCIClient()
	mempool := NewMempool(config, appConnMem)

	deliverTxsRange := func(start, end int) {
		// Deliver some txs.
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
		txs := mempool.Reap(-1)
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
		// Deliver some txs.
		for i := start; i < end; i++ {
			txBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(txBytes, uint64(i))
			res := appConnCon.DeliverTxSync(txBytes)
			if !res.IsOK() {
				t.Errorf("Error committing tx. Code:%v result:%X log:%v",
					res.Code, res.Data, res.Log)
			}
		}
		res := appConnCon.CommitSync()
		if len(res.Data) != 8 {
			t.Errorf("Error committing. Hash:%X log:%v", res.Data, res.Log)
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
