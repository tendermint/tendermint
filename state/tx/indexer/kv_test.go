package indexer

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/abci/types"
	db "github.com/tendermint/go-db"
	"github.com/tendermint/tendermint/types"
)

func TestKVIndex(t *testing.T) {
	indexer := &KV{store: db.NewMemDB()}

	tx := types.Tx("HELLO WORLD")
	txResult := &types.TxResult{1, 1, abci.ResponseDeliverTx{Data: []byte{0}, Code: abci.CodeType_OK, Log: ""}}
	hash := string(tx.Hash())

	batch := NewBatch()
	batch.Index(hash, *txResult)
	err := indexer.Batch(batch)
	require.Nil(t, err)

	loadedTxResult, err := indexer.Tx(hash)
	require.Nil(t, err)
	assert.Equal(t, txResult, loadedTxResult)
}

func benchmarkKVIndex(txsCount int, b *testing.B) {
	txResult := &types.TxResult{1, 1, abci.ResponseDeliverTx{Data: []byte{0}, Code: abci.CodeType_OK, Log: ""}}

	dir, err := ioutil.TempDir("", "tx_indexer_db")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)

	store := db.NewDB("tx_indexer", "leveldb", dir)
	indexer := &KV{store: store}

	batch := NewBatch()
	for i := 0; i < txsCount; i++ {
		batch.Index(fmt.Sprintf("hash%v", i), *txResult)
	}

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		err = indexer.Batch(batch)
	}
}

func BenchmarkKVIndex1(b *testing.B)     { benchmarkKVIndex(1, b) }
func BenchmarkKVIndex500(b *testing.B)   { benchmarkKVIndex(500, b) }
func BenchmarkKVIndex1000(b *testing.B)  { benchmarkKVIndex(1000, b) }
func BenchmarkKVIndex2000(b *testing.B)  { benchmarkKVIndex(2000, b) }
func BenchmarkKVIndex10000(b *testing.B) { benchmarkKVIndex(10000, b) }
