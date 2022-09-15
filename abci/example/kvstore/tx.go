package kvstore

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/tendermint/tendermint/abci/example/code"
	abci "github.com/tendermint/tendermint/abci/types"
)

const (
	VoteExtensionKey string = "extensionSum"
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
	var (
		totalBytes int64
		txRecords  []*abci.TxRecord
	)

	txs := req.Txs
	extCount := len(req.LocalLastCommit.ThresholdVoteExtensions)

	txRecords = make([]*abci.TxRecord, 0, len(txs)+1)
	extTxPrefix := VoteExtensionKey + "="
	extTx := []byte(fmt.Sprintf("%s%d", extTxPrefix, extCount))

	// app.logger.Info("preparing proposal with custom transaction from vote extensions", "tx", extTx)

	// Our generated transaction takes precedence over any supplied
	// transaction that attempts to modify the "extensionSum" value.
	for _, tx := range txs {
		// we only modify transactions if there is at least 1 extension, eg. extCount > 0
		if extCount > 0 && strings.HasPrefix(string(tx), extTxPrefix) {
			txRecords = append(txRecords, &abci.TxRecord{
				Action: abci.TxRecord_REMOVED,
				Tx:     tx,
			})
			totalBytes -= int64(len(tx))
		} else {
			txRecords = append(txRecords, &abci.TxRecord{
				Action: abci.TxRecord_UNMODIFIED,
				Tx:     tx,
			})
			totalBytes += int64(len(tx))
		}
	}
	// we only modify transactions if there is at least 1 extension, eg. extCount > 0
	if extCount > 0 {
		if totalBytes+int64(len(extTx)) < req.MaxTxBytes {
			txRecords = append(txRecords, &abci.TxRecord{
				Action: abci.TxRecord_ADDED,
				Tx:     extTx,
			})
		}
	}

	return substPrepareTx(txRecords, req.MaxTxBytes), nil
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

	k, v, err := parseTx(tx)
	if err != nil {
		return abci.ResponseCheckTx{
			Code: code.CodeTypeEncodingError,
			Data: []byte(err.Error()),
		}, nil
	}

	if k == VoteExtensionKey {
		_, err := strconv.Atoi(v)
		if err != nil {
			return abci.ResponseCheckTx{Code: code.CodeTypeUnknownError},
				fmt.Errorf("malformed vote extension transaction %X=%X: %w", k, v, err)
		}
	}
	return abci.ResponseCheckTx{Code: code.CodeTypeOK, GasWanted: 1}, nil
}

// tx is either "val:pubkey!power" or "key=value" or just arbitrary bytes
func (app *txProcessor) ExecTx(tx []byte, roundState State) (abci.ExecTxResult, error) {
	if isPrepareTx(tx) {
		return app.execPrepareTx(tx)
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
func (app *txProcessor) execPrepareTx(tx []byte) (abci.ExecTxResult, error) {
	// noop
	return abci.ExecTxResult{}, nil
}

// substPrepareTx substitutes all the transactions prefixed with 'prepare' in the
// proposal for transactions with the prefix stripped.
// It marks all of the original transactions as 'REMOVED' so that
// Tendermint will remove them from its mempool.
func substPrepareTx(records []*abci.TxRecord, maxTxBytes int64) []*abci.TxRecord {
	trs := make([]*abci.TxRecord, 0, len(records))
	var removed []*abci.TxRecord
	var totalBytes int64
	for _, record := range records {
		tx := record.Tx
		action := record.Action
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
func parseTx(tx []byte) (string, string, error) {
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
