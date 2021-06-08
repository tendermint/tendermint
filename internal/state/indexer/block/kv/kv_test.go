package kv_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/pubsub/query"
	blockidxkv "github.com/tendermint/tendermint/internal/state/indexer/block/kv"
	"github.com/tendermint/tendermint/types"
)

func TestBlockIndexer(t *testing.T) {
	store := dbm.NewPrefixDB(dbm.NewMemDB(), []byte("block_events"))
	indexer := blockidxkv.New(store)

	require.NoError(t, indexer.Index(types.EventDataNewBlockHeader{
		Header: types.Header{Height: 1},
		ResultFinalizeBlock: abci.ResponseFinalizeBlock{
			Events: []abci.Event{
				{
					Type: "begin_event",
					Attributes: []abci.EventAttribute{
						{
							Key:   "proposer",
							Value: "FCAA001",
							Index: true,
						},
					},
				},
			},
		},
	}))

	for i := 2; i < 12; i++ {
		require.NoError(t, indexer.Index(types.EventDataNewBlockHeader{
			Header: types.Header{Height: int64(i)},
			ResultFinalizeBlock: abci.ResponseFinalizeBlock{
				Events: []abci.Event{
					{
						Type: "finalize_event",
						Attributes: []abci.EventAttribute{
							{
								Key:   "proposer",
								Value: "FCAA001",
								Index: true,
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
			q:       query.MustCompile(`block.height = 100`),
			results: []int64{},
		},
		"block.height = 5": {
			q:       query.MustCompile(`block.height = 5`),
			results: []int64{5},
		},
		"begin_event.key1 = 'value1'": {
			q:       query.MustCompile(`begin_event.key1 = 'value1'`),
			results: []int64{},
		},
		"begin_event.proposer = 'FCAA001'": {
			q:       query.MustCompile(`begin_event.proposer = 'FCAA001'`),
			results: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
		},
		"end_event.foo <= 5": {
			q:       query.MustCompile(`end_event.foo <= 5`),
			results: []int64{2, 4},
		},
		"end_event.foo >= 100": {
			q:       query.MustCompile(`end_event.foo >= 100`),
			results: []int64{1},
		},
		"block.height > 2 AND end_event.foo <= 8": {
			q:       query.MustCompile(`block.height > 2 AND end_event.foo <= 8`),
			results: []int64{4, 6, 8},
		},
		"begin_event.proposer CONTAINS 'FFFFFFF'": {
			q:       query.MustCompile(`begin_event.proposer CONTAINS 'FFFFFFF'`),
			results: []int64{},
		},
		"begin_event.proposer CONTAINS 'FCAA001'": {
			q:       query.MustCompile(`begin_event.proposer CONTAINS 'FCAA001'`),
			results: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
		},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			results, err := indexer.Search(ctx, tc.q)
			require.NoError(t, err)
			require.Equal(t, tc.results, results)
		})
	}
}
