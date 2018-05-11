package txindex

import (
	"context"

	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
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
		var numTxs, got int64
		var batch *Batch
		for {
			select {
			case e, ok := <-blockHeadersCh:
				if !ok {
					return
				}
				numTxs = e.(types.EventDataNewBlockHeader).Header.NumTxs
				batch = NewBatch(numTxs)
			case e, ok := <-txsCh:
				if !ok {
					return
				}
				if batch == nil {
					panic("Expected pubsub to send block header first, but got tx event")
				}
				txResult := e.(types.EventDataTx).TxResult
				batch.Add(&txResult)
				got++
				if numTxs == got {
					is.idr.AddBatch(batch)
					batch = nil
					got = 0
				}
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
