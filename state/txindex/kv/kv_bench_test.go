package kv

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"testing"

	abci "github.com/tendermint/tendermint/abci/types"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

func BenchmarkTxSearch(b *testing.B) {
	dbDir, err := ioutil.TempDir("", "benchmark_tx_search_test")
	if err != nil {
		b.Errorf("failed to create temporary directory: %s", err)
	}

	db, err := dbm.NewGoLevelDB("benchmark_tx_search_test", dbDir)
	if err != nil {
		b.Errorf("failed to create database: %s", err)
	}

	allowedTags := []string{"transfer.address", "transfer.amount"}
	indexer := NewTxIndex(db, IndexTags(allowedTags))

	for i := 0; i < 35000; i++ {
		events := []abci.Event{
			{
				Type: "transfer",
				Attributes: []cmn.KVPair{
					{Key: []byte("address"), Value: []byte(fmt.Sprintf("address_%d", i%100))},
					{Key: []byte("amount"), Value: []byte("50")},
				},
			},
		}

		txBz := make([]byte, 8)
		if _, err := rand.Read(txBz); err != nil {
			b.Errorf("failed produce random bytes: %s", err)
		}

		txResult := &types.TxResult{
			Height: int64(i),
			Index:  0,
			Tx:     types.Tx(string(txBz)),
			Result: abci.ResponseDeliverTx{
				Data:   []byte{0},
				Code:   abci.CodeTypeOK,
				Log:    "",
				Events: events,
			},
		}

		if err := indexer.Index(txResult); err != nil {
			b.Errorf("failed to index tx: %s", err)
		}
	}

	txQuery := query.MustParse("transfer.address = 'address_43' AND transfer.amount = 50")

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := indexer.Search(txQuery); err != nil {
			b.Errorf("failed to query for txs: %s", err)
		}
	}
}
