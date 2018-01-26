package postgresql

import (
	"database/sql"
	"fmt"

	"github.com/lib/pq"
	"github.com/pkg/errors"

	abci "github.com/tendermint/abci/types"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/types"
)

type TxIndex struct {
	db *sql.DB
}

// NewTxIndex returns new instance of TxIndex.
func NewTxIndex(db *sql.DB) *TxIndex {
	return &TxIndex{db: db}
}

// Get gets transaction from the TxIndex db and returns it or nil if the
// transaction is not found.
func (txi *TxIndex) Get(hash []byte) (*types.TxResult, error) {
	if len(hash) == 0 {
		return nil, txindex.ErrorEmptyHash
	}

	rows, err := txi.db.Query("SELECT height, index, tx, result_data, result_code, result_log FROM txs WHERE hash = $1 LIMIT 1", fmt.Sprintf("%X", hash))
	if err != nil {
		return nil, errors.Wrap(err, "failed to query PG")
	}
	defer rows.Close()

	for rows.Next() {
		var height int64
		var index int64
		var tx []byte
		var resultData []byte
		var resultCode int64
		var resultLog string
		err = rows.Scan(&height, &index, &tx, &resultData, &resultCode, &resultLog)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan row in PG")
		}
		return &types.TxResult{
			uint64(height),
			uint32(index),
			tx,
			abci.ResponseDeliverTx{abci.CodeType(resultCode), resultData, resultLog}}, nil
	}

	return nil, txindex.ErrorNotFound
}

// Batch writes a batch of transactions into the TxIndex storage.
func (txi *TxIndex) AddBatch(b *txindex.Batch) error {
	txn, err := txi.db.Begin()
	if err != nil {
		return errors.Wrap(err, "failed to begin PG transaction")
	}

	stmt, err := txn.Prepare(pq.CopyIn("txs", "hash", "height", "index", "tx", "result_data", "result_code", "result_log"))
	if err != nil {
		return errors.Wrap(err, "failed to prepare PG statement")
	}

	for _, result := range b.Ops {
		_, err = stmt.Exec(fmt.Sprintf("%X", result.Tx.Hash()), int64(result.Height), int64(result.Index), result.Tx, result.Result.Data, int64(result.Result.Code), result.Result.Log)
		if err != nil {
			return errors.Wrap(err, "failed to execute PG statement")
		}
	}

	_, err = stmt.Exec()
	if err != nil {
		return errors.Wrap(err, "failed to execute PG statement")
	}

	err = stmt.Close()
	if err != nil {
		return errors.Wrap(err, "failed to close PG statement")
	}

	err = txn.Commit()
	if err != nil {
		return errors.Wrap(err, "failed to commit PG transaction")
	}

	return nil
}
