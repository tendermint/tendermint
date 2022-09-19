package kvstore

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/abci/example/code"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/types"
)

// PrepareTxsFunc prepares transactions, possibly adding and/or removing some of them
type PrepareTxsFunc func(req abci.RequestPrepareProposal) ([]*abci.TxRecord, error)

// VerifyTxFunc checks if transaction is correct
type VerifyTxFunc func(tx types.Tx, typ abci.CheckTxType) (abci.ResponseCheckTx, error)

// ExecTxFunc executes the transaction against some state
type ExecTxFunc func(tx types.Tx, roundState State) (abci.ExecTxResult, error)

func prepareTxs(req abci.RequestPrepareProposal) ([]*abci.TxRecord, error) {

	return substPrepareTx(req.Txs, req.MaxTxBytes), nil
}

func txRecords2Txs(txRecords []*abci.TxRecord) types.Txs {
	txRecordSet := types.NewTxRecordSet(txRecords)
	return txRecordSet.IncludedTxs()
}

func verifyTx(tx types.Tx, _ abci.CheckTxType) (abci.ResponseCheckTx, error) {

	_, _, err := parseTx(tx)
	if err != nil {
		return abci.ResponseCheckTx{
			Code: code.CodeTypeEncodingError,
			Data: []byte(err.Error()),
		}, nil
	}

	return abci.ResponseCheckTx{Code: code.CodeTypeOK, GasWanted: 1}, nil
}

// tx is either "val:pubkey!power" or "key=value" or just arbitrary bytes
func execTx(tx types.Tx, roundState State) (abci.ExecTxResult, error) {
	if isPrepareTx(tx) {
		return execPrepareTx(tx)
	}

	key, value, err := parseTx(tx)
	if err != nil {
		err = fmt.Errorf("malformed transaction %X in ProcessProposal: %w", tx, err)
		return abci.ExecTxResult{Code: code.CodeTypeUnknownError, Log: err.Error()}, err
	}

	err = roundState.Set(prefixKey([]byte(key)), []byte(value))
	if err != nil {
		return abci.ExecTxResult{Code: code.CodeTypeUnknownError, Log: err.Error()}, err
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
func execPrepareTx(tx []byte) (abci.ExecTxResult, error) {
	// noop
	return abci.ExecTxResult{}, nil
}

// substPrepareTx substitutes all the transactions prefixed with 'prepare' in the
// proposal for transactions with the prefix stripped.
// It marks all of the original transactions as 'REMOVED' so that
// Tendermint will remove them from its mempool.
func substPrepareTx(txs [][]byte, maxTxBytes int64) []*abci.TxRecord {
	trs := make([]*abci.TxRecord, 0, len(txs))
	var removed []*abci.TxRecord
	var totalBytes int64
	for _, item := range txs {
		tx := item
		action := abci.TxRecord_UNMODIFIED
		if isPrepareTx(tx) {
			// replace tx and add it as REMOVED
			if action != abci.TxRecord_ADDED {
				removed = append(removed, &abci.TxRecord{
					Tx:     tx,
					Action: abci.TxRecord_REMOVED,
				})
				totalBytes -= int64(len(tx))
			}
			tx = bytes.TrimPrefix(tx, []byte(PreparePrefix))
			action = abci.TxRecord_ADDED
		}
		totalBytes += int64(len(tx))
		if totalBytes > maxTxBytes {
			break
		}
		trs = append(trs, &abci.TxRecord{
			Tx:     tx,
			Action: action,
		})
	}

	return append(trs, removed...)
}

const PreparePrefix = "prepare"

func isPrepareTx(tx []byte) bool {
	return bytes.HasPrefix(tx, []byte(PreparePrefix))
}

// parseTx parses a tx in 'key=value' format into a key and value.
func parseTx(tx types.Tx) (string, string, error) {
	parts := bytes.SplitN(tx, []byte("="), 2)
	switch len(parts) {
	case 0:
		return "", "", errors.New("key cannot be empty")
	case 1:
		return string(parts[0]), string(parts[0]), nil
	case 2:
		return string(parts[0]), string(parts[1]), nil
	default:
		return "", "", fmt.Errorf("invalid tx format: %q", string(tx))
	}
}
