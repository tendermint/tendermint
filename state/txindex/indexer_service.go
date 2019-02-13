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
			case msg := <-blockHeadersSub.Out():
				header := msg.Data().(types.EventDataNewBlockHeader).Header
				if header.NumTxs == 0 {
					continue
				}
				batch := NewBatch(header.NumTxs)
				i := int64(0)
			TXS_LOOP:
				for {
					select {
					case msg2 := <-txsSub.Out():
						txResult := msg2.Data().(types.EventDataTx).TxResult
						if err = batch.Add(&txResult); err != nil {
							is.Logger.Error("Can't add tx to batch",
								"height", header.Height,
								"index", txResult.Index,
								"err", err)
						}
						i++
						if i == header.NumTxs {
							break TXS_LOOP
						}
					case <-txsSub.Cancelled():
						is.Logger.Error("Failed to index block. txsSub was cancelled. Did the Tendermint stop?",
							"height", header.Height,
							"numTxs", header.NumTxs,
							"numProcessed", i,
							"err", txsSub.Err(),
						)
						return
					}
				}
				if err = is.idr.AddBatch(batch); err != nil {
					is.Logger.Error("Failed to index block", "height", header.Height, "err", err)
				} else {
					is.Logger.Info("Indexed block", "height", header.Height)
				}
			case <-blockHeadersSub.Cancelled():
				is.Logger.Error("blockHeadersSub was cancelled. Did the Tendermint stop?",
					"err", blockHeadersSub.Err())
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
