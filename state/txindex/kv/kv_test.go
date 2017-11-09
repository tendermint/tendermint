package kv

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/abci/types"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/types"
	db "github.com/tendermint/tmlibs/db"
)

func TestTxIndex(t *testing.T) {
	indexer := &TxIndex{store: db.NewMemDB()}

	tx := types.Tx("HELLO WORLD")
	txResult := &types.TxResult{1, 0, tx, abci.ResponseDeliverTx{Data: []byte{0}, Code: abci.CodeType_OK, Log: "", Tags: []*abci.KVPair{}}}
	hash := tx.Hash()

	batch := txindex.NewBatch(1)
	if err := batch.Add(*txResult); err != nil {
		t.Error(err)
	}
	err := indexer.AddBatch(batch)
	require.Nil(t, err)

	loadedTxResult, err := indexer.Get(hash)
	require.Nil(t, err)
	assert.Equal(t, txResult, loadedTxResult)
}

func benchmarkTxIndex(txsCount int, b *testing.B) {
	tx := types.Tx("HELLO WORLD")
	txResult := &types.TxResult{1, 0, tx, abci.ResponseDeliverTx{Data: []byte{0}, Code: abci.CodeType_OK, Log: "", Tags: []*abci.KVPair{}}}

	dir, err := ioutil.TempDir("", "tx_index_db")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir) // nolint: errcheck

	store := db.NewDB("tx_index", "leveldb", dir)
	indexer := &TxIndex{store: store}

	batch := txindex.NewBatch(txsCount)
	for i := 0; i < txsCount; i++ {
		if err := batch.Add(*txResult); err != nil {
			b.Fatal(err)
		}
		txResult.Index += 1
	}

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		err = indexer.AddBatch(batch)
	}
	if err != nil {
		b.Fatal(err)
	}
}

func BenchmarkTxIndex1(b *testing.B)     { benchmarkTxIndex(1, b) }
func BenchmarkTxIndex500(b *testing.B)   { benchmarkTxIndex(500, b) }
func BenchmarkTxIndex1000(b *testing.B)  { benchmarkTxIndex(1000, b) }
func BenchmarkTxIndex2000(b *testing.B)  { benchmarkTxIndex(2000, b) }
func BenchmarkTxIndex10000(b *testing.B) { benchmarkTxIndex(10000, b) }
