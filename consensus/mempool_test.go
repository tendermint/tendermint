package consensus

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/tendermint/tendermint/config/tendermint_test"
	"github.com/tendermint/tendermint/types"
	tmsp "github.com/tendermint/tmsp/types"

	. "github.com/tendermint/go-common"
)

func init() {
	config = tendermint_test.ResetConfig("consensus_mempool_test")
}

func TestTxConcurrentWithCommit(t *testing.T) {

	state, privVals := randGenesisState(1, false, 10)
	cs := newConsensusState(state, privVals[0], NewCounterApplication())
	height, round := cs.Height, cs.Round
	newBlockCh := subscribeToEvent(cs.evsw, "tester", types.EventStringNewBlock(), 1)

	appendTxsRange := func(start, end int) {
		// Append some txs.
		for i := start; i < end; i++ {
			txBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(txBytes, uint64(i))
			err := cs.mempool.CheckTx(txBytes, nil)
			if err != nil {
				panic(Fmt("Error after CheckTx: %v", err))
			}
			//	time.Sleep(time.Microsecond * time.Duration(rand.Int63n(3000)))
		}
	}

	NTxs := 10000
	go appendTxsRange(0, NTxs)

	startTestRound(cs, height, round)
	ticker := time.NewTicker(time.Second * 20)
	for nTxs := 0; nTxs < NTxs; {
		select {
		case b := <-newBlockCh:
			nTxs += b.(types.EventDataNewBlock).Block.Header.NumTxs
		case <-ticker.C:
			panic("Timed out waiting to commit blocks with transactions")
		}
	}
}

func TestRmBadTx(t *testing.T) {
	state, privVals := randGenesisState(1, false, 10)
	app := NewCounterApplication()
	cs := newConsensusState(state, privVals[0], app)

	// increment the counter by 1
	txBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(txBytes, uint64(0))
	app.AppendTx(txBytes)
	app.Commit()

	ch := make(chan struct{})
	cbCh := make(chan struct{})
	go func() {
		// Try to send the tx through the mempool.
		// CheckTx should not err, but the app should return a bad tmsp code
		// and the tx should get removed from the pool
		err := cs.mempool.CheckTx(txBytes, func(r *tmsp.Response) {
			if r.GetCheckTx().Code != tmsp.CodeType_BadNonce {
				t.Fatalf("expected checktx to return bad nonce, got %v", r)
			}
			cbCh <- struct{}{}
		})
		if err != nil {
			t.Fatal("Error after CheckTx: %v", err)
		}

		// check for the tx
		for {
			time.Sleep(time.Second)
			select {
			case <-ch:
			default:
				txs := cs.mempool.Reap(1)
				if len(txs) == 0 {
					ch <- struct{}{}
				}

			}
		}
	}()

	// Wait until the tx returns
	ticker := time.After(time.Second * 5)
	select {
	case <-cbCh:
		// success
	case <-ticker:
		t.Fatalf("Timed out waiting for tx to return")
	}

	// Wait until the tx is removed
	ticker = time.After(time.Second * 5)
	select {
	case <-ch:
		// success
	case <-ticker:
		t.Fatalf("Timed out waiting for tx to be removed")
	}
}

// CounterApplication that maintains a mempool state and resets it upon commit
type CounterApplication struct {
	txCount        int
	mempoolTxCount int
}

func NewCounterApplication() *CounterApplication {
	return &CounterApplication{}
}

func (app *CounterApplication) Info() (string, *tmsp.TMSPInfo, *tmsp.LastBlockInfo, *tmsp.ConfigInfo) {
	return Fmt("txs:%v", app.txCount), nil, nil, nil
}

func (app *CounterApplication) SetOption(key string, value string) (log string) {
	return ""
}

func (app *CounterApplication) AppendTx(tx []byte) tmsp.Result {
	return runTx(tx, &app.txCount)
}

func (app *CounterApplication) CheckTx(tx []byte) tmsp.Result {
	return runTx(tx, &app.mempoolTxCount)
}

func runTx(tx []byte, countPtr *int) tmsp.Result {
	count := *countPtr
	tx8 := make([]byte, 8)
	copy(tx8[len(tx8)-len(tx):], tx)
	txValue := binary.BigEndian.Uint64(tx8)
	if txValue != uint64(count) {
		return tmsp.ErrBadNonce.AppendLog(Fmt("Invalid nonce. Expected %v, got %v", count, txValue))
	}
	*countPtr += 1
	return tmsp.OK
}

func (app *CounterApplication) Commit() tmsp.Result {
	app.mempoolTxCount = app.txCount
	if app.txCount == 0 {
		return tmsp.OK
	} else {
		hash := make([]byte, 8)
		binary.BigEndian.PutUint64(hash, uint64(app.txCount))
		return tmsp.NewResultOK(hash, "")
	}
}

func (app *CounterApplication) Query(query []byte) tmsp.Result {
	return tmsp.NewResultOK(nil, Fmt("Query is not supported"))
}
