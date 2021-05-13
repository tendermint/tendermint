package psql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	proto "github.com/gogo/protobuf/proto"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/types"
)

var _ indexer.EventSink = (*EventSink)(nil)

const (
	TableEventBlock = "block_events"
	TableEventTx    = "tx_events"
	TableResultTx   = "tx_results"
	DriverName      = "postgres"
)

// EventSink is an indexer backend providing the tx/block index services.
type EventSink struct {
	store *sql.DB
}

func NewEventSink(connStr string) (indexer.EventSink, *sql.DB, error) {
	db, err := sql.Open(DriverName, connStr)
	if err != nil {
		return nil, nil, err
	}

	return &EventSink{
		store: db,
	}, db, nil
}

func (es *EventSink) Type() indexer.EventSinkType {
	return indexer.PSQL
}

func (es *EventSink) IndexBlockEvents(h types.EventDataNewBlockHeader) error {
	sqlStmt := sq.
		Insert(TableEventBlock).Columns("key", "value", "height", "type", "created_at").PlaceholderFormat(sq.Dollar)

	ts := time.Now()
	// index the reserved block height index
	sqlStmt = sqlStmt.Values(types.BlockHeightKey, fmt.Sprint(h.Header.Height), h.Header.Height, "", ts)

	// index begin_block events
	err := indexBlockEvents(&sqlStmt, h.ResultBeginBlock.Events, types.EventTypeBeginBlock, h.Header.Height, ts)
	if err != nil {
		return err
	}

	// index end_block events
	err = indexBlockEvents(&sqlStmt, h.ResultEndBlock.Events, types.EventTypeEndBlock, h.Header.Height, ts)
	if err != nil {
		return err
	}

	_, err = sqlStmt.RunWith(es.store).Exec()
	return err
}

func (es *EventSink) IndexTxEvents(txr []*abci.TxResult) error {
	// index the tx result
	var txid uint32
	sqlStmtTxResult := sq.
		Insert(TableResultTx).
		Columns("tx_result", "created_at").
		PlaceholderFormat(sq.Dollar).
		RunWith(es.store).
		Suffix("RETURNING \"id\"")

	sqlStmtEvents := sq.
		Insert(TableEventTx).
		Columns("key", "value", "height", "hash", "tx_result_id", "created_at").
		PlaceholderFormat(sq.Dollar)

	ts := time.Now()
	for _, tx := range txr {
		txBz, err := proto.Marshal(tx)
		if err != nil {
			return err
		}

		sqlStmtTxResult = sqlStmtTxResult.Values(txBz, ts)

		// execute sqlStmtTxResult db query and retrieve the txid
		err = sqlStmtTxResult.QueryRow().Scan(&txid)
		if err != nil {
			return err
		}

		// index the reserved height and hash indices
		hash := fmt.Sprintf("%X", types.Tx(tx.Tx).Hash())

		sqlStmtEvents = sqlStmtEvents.Values(types.TxHashKey, hash, tx.Height, hash, txid, ts)
		sqlStmtEvents = sqlStmtEvents.Values(types.TxHeightKey, fmt.Sprint(tx.Height), tx.Height, hash, txid, ts)

		for _, event := range tx.Result.Events {
			// only index events with a non-empty type
			if len(event.Type) == 0 {
				continue
			}

			for _, attr := range event.Attributes {
				if len(attr.Key) == 0 {
					continue
				}

				// index if `index: true` is set
				compositeTag := fmt.Sprintf("%s.%s", event.Type, attr.Key)

				// ensure event does not conflict with a reserved prefix key
				if compositeTag == types.TxHashKey || compositeTag == types.TxHeightKey {
					return fmt.Errorf("event type and attribute key \"%s\" is reserved; please use a different key", compositeTag)
				}

				if attr.GetIndex() {
					sqlStmtEvents = sqlStmtEvents.Values(compositeTag, attr.Value, tx.Height, hash, txid, ts)
				}
			}
		}
	}

	// execute sqlStmtEvents db query...
	_, err := sqlStmtEvents.RunWith(es.store).Exec()
	return err
}

func (es *EventSink) SearchBlockEvents(ctx context.Context, q *query.Query) ([]int64, error) {
	return nil, errors.New("block search is not supported via the postgres event sink")
}

func (es *EventSink) SearchTxEvents(ctx context.Context, q *query.Query) ([]*abci.TxResult, error) {
	return nil, errors.New("tx search is not supported via the postgres event sink")
}

func (es *EventSink) GetTxByHash(hash []byte) (*abci.TxResult, error) {
	return nil, errors.New("getTxByHash is not supported via the postgres event sink")
}

func (es *EventSink) HasBlock(h int64) (bool, error) {
	return false, errors.New("hasBlock is not supported via the postgres event sink")
}

func indexBlockEvents(sqlStmt *sq.InsertBuilder, events []abci.Event, ty string, height int64, ts time.Time) error {

	for _, event := range events {
		// only index events with a non-empty type
		if len(event.Type) == 0 {
			continue
		}

		for _, attr := range event.Attributes {
			if len(attr.Key) == 0 {
				continue
			}

			// index iff the event specified index:true and it's not a reserved event
			compositeKey := fmt.Sprintf("%s.%s", event.Type, attr.Key)
			if compositeKey == types.BlockHeightKey {
				return fmt.Errorf("event type and attribute key \"%s\" is reserved; please use a different key", compositeKey)
			}

			if attr.GetIndex() {
				*sqlStmt = sqlStmt.Values(compositeKey, attr.Value, height, ty, ts)
			}
		}
	}
	return nil
}

func (es *EventSink) Stop() error {
	return es.store.Close()
}
