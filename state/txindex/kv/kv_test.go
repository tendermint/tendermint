package kv

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/types"
)

func TestTxIndex(t *testing.T) {
	indexer := NewTxIndex(db.NewMemDB())

	tx := types.Tx("HELLO WORLD")
	txResult := &abci.TxResult{
		Height: 1,
		Index:  0,
		Tx:     tx,
		Result: abci.ResponseDeliverTx{
			Data: []byte{0},
			Code: abci.CodeTypeOK, Log: "", Events: nil,
		},
	}
	hash := tx.Hash()

	batch := txindex.NewBatch(1)
	if err := batch.Add(txResult); err != nil {
		t.Error(err)
	}
	err := indexer.AddBatch(batch)
	require.NoError(t, err)

	loadedTxResult, err := indexer.Get(hash)
	require.NoError(t, err)
	assert.True(t, proto.Equal(txResult, loadedTxResult))

	tx2 := types.Tx("BYE BYE WORLD")
	txResult2 := &abci.TxResult{
		Height: 1,
		Index:  0,
		Tx:     tx2,
		Result: abci.ResponseDeliverTx{
			Data: []byte{0},
			Code: abci.CodeTypeOK, Log: "", Events: nil,
		},
	}
	hash2 := tx2.Hash()

	err = indexer.Index(txResult2)
	require.NoError(t, err)

	loadedTxResult2, err := indexer.Get(hash2)
	require.NoError(t, err)
	assert.True(t, proto.Equal(txResult2, loadedTxResult2))
}

func TestTxSearch(t *testing.T) {
	indexer := NewTxIndex(db.NewMemDB())

	txResult := txResultWithEvents([]abci.Event{
		{Type: "account", Attributes: []abci.EventAttribute{{Key: []byte("number"), Value: []byte("1"), Index: true}}},
		{Type: "account", Attributes: []abci.EventAttribute{{Key: []byte("owner"), Value: []byte("Ivan"), Index: true}}},
		{Type: "", Attributes: []abci.EventAttribute{{Key: []byte("not_allowed"), Value: []byte("Vlad"), Index: true}}},
	})
	hash := types.Tx(txResult.Tx).Hash()

	err := indexer.Index(txResult)
	require.NoError(t, err)

	testCases := []struct {
		q             string
		resultsLength int
	}{
		// search by hash
		{fmt.Sprintf("tx.hash = '%X'", hash), 1},
		// search by exact match (one key)
		{"account.number = 1", 1},
		// search by exact match (two keys)
		{"account.number = 1 AND account.owner = 'Ivan'", 1},
		// search by exact match (two keys)
		{"account.number = 1 AND account.owner = 'Vlad'", 0},
		{"account.owner = 'Vlad' AND account.number = 1", 0},
		{"account.number >= 1 AND account.owner = 'Vlad'", 0},
		{"account.owner = 'Vlad' AND account.number >= 1", 0},
		{"account.number <= 0", 0},
		{"account.number <= 0 AND account.owner = 'Ivan'", 0},
		// search using a prefix of the stored value
		{"account.owner = 'Iv'", 0},
		// search by range
		{"account.number >= 1 AND account.number <= 5", 1},
		// search by range (lower bound)
		{"account.number >= 1", 1},
		// search by range (upper bound)
		{"account.number <= 5", 1},
		// search using not allowed key
		{"not_allowed = 'boom'", 0},
		// search for not existing tx result
		{"account.number >= 2 AND account.number <= 5", 0},
		// search using not existing key
		{"account.date >= TIME 2013-05-03T14:45:00Z", 0},
		// search using CONTAINS
		{"account.owner CONTAINS 'an'", 1},
		// search for non existing value using CONTAINS
		{"account.owner CONTAINS 'Vlad'", 0},
		// search using the wrong key (of numeric type) using CONTAINS
		{"account.number CONTAINS 'Iv'", 0},
		// search using EXISTS
		{"account.number EXISTS", 1},
		// search using EXISTS for non existing key
		{"account.date EXISTS", 0},
	}

	ctx := context.Background()

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.q, func(t *testing.T) {
			results, err := indexer.Search(ctx, query.MustParse(tc.q))
			assert.NoError(t, err)

			assert.Len(t, results, tc.resultsLength)
			if tc.resultsLength > 0 {
				for _, txr := range results {
					assert.True(t, proto.Equal(txResult, txr))
				}
			}
		})
	}
}

func TestTxSearchWithCancelation(t *testing.T) {
	indexer := NewTxIndex(db.NewMemDB())

	txResult := txResultWithEvents([]abci.Event{
		{Type: "account", Attributes: []abci.EventAttribute{{Key: []byte("number"), Value: []byte("1"), Index: true}}},
		{Type: "account", Attributes: []abci.EventAttribute{{Key: []byte("owner"), Value: []byte("Ivan"), Index: true}}},
		{Type: "", Attributes: []abci.EventAttribute{{Key: []byte("not_allowed"), Value: []byte("Vlad"), Index: true}}},
	})
	err := indexer.Index(txResult)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	results, err := indexer.Search(ctx, query.MustParse("account.number = 1"))
	assert.NoError(t, err)
	assert.Empty(t, results)
}

func TestTxSearchDeprecatedIndexing(t *testing.T) {
	indexer := NewTxIndex(db.NewMemDB())

	// index tx using events indexing (composite key)
	txResult1 := txResultWithEvents([]abci.Event{
		{Type: "account", Attributes: []abci.EventAttribute{{Key: []byte("number"), Value: []byte("1"), Index: true}}},
	})
	hash1 := types.Tx(txResult1.Tx).Hash()

	err := indexer.Index(txResult1)
	require.NoError(t, err)

	// index tx also using deprecated indexing (event as key)
	txResult2 := txResultWithEvents(nil)
	txResult2.Tx = types.Tx("HELLO WORLD 2")

	hash2 := types.Tx(txResult2.Tx).Hash()
	b := indexer.store.NewBatch()

	rawBytes, err := proto.Marshal(txResult2)
	require.NoError(t, err)

	depKey := []byte(fmt.Sprintf("%s/%s/%d/%d",
		"sender",
		"addr1",
		txResult2.Height,
		txResult2.Index,
	))

	err = b.Set(depKey, hash2)
	require.NoError(t, err)
	err = b.Set(keyForHeight(txResult2), hash2)
	require.NoError(t, err)
	err = b.Set(hash2, rawBytes)
	require.NoError(t, err)
	err = b.Write()
	require.NoError(t, err)

	testCases := []struct {
		q       string
		results []*abci.TxResult
	}{
		// search by hash
		{fmt.Sprintf("tx.hash = '%X'", hash1), []*abci.TxResult{txResult1}},
		// search by hash
		{fmt.Sprintf("tx.hash = '%X'", hash2), []*abci.TxResult{txResult2}},
		// search by exact match (one key)
		{"account.number = 1", []*abci.TxResult{txResult1}},
		{"account.number >= 1 AND account.number <= 5", []*abci.TxResult{txResult1}},
		// search by range (lower bound)
		{"account.number >= 1", []*abci.TxResult{txResult1}},
		// search by range (upper bound)
		{"account.number <= 5", []*abci.TxResult{txResult1}},
		// search using not allowed key
		{"not_allowed = 'boom'", []*abci.TxResult{}},
		// search for not existing tx result
		{"account.number >= 2 AND account.number <= 5", []*abci.TxResult{}},
		// search using not existing key
		{"account.date >= TIME 2013-05-03T14:45:00Z", []*abci.TxResult{}},
		// search by deprecated key
		{"sender = 'addr1'", []*abci.TxResult{txResult2}},
	}

	ctx := context.Background()

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.q, func(t *testing.T) {
			results, err := indexer.Search(ctx, query.MustParse(tc.q))
			require.NoError(t, err)
			for _, txr := range results {
				for _, tr := range tc.results {
					assert.True(t, proto.Equal(tr, txr))
				}
			}
		})
	}
}

func TestTxSearchOneTxWithMultipleSameTagsButDifferentValues(t *testing.T) {
	indexer := NewTxIndex(db.NewMemDB())

	txResult := txResultWithEvents([]abci.Event{
		{Type: "account", Attributes: []abci.EventAttribute{{Key: []byte("number"), Value: []byte("1"), Index: true}}},
		{Type: "account", Attributes: []abci.EventAttribute{{Key: []byte("number"), Value: []byte("2"), Index: true}}},
	})

	err := indexer.Index(txResult)
	require.NoError(t, err)

	ctx := context.Background()

	results, err := indexer.Search(ctx, query.MustParse("account.number >= 1"))
	assert.NoError(t, err)

	assert.Len(t, results, 1)
	for _, txr := range results {
		assert.True(t, proto.Equal(txResult, txr))
	}
}

func TestTxSearchMultipleTxs(t *testing.T) {
	indexer := NewTxIndex(db.NewMemDB())

	// indexed first, but bigger height (to test the order of transactions)
	txResult := txResultWithEvents([]abci.Event{
		{Type: "account", Attributes: []abci.EventAttribute{{Key: []byte("number"), Value: []byte("1"), Index: true}}},
	})

	txResult.Tx = types.Tx("Bob's account")
	txResult.Height = 2
	txResult.Index = 1
	err := indexer.Index(txResult)
	require.NoError(t, err)

	// indexed second, but smaller height (to test the order of transactions)
	txResult2 := txResultWithEvents([]abci.Event{
		{Type: "account", Attributes: []abci.EventAttribute{{Key: []byte("number"), Value: []byte("2"), Index: true}}},
	})
	txResult2.Tx = types.Tx("Alice's account")
	txResult2.Height = 1
	txResult2.Index = 2

	err = indexer.Index(txResult2)
	require.NoError(t, err)

	// indexed third (to test the order of transactions)
	txResult3 := txResultWithEvents([]abci.Event{
		{Type: "account", Attributes: []abci.EventAttribute{{Key: []byte("number"), Value: []byte("3"), Index: true}}},
	})
	txResult3.Tx = types.Tx("Jack's account")
	txResult3.Height = 1
	txResult3.Index = 1
	err = indexer.Index(txResult3)
	require.NoError(t, err)

	// indexed fourth (to test we don't include txs with similar events)
	// https://github.com/tendermint/tendermint/issues/2908
	txResult4 := txResultWithEvents([]abci.Event{
		{Type: "account", Attributes: []abci.EventAttribute{{Key: []byte("number.id"), Value: []byte("1"), Index: true}}},
	})
	txResult4.Tx = types.Tx("Mike's account")
	txResult4.Height = 2
	txResult4.Index = 2
	err = indexer.Index(txResult4)
	require.NoError(t, err)

	ctx := context.Background()

	results, err := indexer.Search(ctx, query.MustParse("account.number >= 1"))
	assert.NoError(t, err)

	require.Len(t, results, 3)
}

func txResultWithEvents(events []abci.Event) *abci.TxResult {
	tx := types.Tx("HELLO WORLD")
	return &abci.TxResult{
		Height: 1,
		Index:  0,
		Tx:     tx,
		Result: abci.ResponseDeliverTx{
			Data:   []byte{0},
			Code:   abci.CodeTypeOK,
			Log:    "",
			Events: events,
		},
	}
}

func benchmarkTxIndex(txsCount int64, b *testing.B) {
	dir, err := ioutil.TempDir("", "tx_index_db")
	require.NoError(b, err)
	defer os.RemoveAll(dir)

	store, err := db.NewDB("tx_index", "goleveldb", dir)
	require.NoError(b, err)
	indexer := NewTxIndex(store)

	batch := txindex.NewBatch(txsCount)
	txIndex := uint32(0)
	for i := int64(0); i < txsCount; i++ {
		tx := tmrand.Bytes(250)
		txResult := &abci.TxResult{
			Height: 1,
			Index:  txIndex,
			Tx:     tx,
			Result: abci.ResponseDeliverTx{
				Data:   []byte{0},
				Code:   abci.CodeTypeOK,
				Log:    "",
				Events: []abci.Event{},
			},
		}
		if err := batch.Add(txResult); err != nil {
			b.Fatal(err)
		}
		txIndex++
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
