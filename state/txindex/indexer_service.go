package txindex

import (
	"context"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/types"
)

// XXX/TODO: These types should be moved to the indexer package.

const (
	subscriber = "IndexerService"
)

// IndexerService connects event bus, transaction and block indexers together in
// order to index transactions and blocks coming from the event bus.
type IndexerService struct {
	service.BaseService

	txIdxr    TxIndexer
	blockIdxr indexer.BlockIndexer
	eventBus  *types.EventBus
}

// NewIndexerService returns a new service instance.
func NewIndexerService(
	txIdxr TxIndexer,
	blockIdxr indexer.BlockIndexer,
	eventBus *types.EventBus,
) *IndexerService {

	is := &IndexerService{txIdxr: txIdxr, blockIdxr: blockIdxr, eventBus: eventBus}
	is.BaseService = *service.NewBaseService(nil, "IndexerService", is)
	return is
}

// OnStart implements service.Service by subscribing for all transactions
// and indexing them by events.
func (is *IndexerService) OnStart() error {
	// Use SubscribeUnbuffered here to ensure both subscriptions does not get
	// canceled due to not pulling messages fast enough. Cause this might
	// sometimes happen when there are no other subscribers.
	blockHeadersSub, err := is.eventBus.SubscribeUnbuffered(
		context.Background(),
		subscriber,
		types.EventQueryNewBlockHeader)
	if err != nil {
		return err
	}

	txsSub, err := is.eventBus.SubscribeUnbuffered(context.Background(), subscriber, types.EventQueryTx)
	if err != nil {
		return err
	}

	go func() {
		for {
			msg := <-blockHeadersSub.Out()
			eventDataHeader := msg.Data().(types.EventDataNewBlockHeader)
			height := eventDataHeader.Header.Height
			batch := NewBatch(eventDataHeader.NumTxs)

			for i := int64(0); i < eventDataHeader.NumTxs; i++ {
				msg2 := <-txsSub.Out()
				txResult := msg2.Data().(types.EventDataTx).TxResult

				if err = batch.Add(&txResult); err != nil {
					is.Logger.Error(
						"failed to add tx to batch",
						"height", height,
						"index", txResult.Index,
						"err", err,
					)
				}
			}

			if err := is.blockIdxr.Index(eventDataHeader); err != nil {
				is.Logger.Error("failed to index block", "height", height, "err", err)
			} else {
				is.Logger.Info("indexed block", "height", height)
			}

			batch.Ops, err = DeduplicateBatch(batch.Ops, is.txIdxr)
			if err != nil {
				is.Logger.Error("deduplicate batch", "height", height)
			}

			if err = is.txIdxr.AddBatch(batch); err != nil {
				is.Logger.Error("failed to index block txs", "height", height, "err", err)
			} else {
				is.Logger.Debug("indexed block txs", "height", height, "num_txs", eventDataHeader.NumTxs)
			}
		}
	}()
	return nil
}

// OnStop implements service.Service by unsubscribing from all transactions.
func (is *IndexerService) OnStop() {
	if is.eventBus.IsRunning() {
		_ = is.eventBus.UnsubscribeAll(context.Background(), subscriber)
	}
}

// DeduplicateBatch consider the case of duplicate txs.
// if the current one under investigation is NOT OK, then we need to check
// whether there's a previously indexed tx.
// SKIP the current tx if the previously indexed record is found and successful.
func DeduplicateBatch(ops []*abci.TxResult, txIdxr TxIndexer) ([]*abci.TxResult, error) {
	result := make([]*abci.TxResult, 0, len(ops))

	// keep track of successful txs in this block in order to suppress latter ones being indexed.
	var successfulTxsInThisBlock = make(map[string]struct{})

	for _, txResult := range ops {
		hash := types.Tx(txResult.Tx).Hash()

		if txResult.Result.IsOK() {
			successfulTxsInThisBlock[string(hash)] = struct{}{}
		} else {
			// if it already appeared in current block and was successful, skip.
			if _, found := successfulTxsInThisBlock[string(hash)]; found {
				continue
			}

			// check if this tx hash is already indexed
			old, err := txIdxr.Get(hash)

			// if db op errored
			// Not found is not an error
			if err != nil {
				return nil, err
			}

			// if it's already indexed in an older block and was successful, skip.
			if old != nil && old.Result.Code == abci.CodeTypeOK {
				continue
			}
		}

		result = append(result, txResult)
	}

	return result, nil
}
