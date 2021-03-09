package null

import (
	"context"

	"github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/types"
)

var _ indexer.BlockIndexer = (*BlockerIndexer)(nil)

// TxIndex implements a no-op block indexer.
type BlockerIndexer struct{}

// Index indexes BeginBlock and EndBlock events for a given block by its height.
func (idx *BlockerIndexer) Index(types.EventDataNewBlockHeader) error {
	return nil
}

// Search performs a query for block heights that match a given BeginBlock
// and Endblock event search criteria.
func (idx *BlockerIndexer) Search(ctx context.Context, q *query.Query) ([]int64, error) {
	return nil, nil
}
