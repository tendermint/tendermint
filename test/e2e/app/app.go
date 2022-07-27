package app

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/tendermint/tendermint/abci/example/code"
	abci "github.com/tendermint/tendermint/abci/types"
<<<<<<< HEAD
=======
	"github.com/tendermint/tendermint/crypto"
	tmstrings "github.com/tendermint/tendermint/internal/libs/strings"
>>>>>>> 48147e1fb (logging: implement lazy sprinting (#8898))
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/version"
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
}

func DefaultConfig(dir string) *Config {
	return &Config{
		PersistInterval:  1,
		SnapshotInterval: 100,
		Dir:              dir,
	}
}

// NewApplication creates the application.
func NewApplication(cfg *Config) (*Application, error) {
	state, err := NewState(cfg.Dir, cfg.PersistInterval)
	if err != nil {
		return nil, err
	}
	snapshots, err := NewSnapshotStore(filepath.Join(cfg.Dir, "snapshots"))
	if err != nil {
		return nil, err
	}
	return &Application{
		logger:    log.MustNewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo, false),
		state:     state,
		snapshots: snapshots,
		cfg:       cfg,
	}, nil
}

// Info implements ABCI.
func (app *Application) Info(req abci.RequestInfo) abci.ResponseInfo {
	app.state.RLock()
	defer app.state.RUnlock()

	return abci.ResponseInfo{
		Version:          version.ABCIVersion,
		AppVersion:       1,
		LastBlockHeight:  int64(app.state.Height),
		LastBlockAppHash: app.state.Hash,
	}
}

// Info implements ABCI.
func (app *Application) InitChain(req abci.RequestInitChain) abci.ResponseInitChain {
	var err error
	app.state.initialHeight = uint64(req.InitialHeight)
	if len(req.AppStateBytes) > 0 {
		err = app.state.Import(0, req.AppStateBytes)
		if err != nil {
			panic(err)
		}
	}
	resp := abci.ResponseInitChain{
		AppHash: app.state.Hash,
		ConsensusParams: &types.ConsensusParams{
			Version: &types.VersionParams{
				AppVersion: 1,
			},
		},
	}
	if resp.Validators, err = app.validatorUpdates(0); err != nil {
		panic(err)
	}
	return resp
}

// CheckTx implements ABCI.
func (app *Application) CheckTx(req abci.RequestCheckTx) abci.ResponseCheckTx {
	_, _, err := parseTx(req.Tx)
	if err != nil {
		return abci.ResponseCheckTx{
			Code: code.CodeTypeEncodingError,
			Log:  err.Error(),
		}
	}
	return abci.ResponseCheckTx{Code: code.CodeTypeOK, GasWanted: 1}
}

// DeliverTx implements ABCI.
func (app *Application) DeliverTx(req abci.RequestDeliverTx) abci.ResponseDeliverTx {
	key, value, err := parseTx(req.Tx)
	if err != nil {
		panic(err) // shouldn't happen since we verified it in CheckTx
	}
	app.state.Set(key, value)
	return abci.ResponseDeliverTx{Code: code.CodeTypeOK}
}

// EndBlock implements ABCI.
func (app *Application) EndBlock(req abci.RequestEndBlock) abci.ResponseEndBlock {
	valUpdates, err := app.validatorUpdates(uint64(req.Height))
	if err != nil {
		panic(err)
	}

	return abci.ResponseEndBlock{
		ValidatorUpdates: valUpdates,
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
	}
}

// Commit implements ABCI.
func (app *Application) Commit() abci.ResponseCommit {
	height, hash, err := app.state.Commit()
	if err != nil {
		panic(err)
	}
	if app.cfg.SnapshotInterval > 0 && height%app.cfg.SnapshotInterval == 0 {
		snapshot, err := app.snapshots.Create(app.state)
		if err != nil {
			panic(err)
		}
		app.logger.Info("Created state sync snapshot", "height", snapshot.Height)
		err = app.snapshots.Prune(maxSnapshotCount)
		if err != nil {
			app.logger.Error("Failed to prune snapshots", "err", err)
		}
	}
	retainHeight := int64(0)
	if app.cfg.RetainBlocks > 0 {
		retainHeight = int64(height - app.cfg.RetainBlocks + 1)
	}
	return abci.ResponseCommit{
		Data:         hash,
		RetainHeight: retainHeight,
	}
}

// Query implements ABCI.
func (app *Application) Query(req abci.RequestQuery) abci.ResponseQuery {
	return abci.ResponseQuery{
		Height: int64(app.state.Height),
		Key:    req.Data,
		Value:  []byte(app.state.Get(string(req.Data))),
	}
}

// ListSnapshots implements ABCI.
func (app *Application) ListSnapshots(req abci.RequestListSnapshots) abci.ResponseListSnapshots {
	snapshots, err := app.snapshots.List()
	if err != nil {
		panic(err)
	}
	return abci.ResponseListSnapshots{Snapshots: snapshots}
}

// LoadSnapshotChunk implements ABCI.
func (app *Application) LoadSnapshotChunk(req abci.RequestLoadSnapshotChunk) abci.ResponseLoadSnapshotChunk {
	chunk, err := app.snapshots.LoadChunk(req.Height, req.Format, req.Chunk)
	if err != nil {
		panic(err)
	}
	return abci.ResponseLoadSnapshotChunk{Chunk: chunk}
}

// OfferSnapshot implements ABCI.
func (app *Application) OfferSnapshot(req abci.RequestOfferSnapshot) abci.ResponseOfferSnapshot {
	if app.restoreSnapshot != nil {
		panic("A snapshot is already being restored")
	}
	app.restoreSnapshot = req.Snapshot
	app.restoreChunks = [][]byte{}
	return abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_ACCEPT}
}

// ApplySnapshotChunk implements ABCI.
func (app *Application) ApplySnapshotChunk(req abci.RequestApplySnapshotChunk) abci.ResponseApplySnapshotChunk {
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
<<<<<<< HEAD
	return abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}
=======
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
func (app *Application) PrepareProposal(_ context.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

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
	// We only generate our special transaction if we have vote extensions
	if extCount > 0 {
		var totalBytes int64
		extTxPrefix := fmt.Sprintf("%s=", voteExtensionKey)
		extTx := []byte(fmt.Sprintf("%s%d", extTxPrefix, sum))
		app.logger.Info("preparing proposal with custom transaction from vote extensions", "tx", extTx)
		// Our generated transaction takes precedence over any supplied
		// transaction that attempts to modify the "extensionSum" value.
		txRecords := make([]*abci.TxRecord, len(req.Txs)+1)
		for i, tx := range req.Txs {
			if strings.HasPrefix(string(tx), extTxPrefix) {
				txRecords[i] = &abci.TxRecord{
					Action: abci.TxRecord_REMOVED,
					Tx:     tx,
				}
			} else {
				txRecords[i] = &abci.TxRecord{
					Action: abci.TxRecord_UNMODIFIED,
					Tx:     tx,
				}
				totalBytes += int64(len(tx))
			}
		}
		if totalBytes+int64(len(extTx)) < req.MaxTxBytes {
			txRecords[len(req.Txs)] = &abci.TxRecord{
				Action: abci.TxRecord_ADDED,
				Tx:     extTx,
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
		return &abci.ResponsePrepareProposal{
			TxRecords: txRecords,
		}, nil
	}
	// None of the transactions are modified by this application.
	trs := make([]*abci.TxRecord, 0, len(req.Txs))
	var totalBytes int64
	for _, tx := range req.Txs {
		totalBytes += int64(len(tx))
		if totalBytes > req.MaxTxBytes {
			break
		}
		trs = append(trs, &abci.TxRecord{
			Action: abci.TxRecord_UNMODIFIED,
			Tx:     tx,
		})
	}

	if app.cfg.PrepareProposalDelayMS != 0 {
		time.Sleep(time.Duration(app.cfg.PrepareProposalDelayMS) * time.Millisecond)
	}

	return &abci.ResponsePrepareProposal{TxRecords: trs}, nil
}

// ProcessProposal implements part of the Application interface.
// It accepts any proposal that does not contain a malformed transaction.
func (app *Application) ProcessProposal(_ context.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

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

	if app.cfg.ProcessProposalDelayMS != 0 {
		time.Sleep(time.Duration(app.cfg.ProcessProposalDelayMS) * time.Millisecond)
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
	app.mu.Lock()
	defer app.mu.Unlock()

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

	if app.cfg.VoteExtensionDelayMS != 0 {
		time.Sleep(time.Duration(app.cfg.VoteExtensionDelayMS) * time.Millisecond)
	}

	app.logger.Info("generated vote extension",
		"num", num,
		"ext", tmstrings.LazySprintf("%x", ext[:extLen]),
		"state.Height", app.state.Height)
	return &abci.ResponseExtendVote{
		VoteExtension: ext[:extLen],
	}, nil
}

// VerifyVoteExtension simply validates vote extensions from other validators
// without doing anything about them. In this case, it just makes sure that the
// vote extension is a well-formed integer value.
func (app *Application) VerifyVoteExtension(_ context.Context, req *abci.RequestVerifyVoteExtension) (*abci.ResponseVerifyVoteExtension, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

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

	if app.cfg.VoteExtensionDelayMS != 0 {
		time.Sleep(time.Duration(app.cfg.VoteExtensionDelayMS) * time.Millisecond)
	}

	app.logger.Info("verified vote extension value", "req", req, "num", num)
	return &abci.ResponseVerifyVoteExtension{
		Status: abci.ResponseVerifyVoteExtension_ACCEPT,
	}, nil
>>>>>>> 48147e1fb (logging: implement lazy sprinting (#8898))
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

	// the validator updates could be returned in arbitrary order,
	// and that seems potentially bad. This orders the validator
	// set.
	sort.Slice(valUpdates, func(i, j int) bool {
		return valUpdates[i].PubKey.Compare(valUpdates[j].PubKey) < 0
	})

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
