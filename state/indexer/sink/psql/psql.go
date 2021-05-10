package psql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
	sqlStmt := sq.Insert(TableEventBlock).Columns("key", "value", "height", "type").PlaceholderFormat(sq.Dollar)

	// index the reserved block height index
	sqlStmt = sqlStmt.Values(types.BlockHeightKey, fmt.Sprint(h.Header.Height), h.Header.Height, "")

	// index begin_block events
	err := indexBlockEvents(&sqlStmt, h.ResultBeginBlock.Events, types.EventTypeBeginBlock, h.Header.Height)
	if err != nil {
		return err
	}

	// index end_block events
	err = indexBlockEvents(&sqlStmt, h.ResultEndBlock.Events, types.EventTypeEndBlock, h.Header.Height)
	if err != nil {
		return err
	}

	_, err = sqlStmt.RunWith(es.store).Exec()
	return err
}

func (es *EventSink) IndexTxEvents(txr *abci.TxResult) error {
	// index the tx result
	var txid uint32
	sqlStmtTxResult := sq.
		Insert(TableResultTx).
		Columns("tx_result").
		PlaceholderFormat(sq.Dollar).
		RunWith(es.store).
		Suffix("RETURNING \"tx_result_id\"")
	txBz, err := proto.Marshal(txr)
	if err != nil {
		return err
	}

	sqlStmtTxResult = sqlStmtTxResult.Values(txBz)

	// execute sqlStmtTxResult db query and retrieve the txid
	err = sqlStmtTxResult.QueryRow().Scan(&txid)
	if err != nil {
		return err
	}

	// index the reserved height and hash indices
	hash := fmt.Sprintf("%X", types.Tx(txr.Tx).Hash())

	sqlStmtEvents := sq.
		Insert(TableEventTx).
		Columns("key", "value", "height", "hash", "txid").
		PlaceholderFormat(sq.Dollar)
	sqlStmtEvents = sqlStmtEvents.Values(types.TxHashKey, hash, txr.Height, hash, txid)
	sqlStmtEvents = sqlStmtEvents.Values(types.TxHeightKey, fmt.Sprint(txr.Height), txr.Height, hash, txid)

	// execute sqlStmtEvents db query...
	_, err = sqlStmtEvents.RunWith(es.store).Exec()
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

func indexBlockEvents(sqlStmt *sq.InsertBuilder, events []abci.Event, ty string, height int64) error {

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
				*sqlStmt = sqlStmt.Values(compositeKey, attr.Value, height, ty)
			}
		}
	}
	return nil
}
