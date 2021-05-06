package indexer

import (
	"context"
	"database/sql"

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
	sqlDB      *sql.DB
}

// NewIndexerService returns a new service instance.
func NewIndexerService(
	es []EventSink,
	eventBus *types.EventBus,
	sqlDB *sql.DB,
) *Service {

	is := &Service{eventSinks: es, eventBus: eventBus, sqlDB: sqlDB}
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

			for _, sink := range is.eventSinks {
				if err := sink.IndexBlockEvents(eventDataHeader); err != nil {
					is.Logger.Error("failed to index block", "height", height, "err", err)
				} else {
					is.Logger.Info("indexed block", "height", height)
				}

				for _, result := range batch.Ops {
					err := sink.IndexTxEvents(result)
					if err != nil {
						is.Logger.Error("failed to index block txs", "height", height, "err", err)
					}
				}
			}
		}
	}()
	return nil
}

// OnStop implements service.Service by unsubscribing from all transactions and
// close the sqlDB if the service is using the psqlEventSink
func (is *Service) OnStop() {
	if is.eventBus.IsRunning() {
		_ = is.eventBus.UnsubscribeAll(context.Background(), subscriber)
	}

	if is.sqlDB != nil {
		err := is.sqlDB.Close()
		is.Logger.Error("failed to close the sqlDB", "err", err)
	}
}
