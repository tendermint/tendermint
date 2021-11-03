package indexer

import (
	"context"

	"github.com/tendermint/tendermint/libs/pubsub"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/types/eventbus"
)

// XXX/TODO: These types should be moved to the indexer package.

const (
	subscriber = "IndexerService"
)

// Service connects event bus, transaction and block indexers together in
// order to index transactions and blocks coming from the event bus.
type Service struct {
	service.BaseService

	eventSinks []EventSink
	eventBus   *eventbus.EventBus

	currentBlock struct {
		header types.EventDataNewBlockHeader
		height int64
		batch  *Batch
	}
}

// NewIndexerService returns a new service instance.
func NewIndexerService(es []EventSink, eventBus *eventbus.EventBus) *Service {
	is := &Service{eventSinks: es, eventBus: eventBus}
	is.BaseService = *service.NewBaseService(nil, "IndexerService", is)
	return is
}

// Publish publishes a pubsub message to the service. The service blocks until
// the message has been fully processed.
func (is *Service) Publish(msg pubsub.Message) error {
	// Indexing has three states. Initially, no block is in progress (WAIT) and
	// we expect a block header. Upon seeing a header, we are waiting for zero
	// or more transactions (GATHER). Once all the expected transactions have
	// been delivered (in some order), we are ready to index. After indexing a
	// block,

	if is.currentBlock.batch == nil {
		// WAIT: Start a new block.
		hdr := msg.Data().(types.EventDataNewBlockHeader)
		is.currentBlock.header = hdr
		is.currentBlock.height = hdr.Header.Height
		is.currentBlock.batch = NewBatch(hdr.NumTxs)

		if hdr.NumTxs != 0 {
			return nil
		}
		// If the block does not expect any transactions, fall through and index
		// it immediately.  This shouldn't happen, but this check ensures we do
		// not get stuck if it does.
	}

	curr := is.currentBlock.batch
	if curr.Pending != 0 {
		// GATHER: Accumulate a transaction into the current block's batch.
		txResult := msg.Data().(types.EventDataTx).TxResult
		if err := curr.Add(&txResult); err != nil {
			is.Logger.Error("failed to add tx to batch",
				"height", is.currentBlock.height, "index", txResult.Index, "err", err)
		}

		// This may have been the last transaction in the batch, so fall through
		// to check whether it is time to index.
	}

	if curr.Pending == 0 {
		// INDEX: We have all the transactions we expect for the current block.
		for _, sink := range is.eventSinks {
			if err := sink.IndexBlockEvents(is.currentBlock.header); err != nil {
				is.Logger.Error("failed to index block header",
					"height", is.currentBlock.height, "err", err)
			} else {
				is.Logger.Debug("indexed block",
					"height", is.currentBlock.height, "sink", sink.Type())
			}

			if curr.Size() != 0 {
				err := sink.IndexTxEvents(curr.Ops)
				if err != nil {
					is.Logger.Error("failed to index block txs",
						"height", is.currentBlock.height, "err", err)
				} else {
					is.Logger.Debug("indexed txs",
						"height", is.currentBlock.height, "sink", sink.Type())
				}
			}
		}
		is.currentBlock.batch = nil // return to the WAIT state for the next block
	}

	return nil
}

// OnStart implements service.Service by subscribing for all transactions
// and indexing them by events.
func (is *Service) OnStart() error {
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
			select {
			case <-blockHeadersSub.Canceled():
				return
			case msg := <-blockHeadersSub.Out():

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

				if !IndexingEnabled(is.eventSinks) {
					continue
				}

				for _, sink := range is.eventSinks {
					if err := sink.IndexBlockEvents(eventDataHeader); err != nil {
						is.Logger.Error("failed to index block", "height", height, "err", err)
					} else {
						is.Logger.Debug("indexed block", "height", height, "sink", sink.Type())
					}

					if len(batch.Ops) > 0 {
						err := sink.IndexTxEvents(batch.Ops)
						if err != nil {
							is.Logger.Error("failed to index block txs", "height", height, "err", err)
						} else {
							is.Logger.Debug("indexed txs", "height", height, "sink", sink.Type())
						}
					}
				}
			}
		}
	}()
	return nil
}

// OnStop implements service.Service by unsubscribing from all transactions and
// close the eventsink.
func (is *Service) OnStop() {
	if is.eventBus.IsRunning() {
		_ = is.eventBus.UnsubscribeAll(context.Background(), subscriber)
	}

	for _, sink := range is.eventSinks {
		if err := sink.Stop(); err != nil {
			is.Logger.Error("failed to close eventsink", "eventsink", sink.Type(), "err", err)
		}
	}
}

// KVSinkEnabled returns the given eventSinks is containing KVEventSink.
func KVSinkEnabled(sinks []EventSink) bool {
	for _, sink := range sinks {
		if sink.Type() == KV {
			return true
		}
	}

	return false
}

// IndexingEnabled returns the given eventSinks is supporting the indexing services.
func IndexingEnabled(sinks []EventSink) bool {
	for _, sink := range sinks {
		if sink.Type() == KV || sink.Type() == PSQL {
			return true
		}
	}

	return false
}
