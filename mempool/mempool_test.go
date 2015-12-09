package mempool

import (
	"encoding/binary"
	"testing"

	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tmsp/example"
	tmsp "github.com/tendermint/tmsp/types"
)

func TestSerialReap(t *testing.T) {

	app := example.NewCounterApplication(true)
	appCtxMempool := app.Open()
	proxyAppCtx := proxy.NewLocalAppContext(appCtxMempool)
	mempool := NewMempool(proxyAppCtx)

	// Create another AppContext for committing.
	appCtxConsensus := app.Open()
	appCtxConsensus.SetOption("serial", "on")

	appendTxsRange := func(start, end int) {
		// Append some txs.
		for i := start; i < end; i++ {

			// This will succeed
			txBytes := make([]byte, 32)
			binary.LittleEndian.PutUint64(txBytes, uint64(i))
			err := mempool.AppendTx(txBytes)
			if err != nil {
				t.Fatal("Error after AppendTx: %v", err)
			}

			// This will fail because not serial (incrementing)
			// However, error should still be nil.
			// It just won't show up on Reap().
			err = mempool.AppendTx(txBytes)
			if err != nil {
				t.Fatal("Error after AppendTx: %v", err)
			}

		}
	}

	reapCheck := func(exp int) {
		txs, _, err := mempool.Reap()
		if err != nil {
			t.Error("Error in mempool.Reap()", err)
		}
		if len(txs) != exp {
			t.Fatalf("Expected to reap %v txs but got %v", exp, len(txs))
		}
	}

	updateRange := func(start, end int) {
		txs := make([]types.Tx, 0)
		for i := start; i < end; i++ {
			txBytes := make([]byte, 32)
			binary.LittleEndian.PutUint64(txBytes, uint64(i))
			txs = append(txs, txBytes)
		}
		blockHeader := &types.Header{Height: 0}
		blockData := &types.Data{Txs: txs}
		block := &types.Block{Header: blockHeader, Data: blockData}
		err := mempool.Update(block)
		if err != nil {
			t.Error("Error in mempool.Update()", err)
		}
	}

	commitRange := func(start, end int) {
		// Append some txs.
		for i := start; i < end; i++ {
			txBytes := make([]byte, 32)
			binary.LittleEndian.PutUint64(txBytes, uint64(i))
			_, retCode := appCtxConsensus.AppendTx(txBytes)
			if retCode != tmsp.RetCodeOK {
				t.Error("Error committing tx", retCode)
			}
		}
		retCode := appCtxConsensus.Commit()
		if retCode != tmsp.RetCodeOK {
			t.Error("Error committing range", retCode)
		}
	}

	//----------------------------------------

	// Append some txs.
	appendTxsRange(0, 100)

	// Reap the txs.
	reapCheck(100)

	// Reap again.  We should get the same amount
	reapCheck(100)

	// Append 0 to 999, we should reap 900 txs
	// because 100 were already counted.
	appendTxsRange(0, 1000)

	// Reap the txs.
	reapCheck(1000)

	// Reap again.  We should get the same amount
	reapCheck(1000)

	// Commit from the conensus AppContext
	commitRange(0, 500)
	updateRange(0, 500)

	// We should have 500 left.
	reapCheck(500)

}
