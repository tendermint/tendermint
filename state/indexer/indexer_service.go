package indexer

import (
	"context"

	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/types"
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
	eventBus   *types.EventBus
}

// NewIndexerService returns a new service instance.
func NewIndexerService(es []EventSink, eventBus *types.EventBus) *Service {

	is := &Service{eventSinks: es, eventBus: eventBus}
	is.BaseService = *service.NewBaseService(nil, "IndexerService", is)
	return is
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
