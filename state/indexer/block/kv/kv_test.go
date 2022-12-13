package kv_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	db "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	blockidxkv "github.com/tendermint/tendermint/state/indexer/block/kv"
	"github.com/tendermint/tendermint/types"
)

func TestBlockIndexer(t *testing.T) {
	store := db.NewPrefixDB(db.NewMemDB(), []byte("block_events"))
	indexer := blockidxkv.New(store)

	require.NoError(t, indexer.Index(types.EventDataNewBlockHeader{
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

		require.NoError(t, indexer.Index(types.EventDataNewBlockHeader{
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
			results, err := indexer.Search(context.Background(), tc.q)
			require.NoError(t, err)
			require.Equal(t, tc.results, results)
		})
	}
}

func TestBlockIndexerMulti(t *testing.T) {
	store := db.NewPrefixDB(db.NewMemDB(), []byte("block_events"))
	indexer := blockidxkv.New(store)

	require.NoError(t, indexer.Index(types.EventDataNewBlockHeader{
		Header: types.Header{Height: 1},
		ResultBeginBlock: abci.ResponseBeginBlock{
			Events: []abci.Event{},
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
						{
							Key:   []byte("bar"),
							Value: []byte("200"),
							Index: true,
						},
					},
				},
				{
					Type: "end_event",
					Attributes: []abci.EventAttribute{
						{
							Key:   []byte("foo"),
							Value: []byte("300"),
							Index: true,
						},
						{
							Key:   []byte("bar"),
							Value: []byte("400"),
							Index: true,
						},
					},
				},
			},
		},
	}))

	require.NoError(t, indexer.Index(types.EventDataNewBlockHeader{
		Header: types.Header{Height: 2},
		ResultBeginBlock: abci.ResponseBeginBlock{
			Events: []abci.Event{},
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
						{
							Key:   []byte("bar"),
							Value: []byte("200"),
							Index: true,
						},
					},
				},
				{
					Type: "end_event",
					Attributes: []abci.EventAttribute{
						{
							Key:   []byte("foo"),
							Value: []byte("300"),
							Index: true,
						},
						{
							Key:   []byte("bar"),
							Value: []byte("400"),
							Index: true,
						},
					},
				},
			},
		},
	}))

	testCases := map[string]struct {
		q       *query.Query
		results []int64
	}{
		"query return all events from a height - exact": {
			q:       query.MustParse("match.events = 1 AND block.height = 1"),
			results: []int64{1},
		},
		"query return all events from a height - exact (deduplicate height)": {
			q:       query.MustParse("match.events = 1 AND block.height = 1 AND block.height = 2"),
			results: []int64{1},
		},
		"query return all events from a height - range": {
			q:       query.MustParse("match.events = 1 AND block.height < 2 AND block.height > 0 AND block.height > 0"),
			results: []int64{1},
		},
		"query matches fields from same event": {
			q:       query.MustParse("match.events = 1 AND end_event.bar < 300 AND end_event.foo = 100 AND block.height > 0 AND block.height <= 2"),
			results: []int64{1, 2},
		},
		"query matches fields from multiple events": {
			q:       query.MustParse("match.events = 1 AND end_event.foo = 100 AND end_event.bar = 400 AND block.height = 2"),
			results: []int64{},
		},
		"deduplication test - match.events only at beginning": {
			q:       query.MustParse("end_event.foo = 100 AND end_event.bar = 400 AND block.height = 2 AND match.events = 1"),
			results: []int64{2},
		},
		"deduplication test - match.events multiple": {
			q:       query.MustParse("match.events = 1 AND end_event.foo = 100 AND end_event.bar = 400 AND block.height = 2 AND match.events = 1"),
			results: []int64{},
		},
		"query matches fields from multiple events allowed": {
			q:       query.MustParse("end_event.foo = 100 AND end_event.bar = 400"),
			results: []int64{1, 2},
		},
		"query matches fields from all events whose attribute is within range": {
			q:       query.MustParse("match.events = 1 AND end_event.foo < 300 AND block.height = 2"),
			results: []int64{1, 2},
		},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			results, err := indexer.Search(context.Background(), tc.q)
			require.NoError(t, err)
			require.Equal(t, tc.results, results)
		})
	}
}
