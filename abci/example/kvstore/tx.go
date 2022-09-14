package kvstore

import (
	"bytes"

	"github.com/tendermint/tendermint/abci/example/code"
	abci "github.com/tendermint/tendermint/abci/types"
)

type TxProcessor interface {
	// PrepareTxs prepares transactions, possibly adding and/or removing some of them
	PrepareTxs(req abci.RequestPrepareProposal) ([]*abci.TxRecord, error)
	// VerifyTx checks if transaction is correct
	VerifyTx(tx []byte, typ abci.CheckTxType) (abci.ResponseCheckTx, error)
	// Exec executes the transaction against some state
	ExecTx(tx []byte, roundState State) (abci.ExecTxResult, error)
}

type txProcessor struct{}

var _ = TxProcessor(&txProcessor{})

func (app *txProcessor) PrepareTxs(req abci.RequestPrepareProposal) ([]*abci.TxRecord, error) {
	return substPrepareTx(req.Txs, req.MaxTxBytes), nil
}

func txs2TxRecords(txs [][]byte) []*abci.TxRecord {
	ret := make([]*abci.TxRecord, 0, len(txs))
	for _, tx := range txs {
		ret = append(ret, &abci.TxRecord{
			Action: abci.TxRecord_UNMODIFIED,
			Tx:     tx,
		})
	}
	return ret
}

func (app *txProcessor) VerifyTx(tx []byte, _ abci.CheckTxType) (abci.ResponseCheckTx, error) {
	return abci.ResponseCheckTx{Code: code.CodeTypeOK, GasWanted: 1}, nil
}

// tx is either "val:pubkey!power" or "key=value" or just arbitrary bytes
func (app *txProcessor) ExecTx(tx []byte, roundState State) (abci.ExecTxResult, error) {
	if isPrepareTx(tx) {
		return app.execPrepareTx(tx)
	}

	var key, value string
	parts := bytes.Split(tx, []byte("="))
	if len(parts) == 2 {
		key, value = string(parts[0]), string(parts[1])
	} else {
		key, value = string(tx), string(tx)
	}

	err := roundState.Set(prefixKey([]byte(key)), []byte(value))
	if err != nil {
		return abci.ExecTxResult{}, err
	}

	events := []abci.Event{
		{
			Type: "app",
			Attributes: []abci.EventAttribute{
				{Key: "creator", Value: "Cosmoshi Netowoko", Index: true},
				{Key: "key", Value: key, Index: true},
				{Key: "index_key", Value: "index is working", Index: true},
				{Key: "noindex_key", Value: "index is working", Index: false},
			},
		},
	}

	return abci.ExecTxResult{Code: code.CodeTypeOK, Events: events}, nil
}

// execPrepareTx is noop. tx data is considered as placeholder
// and is substitute at the PrepareProposal.
func (app *txProcessor) execPrepareTx(tx []byte) (abci.ExecTxResult, error) {
	// noop
	return abci.ExecTxResult{}, nil
}

// substPrepareTx substitutes all the transactions prefixed with 'prepare' in the
// proposal for transactions with the prefix stripped.
// It marks all of the original transactions as 'REMOVED' so that
// Tendermint will remove them from its mempool.
func substPrepareTx(blockData [][]byte, maxTxBytes int64) []*abci.TxRecord {
	trs := make([]*abci.TxRecord, 0, len(blockData))
	var removed []*abci.TxRecord
	var totalBytes int64
	for _, tx := range blockData {
		txMod := tx
		action := abci.TxRecord_UNMODIFIED
		if isPrepareTx(tx) {
			removed = append(removed, &abci.TxRecord{
				Tx:     tx,
				Action: abci.TxRecord_REMOVED,
			})
			txMod = bytes.TrimPrefix(tx, []byte(PreparePrefix))
			action = abci.TxRecord_ADDED
		}
		totalBytes += int64(len(txMod))
		if totalBytes > maxTxBytes {
			break
		}
		trs = append(trs, &abci.TxRecord{
			Tx:     txMod,
			Action: action,
		})
	}

	return append(trs, removed...)
}

const PreparePrefix = "prepare"

func isPrepareTx(tx []byte) bool {
	return bytes.HasPrefix(tx, []byte(PreparePrefix))
}
