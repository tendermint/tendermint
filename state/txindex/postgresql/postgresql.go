package postgresql

import (
	"database/sql/driver"

	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/types"
)

type TxIndex struct {
	db driver.Conn
}

// NewTxIndex returns new instance of TxIndex.
func NewTxIndex(db driver.Conn) *TxIndex {
	return &TxIndex{db: db}
}

// Get gets transaction from the TxIndex db and returns it or nil if the
// transaction is not found.
func (txi *TxIndex) Get(hash []byte) (*types.TxResult, error) {
	if len(hash) == 0 {
		return nil, txindex.ErrorEmptyHash
	}

	rows, err := txi.db.Query("SELECT height, index, tx, result_data, result_code, result_log FROM txs WHERE hash = $1 LIMIT 0, 1", hash)

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
			abci.ResponseDeliverTx{resultData, abci.CodeType(resultCode), resultLog}}, nil
	}

	return nil, txindex.ErrorNotFound
}

// Batch writes a batch of transactions into the TxIndex storage.
func (txi *TxIndex) AddBatch(b *txindex.Batch) error {
	txn, err := txi.db.Begin()
	if err != nil {
		return errors.Wrap(err, "failed to begin PG transaction")
	}

	stmt, err := txn.Prepare(pq.CopyIn("txs", "height", "index", "tx", "result_data", "result_code", "result_log"))
	if err != nil {
		return errors.Wrap(err, "failed to prepare PG statement")
	}

	for _, txResult := range b {
		_, err = stmt.Exec(int64(txResult.Height), int64(txResult.Index), txResult.Tx, txResult.Result.Data, int64(txResult.Result.Code), txResult.Result.Log)
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
