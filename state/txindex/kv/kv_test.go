package kv

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/abci/types"
	db "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/types"
)

func TestTxIndex(t *testing.T) {
	indexer := &TxIndex{store: db.NewMemDB()}

	tx := types.Tx("HELLO WORLD")
	txResult := &types.TxResult{1, 0, tx, abci.ResponseDeliverTx{Data: []byte{0}, Code: abci.CodeType_OK, Log: ""}}
	hash := tx.Hash()

	batch := txindex.NewBatch(1)
	batch.Add(*txResult)
	err := indexer.AddBatch(batch)
	require.Nil(t, err)

	loadedTxResult, err := indexer.Get(hash)
	require.Nil(t, err)
	assert.Equal(t, txResult, loadedTxResult)
}

func benchmarkTxIndex(txsCount int, b *testing.B) {
	tx := types.Tx("HELLO WORLD")
	txResult := &types.TxResult{1, 0, tx, abci.ResponseDeliverTx{Data: []byte{0}, Code: abci.CodeType_OK, Log: ""}}

	dir, err := ioutil.TempDir("", "tx_index_db")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store := db.NewDB("tx_index", "leveldb", dir)
	indexer := &TxIndex{store: store}

	batch := txindex.NewBatch(txsCount)
	for i := 0; i < txsCount; i++ {
		txResult.Index += 1
		batch.Add(*txResult)
	}

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		err = indexer.AddBatch(batch)
	}
}

func BenchmarkTxIndex1(b *testing.B)     { benchmarkTxIndex(1, b) }
func BenchmarkTxIndex500(b *testing.B)   { benchmarkTxIndex(500, b) }
func BenchmarkTxIndex1000(b *testing.B)  { benchmarkTxIndex(1000, b) }
func BenchmarkTxIndex2000(b *testing.B)  { benchmarkTxIndex(2000, b) }
func BenchmarkTxIndex10000(b *testing.B) { benchmarkTxIndex(10000, b) }
