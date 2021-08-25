// Package psql implements an event sink backed by a PostgreSQL database.
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

const (
	TableEventBlock = "block_events"
	TableEventTx    = "tx_events"
	TableResultTx   = "tx_results"
	DriverName      = "postgres"
)

// EventSink is an indexer backend providing the tx/block index services.  This
// implementation stores records in a PostgreSQL database using the schema
// defined in state/indexer/sink/psql/schema.sql.
type EventSink struct {
	store   *sql.DB
	chainID string
}

// NewEventSink constructs an event sink associated with the PostgreSQL
// database specified by connStr. Events written to the sink are attributed to
// the specified chainID.
func NewEventSink(connStr, chainID string) (*EventSink, error) {
	db, err := sql.Open(DriverName, connStr)
	if err != nil {
		return nil, err
	}

	return &EventSink{
		store:   db,
		chainID: chainID,
	}, nil
}

// DB returns the underlying Postgres connection used by the sink.
// This is exported to support testing.
func (es *EventSink) DB() *sql.DB { return es.store }

// Type returns the structure type for this sink, which is Postgres.
func (es *EventSink) Type() indexer.EventSinkType { return indexer.PSQL }

// IndexBlockEvents indexes the specified block header, part of the
// indexer.EventSink interface.
func (es *EventSink) IndexBlockEvents(h types.EventDataNewBlockHeader) error {
	sqlStmt := sq.
		Insert(TableEventBlock).
		Columns("key", "value", "height", "type", "created_at", "chain_id").
		PlaceholderFormat(sq.Dollar).
		Suffix("ON CONFLICT (key,height)").
		Suffix("DO NOTHING")

	ts := time.Now()
	// index the reserved block height index
	sqlStmt = sqlStmt.
		Values(types.BlockHeightKey, fmt.Sprint(h.Header.Height), h.Header.Height, "", ts, es.chainID)

	// index begin_block events
	sqlStmt, err := indexBlockEvents(
		sqlStmt, h.ResultBeginBlock.Events, types.EventTypeBeginBlock, h.Header.Height, ts, es.chainID)
	if err != nil {
		return err
	}

	// index end_block events
	sqlStmt, err = indexBlockEvents(
		sqlStmt, h.ResultEndBlock.Events, types.EventTypeEndBlock, h.Header.Height, ts, es.chainID)
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
		Suffix("ON CONFLICT (tx_result)").
		Suffix("DO NOTHING").
		Suffix("RETURNING \"id\"")

	sqlStmtEvents := sq.
		Insert(TableEventTx).
		Columns("key", "value", "height", "hash", "tx_result_id", "created_at", "chain_id").
		PlaceholderFormat(sq.Dollar).
		Suffix("ON CONFLICT (key,hash)").
		Suffix("DO NOTHING")

	ts := time.Now()
	for _, tx := range txr {
		txBz, err := proto.Marshal(tx)
		if err != nil {
			return err
		}

		sqlStmtTxResult = sqlStmtTxResult.Values(txBz, ts)

		// execute sqlStmtTxResult db query and retrieve the txid
		r, err := sqlStmtTxResult.Query()
		if err != nil {
			return err
		}
		defer r.Close()

		if !r.Next() {
			return nil
		}

		if err := r.Scan(&txid); err != nil {
			return err
		}

		// index the reserved height and hash indices
		hash := fmt.Sprintf("%X", types.Tx(tx.Tx).Hash())

		sqlStmtEvents = sqlStmtEvents.Values(types.TxHashKey, hash, tx.Height, hash, txid, ts, es.chainID)
		sqlStmtEvents = sqlStmtEvents.Values(types.TxHeightKey, fmt.Sprint(tx.Height), tx.Height, hash, txid, ts, es.chainID)
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
					sqlStmtEvents = sqlStmtEvents.Values(compositeTag, attr.Value, tx.Height, hash, txid, ts, es.chainID)
				}
			}
		}
	}

	// execute sqlStmtEvents db query...
	_, err := sqlStmtEvents.RunWith(es.store).Exec()
	return err
}

// SearchBlockEvents is not implemented by this sink, and reports an error for all queries.
func (es *EventSink) SearchBlockEvents(ctx context.Context, q *query.Query) ([]int64, error) {
	return nil, errors.New("block search is not supported via the postgres event sink")
}

// SearchTxEvents is not implemented by this sink, and reports an error for all queries.
func (es *EventSink) SearchTxEvents(ctx context.Context, q *query.Query) ([]*abci.TxResult, error) {
	return nil, errors.New("tx search is not supported via the postgres event sink")
}

// GetTxByHash is not implemented by this sink, and reports an error for all queries.
func (es *EventSink) GetTxByHash(hash []byte) (*abci.TxResult, error) {
	return nil, errors.New("getTxByHash is not supported via the postgres event sink")
}

// HasBlock is not implemented by this sink, and reports an error for all queries.
func (es *EventSink) HasBlock(h int64) (bool, error) {
	return false, errors.New("hasBlock is not supported via the postgres event sink")
}

func indexBlockEvents(
	sqlStmt sq.InsertBuilder,
	events []abci.Event,
	ty string,
	height int64,
	ts time.Time,
	chainID string,
) (sq.InsertBuilder, error) {
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
				return sqlStmt, fmt.Errorf(
					"event type and attribute key \"%s\" is reserved; please use a different key", compositeKey)
			}

			if attr.GetIndex() {
				sqlStmt = sqlStmt.Values(compositeKey, attr.Value, height, ty, ts, chainID)
			}
		}
	}
	return sqlStmt, nil
}

// Stop closes the underlying PostgreSQL database.
func (es *EventSink) Stop() error { return es.store.Close() }
