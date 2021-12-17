package app

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/tendermint/tendermint/crypto"

	"github.com/tendermint/tendermint/types"

	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/crypto/encoding"

	"github.com/tendermint/tendermint/abci/example/code"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	types1 "github.com/tendermint/tendermint/proto/tendermint/types"
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
	ValidatorUpdates map[string]map[string]string `toml:"validator_update"`

	// dash parameters
	ThesholdPublicKeyUpdate map[string]string `toml:"threshold_public_key_update"`
	QuorumHashUpdate        map[string]string `toml:"quorum_hash_update"`
	ChainLockUpdates        map[string]string `toml:"chainlock_updates"`
	PrivValServerType       string            `toml:"privval_server_type"`
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
	state, err := NewState(filepath.Join(cfg.Dir, "state.json"), cfg.PersistInterval)
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
	return abci.ResponseInfo{
		Version:                   version.ABCIVersion,
		AppVersion:                1,
		LastBlockHeight:           int64(app.state.Height),
		LastBlockAppHash:          app.state.Hash,
		LastCoreChainLockedHeight: app.state.CoreHeight,
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
		ConsensusParams: &types1.ConsensusParams{
			Version: &types1.VersionParams{
				AppVersion: 1,
			},
		},
	}

	validatorSetUpdate, err := app.validatorSetUpdates(0)
	if err != nil {
		panic(err)
	}
	resp.ValidatorSetUpdate = *validatorSetUpdate

	if resp.NextCoreChainLockUpdate, err = app.chainLockUpdate(0); err != nil {
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
	var err error
	resp := abci.ResponseEndBlock{}
	resp.ValidatorSetUpdate, err = app.validatorSetUpdates(uint64(req.Height))
	if err != nil {
		panic(err)
	}
	resp.NextCoreChainLockUpdate, err = app.chainLockUpdate(uint64(req.Height))
	if err != nil {
		panic(err)
	}
	resp.Events = []abci.Event{
		{
			Type: "val_updates",
			Attributes: []abci.EventAttribute{
				{
					Key:   "size",
					Value: strconv.Itoa(len(resp.ValidatorSetUpdate.ValidatorUpdates)),
				},
				{
					Key:   "height",
					Value: strconv.Itoa(int(req.Height)),
				},
			},
		},
	}
	return resp
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
	return abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}
}

// validatorUpdates generates a validator set update.
func (app *Application) validatorSetUpdates(height uint64) (*abci.ValidatorSetUpdate, error) {
	updates := app.cfg.ValidatorUpdates[fmt.Sprintf("%v", height)]
	if len(updates) == 0 {
		return &abci.ValidatorSetUpdate{}, nil
	}

	thresholdPublicKeyUpdateString := app.cfg.ThesholdPublicKeyUpdate[fmt.Sprintf("%v", height)]
	if len(thresholdPublicKeyUpdateString) == 0 {
		return nil, fmt.Errorf("thresholdPublicKeyUpdate must be set")
	}
	thresholdPublicKeyUpdateBytes, err := base64.StdEncoding.DecodeString(thresholdPublicKeyUpdateString)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 pubkey value %q: %w", thresholdPublicKeyUpdateString, err)
	}
	thresholdPublicKeyUpdate := bls12381.PubKey(thresholdPublicKeyUpdateBytes)
	abciThresholdPublicKeyUpdate, err := encoding.PubKeyToProto(thresholdPublicKeyUpdate)
	if err != nil {
		panic(err)
	}

	quorumHashUpdateString := app.cfg.QuorumHashUpdate[fmt.Sprintf("%v", height)]
	if len(quorumHashUpdateString) == 0 {
		return nil, fmt.Errorf("quorumHashUpdate must be set")
	}
	quorumHashUpdateBytes, err := hex.DecodeString(quorumHashUpdateString)
	if err != nil {
		return nil, fmt.Errorf("invalid hex quorum value %q: %w", quorumHashUpdateString, err)
	}
	quorumHashUpdate := crypto.QuorumHash(quorumHashUpdateBytes)

	valSetUpdates := abci.ValidatorSetUpdate{}

	valUpdates := abci.ValidatorUpdates{}
	for proTxHashString, keyString := range updates {
		keyBytes, err := base64.StdEncoding.DecodeString(keyString)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 pubkey value %q: %w", keyString, err)
		}
		proTxHashBytes, err := hex.DecodeString(proTxHashString)
		if err != nil {
			return nil, fmt.Errorf("invalid hex proTxHash value %q: %w", proTxHashBytes, err)
		}
		publicKeyUpdate := bls12381.PubKey(keyBytes)
		valUpdates = append(valUpdates, abci.UpdateValidator(proTxHashBytes, publicKeyUpdate, types.DefaultDashVotingPower))
	}
	valSetUpdates.ValidatorUpdates = valUpdates
	valSetUpdates.ThresholdPublicKey = abciThresholdPublicKeyUpdate
	valSetUpdates.QuorumHash = quorumHashUpdate
	return &valSetUpdates, nil
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
