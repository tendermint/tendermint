package kv

import (
	"context"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/state/indexer"
	kvb "github.com/tendermint/tendermint/state/indexer/block/kv"
	kvt "github.com/tendermint/tendermint/state/indexer/tx/kv"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

var _ indexer.EventSink = (*EventSink)(nil)

// The EventSink is an aggregator for redirecting the call path of the tx/block kvIndexer.
// For the implementation details please see the kv.go in the indexer/block and indexer/tx folder.
type EventSink struct {
	txi *kvt.TxIndex
	bi  *kvb.BlockerIndexer
}

func NewEventSink(store dbm.DB) indexer.EventSink {
	return &EventSink{
		txi: kvt.NewTxIndex(store),
		bi:  kvb.New(store),
	}
}

func (kves *EventSink) Type() indexer.EventSinkType {
	return indexer.KV
}

func (kves *EventSink) IndexBlockEvents(bh types.EventDataNewBlockHeader) error {
	return kves.bi.Index(bh)
}

func (kves *EventSink) IndexTxEvents(results []*abci.TxResult) error {
	return kves.txi.Index(results)
}

func (kves *EventSink) SearchBlockEvents(ctx context.Context, q *query.Query) ([]int64, error) {
	return kves.bi.Search(ctx, q)
}

func (kves *EventSink) SearchTxEvents(ctx context.Context, q *query.Query) ([]*abci.TxResult, error) {
	return kves.txi.Search(ctx, q)
}

func (kves *EventSink) GetTxByHash(hash []byte) (*abci.TxResult, error) {
	return kves.txi.Get(hash)
}

func (kves *EventSink) HasBlock(h int64) (bool, error) {
	return kves.bi.Has(h)
}

func (kves *EventSink) Stop() error {
	return nil
}
