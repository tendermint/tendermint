package app

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	db "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	types1 "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

const (
	voteExtensionKey    string = "extensionSum"
	voteExtensionMaxVal int64  = 128
)

// Application is an ABCI application for use by end-to-end tests. It is a
// simple key/value store for strings, storing data in memory and persisting
// to disk as JSON, taking state sync snapshots if requested.
type Application struct {
	*kvstore.Application
	mu sync.Mutex

	logger          log.Logger
	cfg             *kvstore.Config
	restoreSnapshot *abci.Snapshot
	restoreChunks   [][]byte
}

// NewApplication creates the application.
func NewApplication(cfg kvstore.Config) (*Application, error) {
	logger, err := log.NewDefaultLogger(log.LogFormatPlain, log.LogLevelDebug)
	if err != nil {
		return nil, err
	}

	db, err := db.NewGoLevelDB("app_state", filepath.Join(cfg.Dir, "app_state"))
	if err != nil {
		return nil, err
	}

	app := Application{
		logger: logger.With("module", "abci_app"),
	}
	app.Application = kvstore.NewApplication(
		kvstore.WithLogger(logger),
		kvstore.WithConfig(cfg),
		kvstore.WithStateStore(kvstore.NewDBStateStore(db)),
		kvstore.WithTxProcessor(&app),
	)

	return &app, nil
}

// CheckTx implements ABCI.
func (app *Application) VerifyTx(tx []byte, _ abci.CheckTxType) (abci.ResponseCheckTx, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	_, _, err := parseTx(tx)
	if err != nil {
		return abci.ResponseCheckTx{
			Code: code.CodeTypeEncodingError,
		}, nil
	}

	return abci.ResponseCheckTx{Code: code.CodeTypeOK, GasWanted: 1}, nil
}

// PrepareProposal will take the given transactions and attempt to prepare a
// proposal from them when it's our turn to do so. In the process, vote
// extensions from the previous round of consensus, if present, will be used to
// construct a special transaction whose value is the sum of all of the vote
// extensions from the previous round.
//
// NB: Assumes that the supplied transactions do not exceed `req.MaxTxBytes`.
// If adding a special vote extension-generated transaction would cause the
// total number of transaction bytes to exceed `req.MaxTxBytes`, we will not
// append our special vote extension transaction.
//func (app *Application) PrepareProposal(_ context.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {

func (app *Application) PrepareTxs(req abci.RequestPrepareProposal) ([]*abci.TxRecord, error) {
	var (
		totalBytes int64
		txRecords  []*abci.TxRecord
	)

	txs := req.Txs
	extCount := len(req.LocalLastCommit.ThresholdVoteExtensions)

	txRecords = make([]*abci.TxRecord, 0, len(txs)+1)
	extTxPrefix := voteExtensionKey + "="
	extTx := []byte(fmt.Sprintf("%s%d", extTxPrefix, extCount))

	app.logger.Info("preparing proposal with custom transaction from vote extensions", "tx", extTx)

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
	} else {
		app.logger.Info(
			"too many txs to include special vote extension-generated tx",
			"totalBytes", totalBytes,
			"MaxTxBytes", req.MaxTxBytes,
			"extTx", extTx,
			"extTxLen", len(extTx),
		)
	}

	return txRecords, nil
}
func (app *Application) ExecTx(tx []byte, roundState kvstore.State) (abci.ExecTxResult, error) {
	k, v, err := parseTx(tx)
	if err != nil {
		return abci.ExecTxResult{Code: code.CodeTypeUnknownError},
			fmt.Errorf("malformed transaction %X in ProcessProposal: %w", tx, err)
	}
	// Additional check for vote extension-related txs
	if k == voteExtensionKey {
		_, err := strconv.Atoi(v)
		if err != nil {
			return abci.ExecTxResult{Code: code.CodeTypeUnknownError},
				fmt.Errorf("malformed vote extension transaction %X=%X: %w", k, v, err)
		}
	}

	return abci.ExecTxResult{
		Code:    code.CodeTypeOK,
		GasUsed: 1,
	}, nil
}

// validatorUpdates generates a validator set update.
func (app *Application) chainLockUpdate(height uint64) (*types1.CoreChainLock, error) {
	updates := app.cfg.ChainLockUpdates[fmt.Sprintf("%v", height)]
	if len(updates) == 0 {
		return nil, nil
	}

	chainLockUpdateString := app.cfg.ChainLockUpdates[fmt.Sprintf("%v", height)]
	if len(chainLockUpdateString) == 0 {
		return nil, fmt.Errorf("chainlockUpdate must be set")
	}
	chainlockUpdateHeight, err := strconv.Atoi(chainLockUpdateString)
	if err != nil {
		return nil, fmt.Errorf("invalid number chainlockUpdate value %q: %w", chainLockUpdateString, err)
	}
	chainLock := types.NewMockChainLock(uint32(chainlockUpdateHeight))
	return chainLock.ToProto(), nil

}

// parseTx parses a tx in 'key=value' format into a key and value.
func parseTx(tx []byte) (string, string, error) {
	parts := bytes.Split(tx, []byte("="))
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid tx format: %q", string(tx))
	}
	if len(parts[0]) == 0 {
		return "", "", errors.New("key cannot be empty")
	}
	return string(parts[0]), string(parts[1]), nil
}

// parseVoteExtension attempts to parse the given extension data into a positive
// integer value.
func parseVoteExtension(ext []byte) (int64, error) {
	num, errVal := binary.Varint(ext)
	if errVal == 0 {
		return 0, errors.New("vote extension is too small to parse")
	}
	if errVal < 0 {
		return 0, errors.New("vote extension value is too large")
	}
	if num >= voteExtensionMaxVal {
		return 0, fmt.Errorf("vote extension value must be smaller than %d (was %d)", voteExtensionMaxVal, num)
	}
	return num, nil
}
