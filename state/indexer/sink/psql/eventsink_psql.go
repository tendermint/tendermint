package psqlsink

import (
	"context"
	"database/sql"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/pubsub/query"
	"github.com/tendermint/tendermint/state/indexer"
	"github.com/tendermint/tendermint/types"
)

var _ indexer.EventSink = (*PSQLEventSink)(nil)

// PSQLEventSink is an indexer backend providing the tx/block index services.
type PSQLEventSink struct {
	store *sql.DB
}

func NewPSQLEventSink(connStr string) (*PSQLEventSink, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	return &PSQLEventSink{
		store: db,
	}, nil
}

func (es *PSQLEventSink) IndexBlockEvents(bh types.EventDataNewBlockHeader) error {
	return nil
}

func (es *PSQLEventSink) IndexTxEvents(result *abci.TxResult) error {
	return nil
}

func (es *PSQLEventSink) SearchBlockEvents(ctx context.Context, q *query.Query) ([]int64, error) {
	return nil, nil
}

func (es *PSQLEventSink) SearchTxEvents(ctx context.Context, q *query.Query) ([]*abci.TxResult, error) {
	return nil, nil
}

func (es *PSQLEventSink) GetTxByHash(hash []byte) (*abci.TxResult, error) {
	return nil, nil
}

func (es *PSQLEventSink) HasBlock(h int64) (bool, error) {
	return false, nil
}
