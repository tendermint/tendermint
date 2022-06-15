package pubsub

import (
	"context"
	"errors"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/types"
)

var (
	_ indexer.BlockIndexer = (*BlockIndexer)(nil)
	_ txindex.TxIndexer    = (*TxIndexer)(nil)
)

// BlockIndexer implements a wrapper around the Pubsub sink and supports block
// indexing by implementing the indexer.BlockIndexer interface.
type BlockIndexer struct {
	sink *EventSink
}

func NewBlockIndexer(sink *EventSink) *BlockIndexer {
	return &BlockIndexer{sink: sink}
}

func (bi *BlockIndexer) Has(_ int64) (bool, error) {
	return false, errors.New("the Has method is not supported for the Pubsub indexer")
}

func (bi *BlockIndexer) Search(_ context.Context, _ *query.Query) ([]int64, error) {
	return nil, errors.New("the Search method is not supported for the Pubsub indexer")
}

func (bi *BlockIndexer) Index(block types.EventDataNewBlockHeader) error {
	return bi.sink.IndexBlock(block)
}

// TxIndexer implements a wrapper around the Pubsub sink and supports tx
// indexing by implementing the txindex.TxIndexer interface.
type TxIndexer struct {
	sink *EventSink
}

func NewTxIndexer(sink *EventSink) *TxIndexer {
	return &TxIndexer{sink: sink}
}

func (ti *TxIndexer) AddBatch(batch *txindex.Batch) error {
	return ti.sink.IndexTxs(batch.Ops)
}

func (ti *TxIndexer) Index(txr *abci.TxResult) error {
	return ti.sink.IndexTxs([]*abci.TxResult{txr})
}

func (ti *TxIndexer) Get(hash []byte) (*abci.TxResult, error) {
	return nil, errors.New("the Get method is not supported for the Pubsub indexer")
}

func (ti *TxIndexer) Search(_ context.Context, _ *query.Query) ([]*abci.TxResult, error) {
	return nil, errors.New("the Search method is not supported for the Pubsub indexer")
}
