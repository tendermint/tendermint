package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/version"
)

const (
	appVersion                 = 1
	voteExtensionKey    string = "extensionSum"
	voteExtensionMaxVal int64  = 128
)

// Application is an ABCI application for use by end-to-end tests. It is a
// simple key/value store for strings, storing data in memory and persisting
// to disk as JSON, taking state sync snapshots if requested.
type Application struct {
	abci.BaseApplication
	logger          log.Logger
	state           *State
	snapshots       *SnapshotStore
	cfg             *Config
	restoreSnapshot *abci.Snapshot
	restoreChunks   [][]byte
}

// Config allows for the setting of high level parameters for running the e2e Application
// KeyType and ValidatorUpdates must be the same for all nodes running the same application.
type Config struct {
	// The directory with which state.json will be persisted in. Usually $HOME/.tendermint/data
	Dir string `toml:"dir"`

	// SnapshotInterval specifies the height interval at which the application
	// will take state sync snapshots. Defaults to 0 (disabled).
	SnapshotInterval uint64 `toml:"snapshot_interval"`

	// RetainBlocks specifies the number of recent blocks to retain. Defaults to
	// 0, which retains all blocks. Must be greater that PersistInterval,
	// SnapshotInterval and EvidenceAgeHeight.
	RetainBlocks uint64 `toml:"retain_blocks"`

	// KeyType sets the curve that will be used by validators.
	// Options are ed25519 & secp256k1
	KeyType string `toml:"key_type"`

	// PersistInterval specifies the height interval at which the application
	// will persist state to disk. Defaults to 1 (every height), setting this to
	// 0 disables state persistence.
	PersistInterval uint64 `toml:"persist_interval"`

	// ValidatorUpdates is a map of heights to validator names and their power,
	// and will be returned by the ABCI application. For example, the following
	// changes the power of validator01 and validator02 at height 1000:
	//
	// [validator_update.1000]
	// validator01 = 20
	// validator02 = 10
	//
	// Specifying height 0 returns the validator update during InitChain. The
	// application returns the validator updates as-is, i.e. removing a
	// validator must be done by returning it with power 0, and any validators
	// not specified are not changed.
	//
	// height <-> pubkey <-> voting power
	ValidatorUpdates map[string]map[string]uint8 `toml:"validator_update"`

	// Add artificial delays to each of the main ABCI calls to mimic computation time
	// of the application
	PrepareProposalDelay time.Duration `toml:"prepare_proposal_delay"`
	ProcessProposalDelay time.Duration `toml:"process_proposal_delay"`
	CheckTxDelay         time.Duration `toml:"check_tx_delay"`
	FinalizeBlockDelay   time.Duration `toml:"finalize_block_delay"`
	// TODO: add vote extension delays once completed (@cmwaters)
}

func DefaultConfig(dir string) *Config {
	return &Config{
		PersistInterval:  1,
		SnapshotInterval: 100,
		Dir:              dir,
	}
}

// NewApplication creates the application.
func NewApplication(cfg *Config) (abci.Application, error) {
	state, err := NewState(cfg.Dir, cfg.PersistInterval)
	if err != nil {
		return nil, err
	}
	snapshots, err := NewSnapshotStore(filepath.Join(cfg.Dir, "snapshots"))
	if err != nil {
		return nil, err
	}
	return &Application{
		logger:    log.NewTMLogger(log.NewSyncWriter(os.Stdout)),
		state:     state,
		snapshots: snapshots,
		cfg:       cfg,
	}, nil
}

// Info implements ABCI.
func (app *Application) Info(_ context.Context, req *abci.RequestInfo) (*abci.ResponseInfo, error) {
	return &abci.ResponseInfo{
		Version:          version.ABCIVersion,
		AppVersion:       appVersion,
		LastBlockHeight:  int64(app.state.Height),
		LastBlockAppHash: app.state.Hash,
	}, nil
}

// Info implements ABCI.
func (app *Application) InitChain(_ context.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	var err error
	app.state.initialHeight = uint64(req.InitialHeight)
	if len(req.AppStateBytes) > 0 {
		err = app.state.Import(0, req.AppStateBytes)
		if err != nil {
			panic(err)
		}
	}
	resp := &abci.ResponseInitChain{
		AppHash: app.state.Hash,
	}
	if resp.Validators, err = app.validatorUpdates(0); err != nil {
		panic(err)
	}
	return resp, nil
}

// CheckTx implements ABCI.
func (app *Application) CheckTx(_ context.Context, req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
	_, _, err := parseTx(req.Tx)
	if err != nil {
		return &abci.ResponseCheckTx{
			Code: kvstore.CodeTypeEncodingError,
			Log:  err.Error(),
		}, nil
	}

	if app.cfg.CheckTxDelay != 0 {
		time.Sleep(app.cfg.CheckTxDelay)
	}

	return &abci.ResponseCheckTx{Code: kvstore.CodeTypeOK, GasWanted: 1}, nil
}

// FinalizeBlock implements ABCI.
func (app *Application) FinalizeBlock(_ context.Context, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	var txs = make([]*abci.ExecTxResult, len(req.Txs))

	for i, tx := range req.Txs {
		key, value, err := parseTx(tx)
		if err != nil {
			panic(err) // shouldn't happen since we verified it in CheckTx
		}
		app.state.Set(key, value)

		txs[i] = &abci.ExecTxResult{Code: kvstore.CodeTypeOK}
	}

	valUpdates, err := app.validatorUpdates(uint64(req.Height))
	if err != nil {
		panic(err)
	}

	if app.cfg.FinalizeBlockDelay != 0 {
		time.Sleep(app.cfg.FinalizeBlockDelay)
	}

	return &abci.ResponseFinalizeBlock{
		TxResults:        txs,
		ValidatorUpdates: valUpdates,
		AgreedAppData:    app.state.Finalize(),
		Events: []abci.Event{
			{
				Type: "val_updates",
				Attributes: []abci.EventAttribute{
					{
						Key:   "size",
						Value: strconv.Itoa(valUpdates.Len()),
					},
					{
						Key:   "height",
						Value: strconv.Itoa(int(req.Height)),
					},
				},
			},
		},
	}, nil
}

// Commit implements ABCI.
func (app *Application) Commit(_ context.Context, _ *abci.RequestCommit) (*abci.ResponseCommit, error) {
	height, err := app.state.Commit()
	if err != nil {
		panic(err)
	}
	if app.cfg.SnapshotInterval > 0 && height%app.cfg.SnapshotInterval == 0 {
		snapshot, err := app.snapshots.Create(app.state)
		if err != nil {
			panic(err)
		}
		app.logger.Info("created state sync snapshot", "height", snapshot.Height)
		err = app.snapshots.Prune(maxSnapshotCount)
		if err != nil {
			app.logger.Error("failed to prune snapshots", "err", err)
		}
	}
	retainHeight := int64(0)
	if app.cfg.RetainBlocks > 0 {
		retainHeight = int64(height - app.cfg.RetainBlocks + 1)
	}
	return &abci.ResponseCommit{
		RetainHeight: retainHeight,
	}, nil
}

// Query implements ABCI.
func (app *Application) Query(_ context.Context, req *abci.RequestQuery) (*abci.ResponseQuery, error) {
	return &abci.ResponseQuery{
		Height: int64(app.state.Height),
		Key:    req.Data,
		Value:  []byte(app.state.Get(string(req.Data))),
	}, nil
}

// ListSnapshots implements ABCI.
func (app *Application) ListSnapshots(_ context.Context, req *abci.RequestListSnapshots) (*abci.ResponseListSnapshots, error) {
	snapshots, err := app.snapshots.List()
	if err != nil {
		panic(err)
	}
	return &abci.ResponseListSnapshots{Snapshots: snapshots}, nil
}

// LoadSnapshotChunk implements ABCI.
func (app *Application) LoadSnapshotChunk(_ context.Context, req *abci.RequestLoadSnapshotChunk) (*abci.ResponseLoadSnapshotChunk, error) {
	chunk, err := app.snapshots.LoadChunk(req.Height, req.Format, req.Chunk)
	if err != nil {
		panic(err)
	}
	return &abci.ResponseLoadSnapshotChunk{Chunk: chunk}, nil
}

// OfferSnapshot implements ABCI.
func (app *Application) OfferSnapshot(_ context.Context, req *abci.RequestOfferSnapshot) (*abci.ResponseOfferSnapshot, error) {
	if app.restoreSnapshot != nil {
		panic("A snapshot is already being restored")
	}
	app.restoreSnapshot = req.Snapshot
	app.restoreChunks = [][]byte{}
	return &abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_ACCEPT}, nil
}

// ApplySnapshotChunk implements ABCI.
func (app *Application) ApplySnapshotChunk(_ context.Context, req *abci.RequestApplySnapshotChunk) (*abci.ResponseApplySnapshotChunk, error) {
	if app.restoreSnapshot == nil {
		panic("No restore in progress")
	}
	app.restoreChunks = append(app.restoreChunks, req.Chunk)
	if len(app.restoreChunks) == int(app.restoreSnapshot.Chunks) {
		bz := []byte{}
		for _, chunk := range app.restoreChunks {
			bz = append(bz, chunk...)
		}
		err := app.state.Import(app.restoreSnapshot.Height, bz)
		if err != nil {
			panic(err)
		}
		app.restoreSnapshot = nil
		app.restoreChunks = nil
	}
	return &abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}, nil
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
func (app *Application) PrepareProposal(
	_ context.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
	var sum int64
	var extCount int
	for _, vote := range req.LocalLastCommit.Votes {
		if !vote.SignedLastBlock || len(vote.VoteExtension) == 0 {
			continue
		}
		extValue, err := parseVoteExtension(vote.VoteExtension)
		// This should have been verified in VerifyVoteExtension
		if err != nil {
			panic(fmt.Errorf("failed to parse vote extension in PrepareProposal: %w", err))
		}
		valAddr := crypto.Address(vote.Validator.Address)
		app.logger.Info("got vote extension value in PrepareProposal", "valAddr", valAddr, "value", extValue)
		sum += extValue
		extCount++
	}

	if app.cfg.PrepareProposalDelay != 0 {
		time.Sleep(app.cfg.PrepareProposalDelay)
	}

	// We only generate our special transaction if we have vote extensions
	if extCount > 0 {
		var totalBytes int64
		extTxPrefix := fmt.Sprintf("%s=", voteExtensionKey)
		extTx := []byte(fmt.Sprintf("%s%d", extTxPrefix, sum))
		app.logger.Info("preparing proposal with custom transaction from vote extensions", "tx", extTx)
		// Our generated transaction takes precedence over any supplied
		// transaction that attempts to modify the "extensionSum" value.
		txs := make([][]byte, len(req.Txs)+1)
		for i, tx := range req.Txs {
			if strings.HasPrefix(string(tx), extTxPrefix) {
				continue
			}
			txs[i] = tx
			totalBytes += int64(len(tx))
		}
		if totalBytes+int64(len(extTx)) < req.MaxTxBytes {
			txs[len(req.Txs)] = extTx
		} else {
			app.logger.Info(
				"too many txs to include special vote extension-generated tx",
				"totalBytes", totalBytes,
				"MaxTxBytes", req.MaxTxBytes,
				"extTx", extTx,
				"extTxLen", len(extTx),
			)
		}
		return &abci.ResponsePrepareProposal{Txs: txs}, nil
	}
	// None of the transactions are modified by this application.
	txs := make([][]byte, 0, len(req.Txs))
	var totalBytes int64
	for _, tx := range req.Txs {
		totalBytes += int64(len(tx))
		if totalBytes > req.MaxTxBytes {
			break
		}
		txs = append(txs, tx)
	}

	return &abci.ResponsePrepareProposal{Txs: txs}, nil
}

// ProcessProposal implements part of the Application interface.
// It accepts any proposal that does not contain a malformed transaction.
func (app *Application) ProcessProposal(_ context.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
	for _, tx := range req.Txs {
		k, v, err := parseTx(tx)
		if err != nil {
			app.logger.Error("malformed transaction in ProcessProposal", "tx", tx, "err", err)
			return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, nil
		}
		// Additional check for vote extension-related txs
		if k == voteExtensionKey {
			_, err := strconv.Atoi(v)
			if err != nil {
				app.logger.Error("malformed vote extension transaction", k, v, "err", err)
				return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, nil
			}
		}
	}

	if app.cfg.ProcessProposalDelay != 0 {
		time.Sleep(app.cfg.ProcessProposalDelay)
	}

	return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil
}

// ExtendVote will produce vote extensions in the form of random numbers to
// demonstrate vote extension nondeterminism.
//
// In the next block, if there are any vote extensions from the previous block,
// a new transaction will be proposed that updates a special value in the
// key/value store ("extensionSum") with the sum of all of the numbers collected
// from the vote extensions.
func (app *Application) ExtendVote(_ context.Context, req *abci.RequestExtendVote) (*abci.ResponseExtendVote, error) {
	// We ignore any requests for vote extensions that don't match our expected
	// next height.
	if req.Height != int64(app.state.Height)+1 {
		app.logger.Error(
			"got unexpected height in ExtendVote request",
			"expectedHeight", app.state.Height+1,
			"requestHeight", req.Height,
		)
		return &abci.ResponseExtendVote{}, nil
	}
	ext := make([]byte, binary.MaxVarintLen64)
	// We don't care that these values are generated by a weak random number
	// generator. It's just for test purposes.
	// nolint:gosec // G404: Use of weak random number generator
	num := rand.Int63n(voteExtensionMaxVal)
	extLen := binary.PutVarint(ext, num)
	app.logger.Info("generated vote extension", "num", num, "ext", fmt.Sprintf("%x", ext[:extLen]), "state.Height", app.state.Height)
	return &abci.ResponseExtendVote{
		VoteExtension: ext[:extLen],
	}, nil
}

// VerifyVoteExtension simply validates vote extensions from other validators
// without doing anything about them. In this case, it just makes sure that the
// vote extension is a well-formed integer value.
func (app *Application) VerifyVoteExtension(_ context.Context, req *abci.RequestVerifyVoteExtension) (*abci.ResponseVerifyVoteExtension, error) {
	// We allow vote extensions to be optional
	if len(req.VoteExtension) == 0 {
		return &abci.ResponseVerifyVoteExtension{
			Status: abci.ResponseVerifyVoteExtension_ACCEPT,
		}, nil
	}
	if req.Height != int64(app.state.Height)+1 {
		app.logger.Error(
			"got unexpected height in VerifyVoteExtension request",
			"expectedHeight", app.state.Height,
			"requestHeight", req.Height,
		)
		return &abci.ResponseVerifyVoteExtension{
			Status: abci.ResponseVerifyVoteExtension_REJECT,
		}, nil
	}

	num, err := parseVoteExtension(req.VoteExtension)
	if err != nil {
		app.logger.Error("failed to verify vote extension", "req", req, "err", err)
		return &abci.ResponseVerifyVoteExtension{
			Status: abci.ResponseVerifyVoteExtension_REJECT,
		}, nil
	}
	app.logger.Info("verified vote extension value", "req", req, "num", num)
	return &abci.ResponseVerifyVoteExtension{
		Status: abci.ResponseVerifyVoteExtension_ACCEPT,
	}, nil
}

func (app *Application) Rollback() error {
	return app.state.Rollback()
}

// validatorUpdates generates a validator set update.
func (app *Application) validatorUpdates(height uint64) (abci.ValidatorUpdates, error) {
	updates := app.cfg.ValidatorUpdates[fmt.Sprintf("%v", height)]
	if len(updates) == 0 {
		return nil, nil
	}

	valUpdates := abci.ValidatorUpdates{}
	for keyString, power := range updates {

		keyBytes, err := base64.StdEncoding.DecodeString(keyString)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 pubkey value %q: %w", keyString, err)
		}
		valUpdates = append(valUpdates, abci.UpdateValidator(keyBytes, int64(power), app.cfg.KeyType))
	}
	return valUpdates, nil
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
