package kvsink

import (
	"context"
	"fmt"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	kvtx "github.com/tendermint/tendermint/state/indexer/tx/kv"
	"github.com/tendermint/tendermint/types"
	db "github.com/tendermint/tm-db"
)

func TestBlockFuncs(t *testing.T) {
	store := db.NewPrefixDB(db.NewMemDB(), []byte("block_events"))
	indexer := NewKVEventSink(store)

	require.NoError(t, indexer.IndexBlockEvents(types.EventDataNewBlockHeader{
		Header: types.Header{Height: 1},
		ResultBeginBlock: abci.ResponseBeginBlock{
			Events: []abci.Event{
				{
					Type: "begin_event",
					Attributes: []abci.EventAttribute{
						{
							Key:   []byte("proposer"),
							Value: []byte("FCAA001"),
							Index: true,
						},
					},
				},
			},
		},
		ResultEndBlock: abci.ResponseEndBlock{
			Events: []abci.Event{
				{
					Type: "end_event",
					Attributes: []abci.EventAttribute{
						{
							Key:   []byte("foo"),
							Value: []byte("100"),
							Index: true,
						},
					},
				},
			},
		},
	}))

	for i := 2; i < 12; i++ {
		var index bool
		if i%2 == 0 {
			index = true
		}

		require.NoError(t, indexer.IndexBlockEvents(types.EventDataNewBlockHeader{
			Header: types.Header{Height: int64(i)},
			ResultBeginBlock: abci.ResponseBeginBlock{
				Events: []abci.Event{
					{
						Type: "begin_event",
						Attributes: []abci.EventAttribute{
							{
								Key:   []byte("proposer"),
								Value: []byte("FCAA001"),
								Index: true,
							},
						},
					},
				},
			},
			ResultEndBlock: abci.ResponseEndBlock{
				Events: []abci.Event{
					{
						Type: "end_event",
						Attributes: []abci.EventAttribute{
							{
								Key:   []byte("foo"),
								Value: []byte(fmt.Sprintf("%d", i)),
								Index: index,
							},
						},
					},
				},
			},
		}))
	}

	testCases := map[string]struct {
		q       *query.Query
		results []int64
	}{
		"block.height = 100": {
			q:       query.MustParse("block.height = 100"),
			results: []int64{},
		},
		"block.height = 5": {
			q:       query.MustParse("block.height = 5"),
			results: []int64{5},
		},
		"begin_event.key1 = 'value1'": {
			q:       query.MustParse("begin_event.key1 = 'value1'"),
			results: []int64{},
		},
		"begin_event.proposer = 'FCAA001'": {
			q:       query.MustParse("begin_event.proposer = 'FCAA001'"),
			results: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
		},
		"end_event.foo <= 5": {
			q:       query.MustParse("end_event.foo <= 5"),
			results: []int64{2, 4},
		},
		"end_event.foo >= 100": {
			q:       query.MustParse("end_event.foo >= 100"),
			results: []int64{1},
		},
		"block.height > 2 AND end_event.foo <= 8": {
			q:       query.MustParse("block.height > 2 AND end_event.foo <= 8"),
			results: []int64{4, 6, 8},
		},
		"begin_event.proposer CONTAINS 'FFFFFFF'": {
			q:       query.MustParse("begin_event.proposer CONTAINS 'FFFFFFF'"),
			results: []int64{},
		},
		"begin_event.proposer CONTAINS 'FCAA001'": {
			q:       query.MustParse("begin_event.proposer CONTAINS 'FCAA001'"),
			results: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
		},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			results, err := indexer.SearchBlockEvents(context.Background(), tc.q)
			require.NoError(t, err)
			require.Equal(t, tc.results, results)
		})
	}
}

func TestTxSearchWithCancelation(t *testing.T) {
	indexer := NewKVEventSink(db.NewMemDB())

	txResult := txResultWithEvents([]abci.Event{
		{Type: "account", Attributes: []abci.EventAttribute{{Key: []byte("number"), Value: []byte("1"), Index: true}}},
		{Type: "account", Attributes: []abci.EventAttribute{{Key: []byte("owner"), Value: []byte("Ivan"), Index: true}}},
		{Type: "", Attributes: []abci.EventAttribute{{Key: []byte("not_allowed"), Value: []byte("Vlad"), Index: true}}},
	})
	err := indexer.IndexTxEvents(txResult)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	results, err := indexer.SearchTxEvents(ctx, query.MustParse("account.number = 1"))
	assert.NoError(t, err)
	assert.Empty(t, results)
}

func TestTxSearchDeprecatedIndexing(t *testing.T) {
	esdb := db.NewMemDB()
	indexer := NewKVEventSink(esdb)

	// index tx using events indexing (composite key)
	txResult1 := txResultWithEvents([]abci.Event{
		{Type: "account", Attributes: []abci.EventAttribute{{Key: []byte("number"), Value: []byte("1"), Index: true}}},
	})
	hash1 := types.Tx(txResult1.Tx).Hash()

	err := indexer.IndexTxEvents(txResult1)
	require.NoError(t, err)

	// index tx also using deprecated indexing (event as key)
	txResult2 := txResultWithEvents(nil)
	txResult2.Tx = types.Tx("HELLO WORLD 2")

	hash2 := types.Tx(txResult2.Tx).Hash()
	b := esdb.NewBatch()

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
	err = b.Set(kvtx.KeyFromHeight(txResult2), hash2)
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
			results, err := indexer.SearchTxEvents(ctx, query.MustParse(tc.q))
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
	indexer := NewKVEventSink(db.NewMemDB())

	txResult := txResultWithEvents([]abci.Event{
		{Type: "account", Attributes: []abci.EventAttribute{{Key: []byte("number"), Value: []byte("1"), Index: true}}},
		{Type: "account", Attributes: []abci.EventAttribute{{Key: []byte("number"), Value: []byte("2"), Index: true}}},
	})

	err := indexer.IndexTxEvents(txResult)
	require.NoError(t, err)

	ctx := context.Background()

	results, err := indexer.SearchTxEvents(ctx, query.MustParse("account.number >= 1"))
	assert.NoError(t, err)

	assert.Len(t, results, 1)
	for _, txr := range results {
		assert.True(t, proto.Equal(txResult, txr))
	}
}

func TestTxSearchMultipleTxs(t *testing.T) {
	indexer := NewKVEventSink(db.NewMemDB())

	// indexed first, but bigger height (to test the order of transactions)
	txResult := txResultWithEvents([]abci.Event{
		{Type: "account", Attributes: []abci.EventAttribute{{Key: []byte("number"), Value: []byte("1"), Index: true}}},
	})

	txResult.Tx = types.Tx("Bob's account")
	txResult.Height = 2
	txResult.Index = 1
	err := indexer.IndexTxEvents(txResult)
	require.NoError(t, err)

	// indexed second, but smaller height (to test the order of transactions)
	txResult2 := txResultWithEvents([]abci.Event{
		{Type: "account", Attributes: []abci.EventAttribute{{Key: []byte("number"), Value: []byte("2"), Index: true}}},
	})
	txResult2.Tx = types.Tx("Alice's account")
	txResult2.Height = 1
	txResult2.Index = 2

	err = indexer.IndexTxEvents(txResult2)
	require.NoError(t, err)

	// indexed third (to test the order of transactions)
	txResult3 := txResultWithEvents([]abci.Event{
		{Type: "account", Attributes: []abci.EventAttribute{{Key: []byte("number"), Value: []byte("3"), Index: true}}},
	})
	txResult3.Tx = types.Tx("Jack's account")
	txResult3.Height = 1
	txResult3.Index = 1
	err = indexer.IndexTxEvents(txResult3)
	require.NoError(t, err)

	// indexed fourth (to test we don't include txs with similar events)
	// https://github.com/tendermint/tendermint/issues/2908
	txResult4 := txResultWithEvents([]abci.Event{
		{Type: "account", Attributes: []abci.EventAttribute{{Key: []byte("number.id"), Value: []byte("1"), Index: true}}},
	})
	txResult4.Tx = types.Tx("Mike's account")
	txResult4.Height = 2
	txResult4.Index = 2
	err = indexer.IndexTxEvents(txResult4)
	require.NoError(t, err)

	ctx := context.Background()

	results, err := indexer.SearchTxEvents(ctx, query.MustParse("account.number >= 1"))
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
