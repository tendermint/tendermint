package psqlsink

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

var _ indexer.EventSink = (*PSQLEventSink)(nil)

const (
	TableEventBlock = "block_events"
	TableEventTx    = "tx_events"
	TableResultTx   = "tx_results"
	DriverName      = "postgres"
)

// PSQLEventSink is an indexer backend providing the tx/block index services.
type PSQLEventSink struct {
	store *sql.DB
}

func NewPSQLEventSink(connStr string) (*PSQLEventSink, error) {
	db, err := sql.Open(DriverName, connStr)
	if err != nil {
		return nil, err
	}

	return &PSQLEventSink{
		store: db,
	}, nil
}

func (es *PSQLEventSink) IndexBlockEvents(h types.EventDataNewBlockHeader) error {

	sqlStmt := sq.Insert(TableEventBlock).Columns("key", "value", "height", "type").PlaceholderFormat(sq.Dollar)

	// index the reserved block height index
	sqlStmt = sqlStmt.Values(types.BlockHeightKey, fmt.Sprint(h.Header.Height), h.Header.Height, "")

	// index begin_block events
	err := indexEvents(&sqlStmt, h.ResultBeginBlock.Events, types.EventTypeBeginBlock, h.Header.Height, nil, 0)
	if err != nil {
		return err
	}

	// index end_block events
	err = indexEvents(&sqlStmt, h.ResultEndBlock.Events, types.EventTypeEndBlock, h.Header.Height, nil, 0)
	if err != nil {
		return err
	}

	_, err = sqlStmt.RunWith(es.store).Exec()
	return err
}

func (es *PSQLEventSink) IndexTxEvents(txr *abci.TxResult) error {

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
	fmt.Println(sqlStmtEvents.ToSql())

	// execute sqlStmtEvents db query...
	_, err = sqlStmtEvents.RunWith(es.store).Exec()
	return err
}

func (es *PSQLEventSink) SearchBlockEvents(ctx context.Context, q *query.Query) ([]int64, error) {
	return nil, errors.New("block search is not supported via the postgres event sink")
}

func (es *PSQLEventSink) SearchTxEvents(ctx context.Context, q *query.Query) ([]*abci.TxResult, error) {
	return nil, errors.New("tx search is not supported via the postgres event sink")
}

func (es *PSQLEventSink) GetTxByHash(hash []byte) (*abci.TxResult, error) {

	join := fmt.Sprintf("%s ON tx_result_id = txid", TableEventTx)
	sqlStmt := sq.
		Select("tx_result", "tx_result_id", "txid", "hash").
		Distinct().From(TableResultTx).
		InnerJoin(join).
		Where("hash = $1", fmt.Sprintf("%X", hash))
	rows, err := sqlStmt.RunWith(es.store).Query()
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if rows.Next() {
		var txResult []byte
		var txResultID, txid int
		var h string
		err = rows.Scan(&txResult, &txResultID, &txid, &h)
		if err != nil {
			return nil, nil
		}

		msg := new(abci.TxResult)
		err = proto.Unmarshal(txResult, msg)
		if err != nil {
			return nil, err
		}

		return msg, err
	}

	// No result
	return nil, nil
}

func (es *PSQLEventSink) HasBlock(h int64) (bool, error) {
	sqlStmt := sq.
		Select("height").
		Distinct().
		From(TableEventBlock).
		Where(fmt.Sprintf("height = %d", h))
	rows, err := sqlStmt.RunWith(es.store).Query()
	if err != nil {
		return false, err
	}

	defer rows.Close()

	return rows.Next(), nil
}

func indexEvents(
	sqlStmt *sq.InsertBuilder,
	events []abci.Event,
	ty string,
	height int64,
	hash []byte,
	idx uint32) error {

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
			compositeKey := fmt.Sprintf("%s.%s", event.Type, string(attr.Key))
			if compositeKey == types.BlockHeightKey || compositeKey == types.TxHashKey || compositeKey == types.TxHeightKey {
				return fmt.Errorf("event type and attribute key \"%s\" is reserved; please use a different key", compositeKey)
			}

			if attr.GetIndex() {
				if hash == nil {
					*sqlStmt = sqlStmt.Values(compositeKey, string(attr.Value), height, ty)
				} else {
					*sqlStmt = sqlStmt.Values(compositeKey, string(attr.Value), height, hash, idx)
				}
			}
		}
	}
	return nil
}
