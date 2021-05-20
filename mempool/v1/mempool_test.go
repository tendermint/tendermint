package v1

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

// application extends the KV store application by overriding CheckTx to provide
// transaction priority based on the value in the key/value pair.
type application struct {
	*kvstore.Application
}

func (app *application) CheckTx(req abci.RequestCheckTx) abci.ResponseCheckTx {
	var priority int64

	// infer the priority from the raw transaction value
	parts := bytes.Split(req.Tx, []byte("="))
	if len(parts) == 2 {
		v, err := binary.ReadVarint(bytes.NewBuffer(parts[1]))
		if err != nil {
			return abci.ResponseCheckTx{
				Priority:  priority,
				Code:      100,
				GasWanted: 1,
			}
		}

		priority = v
	} else {
		return abci.ResponseCheckTx{
			Priority:  priority,
			Code:      101,
			GasWanted: 1,
		}
	}

	return abci.ResponseCheckTx{
		Priority:  priority,
		Code:      code.CodeTypeOK,
		GasWanted: 1,
	}
}

func setup(t *testing.T) *TxMempool {
	t.Helper()

	app := &application{kvstore.NewApplication()}
	cc := proxy.NewLocalClientCreator(app)
	cfg := config.ResetTestRoot(t.Name())

	appConnMem, err := cc.NewABCIClient()
	require.NoError(t, err)
	require.NoError(t, appConnMem.Start())

	t.Cleanup(func() {
		os.RemoveAll(cfg.RootDir)
	})

	return NewTxMempool(log.TestingLogger().With("test", t.Name()), cfg.Mempool, appConnMem, 0)
}

func checkTxs(t *testing.T, txmp *TxMempool, numTxs int, peerID uint16) types.Txs {
	txs := make(types.Txs, numTxs)
	txInfo := mempool.TxInfo{SenderID: peerID}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < numTxs; i++ {
		prefix := make([]byte, 20)
		_, err := rng.Read(prefix)
		require.NoError(t, err)

		txs[i] = []byte(fmt.Sprintf("%X=%d", prefix, i))
		require.NoError(t, txmp.CheckTx(txs[i], nil, txInfo))
	}

	return txs
}

func TestTxMempool_TxsAvailable(t *testing.T) {
	txmp := setup(t)
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

	responses := make([]*abci.ResponseDeliverTx, len(txs[:50]))
	for i := 0; i < len(responses); i++ {
		responses[i] = &abci.ResponseDeliverTx{Code: abci.CodeTypeOK}
	}

	// commit half the transactions and ensure we fire an event
	txmp.Lock()
	require.NoError(t, txmp.Update(1, txs[:50], responses, nil, nil))
	txmp.Unlock()
	ensureTxFire()
	ensureNoTxFire()

	// Execute CheckTx for more transactions and ensure we do not fire another
	// event as we're still on the same height (1).
	_ = checkTxs(t, txmp, 100, 0)
	ensureNoTxFire()
}

func TestTxMempool_Size(t *testing.T) {
	txmp := setup(t)

	txs := checkTxs(t, txmp, 100, 0)
	require.Equal(t, len(txs), txmp.Size())

	responses := make([]*abci.ResponseDeliverTx, len(txs[:50]))
	for i := 0; i < len(responses); i++ {
		responses[i] = &abci.ResponseDeliverTx{Code: abci.CodeTypeOK}
	}

	txmp.Lock()
	require.NoError(t, txmp.Update(1, txs[:50], responses, nil, nil))
	txmp.Unlock()

	require.Equal(t, len(txs)/2, txmp.Size())
}
