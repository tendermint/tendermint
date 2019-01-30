package txindex

import (
	"context"

	cmn "github.com/tendermint/tendermint/libs/common"

	"github.com/tendermint/tendermint/types"
)

const (
	subscriber = "IndexerService"
)

// IndexerService connects event bus and transaction indexer together in order
// to index transactions coming from event bus.
type IndexerService struct {
	cmn.BaseService

	idr      TxIndexer
	eventBus *types.EventBus
}

// NewIndexerService returns a new service instance.
func NewIndexerService(idr TxIndexer, eventBus *types.EventBus) *IndexerService {
	is := &IndexerService{idr: idr, eventBus: eventBus}
	is.BaseService = *cmn.NewBaseService(nil, "IndexerService", is)
	return is
}

// OnStart implements cmn.Service by subscribing for all transactions
// and indexing them by tags.
func (is *IndexerService) OnStart() error {
	blockHeadersSub, err := is.eventBus.Subscribe(context.Background(), subscriber, types.EventQueryNewBlockHeader)
	if err != nil {
		return err
	}

	txsSub, err := is.eventBus.Subscribe(context.Background(), subscriber, types.EventQueryTx)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case mt := <-blockHeadersSub.Out():
				header := mt.Msg().(types.EventDataNewBlockHeader).Header
				batch := NewBatch(header.NumTxs)
				for i := int64(0); i < header.NumTxs; i++ {
					select {
					case mt2 := <-txsSub.Out():
						txResult := mt2.Msg().(types.EventDataTx).TxResult
						batch.Add(&txResult)
					case <-txsSub.Cancelled():
						is.Logger.Error("Failed to index a block. txsSub was cancelled. Did the Tendermint stop?",
							"err", txsSub.Err(),
							"height", header.Height,
							"numTxs", header.NumTxs,
							"numProcessed", i,
						)
						return
					}
				}
				is.idr.AddBatch(batch)
				is.Logger.Info("Indexed block", "height", header.Height)
			case <-blockHeadersSub.Cancelled():
				is.Logger.Error("Failed to index a block. blockHeadersSub was cancelled. Did the Tendermint stop?",
					"reason", blockHeadersSub.Err())
				return
			}
		}
	}()
	return nil
}

// OnStop implements cmn.Service by unsubscribing from all transactions.
func (is *IndexerService) OnStop() {
	if is.eventBus.IsRunning() {
		_ = is.eventBus.UnsubscribeAll(context.Background(), subscriber)
	}
}
