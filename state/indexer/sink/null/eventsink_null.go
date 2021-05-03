package nullsink

import (
	"context"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/types"
)

var _ indexer.EventSink = (*NullEventSink)(nil)

// NullEventSink implements a no-op indexer.
type NullEventSink struct{}

func NewNullEventSink() indexer.EventSink {
	return &NullEventSink{}
}

func (nes *NullEventSink) Type() indexer.EventSinkType {
	return indexer.NULL
}

func (nes *NullEventSink) IndexBlockEvents(bh types.EventDataNewBlockHeader) error {
	return nil
}

func (nes *NullEventSink) IndexTxEvents(result *abci.TxResult) error {
	return nil
}

func (nes *NullEventSink) SearchBlockEvents(ctx context.Context, q *query.Query) ([]int64, error) {
	return nil, nil
}

func (nes *NullEventSink) SearchTxEvents(ctx context.Context, q *query.Query) ([]*abci.TxResult, error) {
	return nil, nil
}

func (nes *NullEventSink) GetTxByHash(hash []byte) (*abci.TxResult, error) {
	return nil, nil
}

func (nes *NullEventSink) HasBlock(h int64) (bool, error) {
	return false, nil
}
