package txindex_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	db "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	blockidxkv "github.com/tendermint/tendermint/state/indexer/block/kv"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/state/txindex/kv"
	"github.com/tendermint/tendermint/types"
)

func TestIndexerServiceIndexesBlocks(t *testing.T) {
	// event bus
	eventBus := types.NewEventBus()
	eventBus.SetLogger(log.TestingLogger())
	err := eventBus.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := eventBus.Stop(); err != nil {
			t.Error(err)
		}
	})

	// tx indexer
	store := db.NewMemDB()
	txIndexer := kv.NewTxIndex(store)
	blockIndexer := blockidxkv.New(db.NewPrefixDB(store, []byte("block_events")))

	service := txindex.NewIndexerService(txIndexer, blockIndexer, eventBus)
	service.SetLogger(log.TestingLogger())
	err = service.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := service.Stop(); err != nil {
			t.Error(err)
		}
	})

	// publish block with txs
	err = eventBus.PublishEventNewBlockHeader(types.EventDataNewBlockHeader{
		Header: types.Header{Height: 1},
		NumTxs: int64(2),
	})
	require.NoError(t, err)
	txResult1 := &abci.TxResult{
		Height: 1,
		Index:  uint32(0),
		Tx:     types.Tx("foo"),
		Result: abci.ResponseDeliverTx{Code: 0},
	}
	err = eventBus.PublishEventTx(types.EventDataTx{TxResult: *txResult1})
	require.NoError(t, err)
	txResult2 := &abci.TxResult{
		Height: 1,
		Index:  uint32(1),
		Tx:     types.Tx("bar"),
		Result: abci.ResponseDeliverTx{Code: 0},
	}
	err = eventBus.PublishEventTx(types.EventDataTx{TxResult: *txResult2})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	res, err := txIndexer.Get(types.Tx("foo").Hash())
	require.NoError(t, err)
	require.Equal(t, txResult1, res)

	ok, err := blockIndexer.Has(1)
	require.NoError(t, err)
	require.True(t, ok)

	res, err = txIndexer.Get(types.Tx("bar").Hash())
	require.NoError(t, err)
	require.Equal(t, txResult2, res)
}

func TestTxIndexDuplicatePreviouslySuccessful(t *testing.T) {
	var mockTx = types.Tx("MOCK_TX_HASH")

	testCases := []struct {
		name    string
		tx1     abci.TxResult
		tx2     abci.TxResult
		expSkip bool // do we expect the second tx to be skipped by tx indexer
	}{
		{"skip, previously successful",
			abci.TxResult{
				Height: 1,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ResponseDeliverTx{
					Code: abci.CodeTypeOK,
				},
			},
			abci.TxResult{
				Height: 2,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ResponseDeliverTx{
					Code: abci.CodeTypeOK + 1,
				},
			},
			true,
		},
		{"not skip, previously unsuccessful",
			abci.TxResult{
				Height: 1,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ResponseDeliverTx{
					Code: abci.CodeTypeOK + 1,
				},
			},
			abci.TxResult{
				Height: 2,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ResponseDeliverTx{
					Code: abci.CodeTypeOK + 1,
				},
			},
			false,
		},
		{"not skip, both successful",
			abci.TxResult{
				Height: 1,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ResponseDeliverTx{
					Code: abci.CodeTypeOK,
				},
			},
			abci.TxResult{
				Height: 2,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ResponseDeliverTx{
					Code: abci.CodeTypeOK,
				},
			},
			false,
		},
		{"not skip, both unsuccessful",
			abci.TxResult{
				Height: 1,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ResponseDeliverTx{
					Code: abci.CodeTypeOK + 1,
				},
			},
			abci.TxResult{
				Height: 2,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ResponseDeliverTx{
					Code: abci.CodeTypeOK + 1,
				},
			},
			false,
		},
		{"skip, same block, previously successful",
			abci.TxResult{
				Height: 1,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ResponseDeliverTx{
					Code: abci.CodeTypeOK,
				},
			},
			abci.TxResult{
				Height: 1,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ResponseDeliverTx{
					Code: abci.CodeTypeOK + 1,
				},
			},
			true,
		},
		{"not skip, same block, previously unsuccessful",
			abci.TxResult{
				Height: 1,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ResponseDeliverTx{
					Code: abci.CodeTypeOK + 1,
				},
			},
			abci.TxResult{
				Height: 1,
				Index:  0,
				Tx:     mockTx,
				Result: abci.ResponseDeliverTx{
					Code: abci.CodeTypeOK,
				},
			},
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			indexer := kv.NewTxIndex(db.NewMemDB())

			if tc.tx1.Height != tc.tx2.Height {
				// index the first tx
				err := indexer.AddBatch(&txindex.Batch{
					Ops: []*abci.TxResult{&tc.tx1},
				})
				require.NoError(t, err)

				// check if the second one should be skipped.
				ops, err := txindex.DeduplicateBatch([]*abci.TxResult{&tc.tx2}, indexer)
				require.NoError(t, err)

				if tc.expSkip {
					require.Empty(t, ops)
				} else {
					require.Equal(t, []*abci.TxResult{&tc.tx2}, ops)
				}
			} else {
				// same block
				ops := []*abci.TxResult{&tc.tx1, &tc.tx2}
				ops, err := txindex.DeduplicateBatch(ops, indexer)
				require.NoError(t, err)
				if tc.expSkip {
					// the second one is skipped
					require.Equal(t, []*abci.TxResult{&tc.tx1}, ops)
				} else {
					require.Equal(t, []*abci.TxResult{&tc.tx1, &tc.tx2}, ops)
				}
			}
		})
	}
}
