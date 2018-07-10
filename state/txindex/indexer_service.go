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
	blockHeadersCh := make(chan interface{})
	if err := is.eventBus.Subscribe(context.Background(), subscriber, types.EventQueryNewBlockHeader, blockHeadersCh); err != nil {
		return err
	}

	txsCh := make(chan interface{})
	if err := is.eventBus.Subscribe(context.Background(), subscriber, types.EventQueryTx, txsCh); err != nil {
		return err
	}

	go func() {
		for {
			e, ok := <-blockHeadersCh
			if !ok {
				return
			}
			header := e.(types.EventDataNewBlockHeader).Header
			batch := NewBatch(header.NumTxs)
			for i := int64(0); i < header.NumTxs; i++ {
				e, ok := <-txsCh
				if !ok {
					is.Logger.Error("Failed to index all transactions due to closed transactions channel", "height", header.Height, "numTxs", header.NumTxs, "numProcessed", i)
					return
				}
				txResult := e.(types.EventDataTx).TxResult
				batch.Add(&txResult)
			}
			is.idr.AddBatch(batch)
			is.Logger.Info("Indexed block", "height", header.Height)
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
