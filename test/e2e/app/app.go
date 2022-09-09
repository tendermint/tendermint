package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"

	"github.com/tendermint/tendermint/crypto"

	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/crypto/encoding"

	"github.com/tendermint/tendermint/abci/example/code"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	types1 "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

const (
	voteExtensionKey    string = "extensionSum"
	voteExtensionMaxVal int64  = 128
)

// Application is an ABCI application for use by end-to-end tests. It is a
// simple key/value store for strings, storing data in memory and persisting
// to disk as JSON, taking state sync snapshots if requested.
type Application struct {
	abci.BaseApplication
	mu              sync.Mutex
	logger          log.Logger
	state           *State
	snapshots       *SnapshotStore
	cfg             *Config
	restoreSnapshot *abci.Snapshot
	restoreChunks   [][]byte

	valUpdates *abci.ValidatorSetUpdate
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

	// Add artificial delays to each of the main ABCI calls to mimic computation time
	// of the application
	PrepareProposalDelayMS uint64 `toml:"prepare_proposal_delay_ms"`
	ProcessProposalDelayMS uint64 `toml:"process_proposal_delay_ms"`
	CheckTxDelayMS         uint64 `toml:"check_tx_delay_ms"`
	VoteExtensionDelayMS   uint64 `toml:"vote_extension_delay_ms"`
	FinalizeBlockDelayMS   uint64 `toml:"finalize_block_delay_ms"`

	// dash parameters
	ThesholdPublicKeyUpdate  map[string]string `toml:"threshold_public_key_update"`
	QuorumHashUpdate         map[string]string `toml:"quorum_hash_update"`
	ChainLockUpdates         map[string]string `toml:"chainlock_updates"`
	PrivValServerType        string            `toml:"privval_server_type"`
	InitAppInitialCoreHeight uint32            `toml:"init_app_core_chain_locked_height"`
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
	logger, err := log.NewDefaultLogger(log.LogFormatPlain, log.LogLevelDebug)
	if err != nil {
		return nil, err
	}

	return &Application{
		logger:    logger.With("module", "abci_app"),
		state:     state,
		snapshots: snapshots,
		cfg:       cfg,
	}, nil
}

// Info implements ABCI.
func (app *Application) Info(_ context.Context, req *abci.RequestInfo) (*abci.ResponseInfo, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	return &abci.ResponseInfo{
		Version:          version.ABCIVersion,
		AppVersion:       1,
		LastBlockHeight:  int64(app.state.Height),
		LastBlockAppHash: app.state.Hash,
	}, nil
}

// InitChain implements ABCI.
func (app *Application) InitChain(_ context.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

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
	resp.InitialCoreHeight = app.cfg.InitAppInitialCoreHeight
	if resp.NextCoreChainLockUpdate, err = app.chainLockUpdate(0); err != nil {
		panic(err)
	}
	app.logger.Debug("InitChain", "req", req, "resp", resp)

	return resp, nil
}

// CheckTx implements ABCI.
func (app *Application) CheckTx(_ context.Context, req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	_, _, err := parseTx(req.Tx)
	if err != nil {
		return &abci.ResponseCheckTx{
			Code: code.CodeTypeEncodingError,
		}, nil
	}

	if app.cfg.CheckTxDelayMS != 0 {
		time.Sleep(time.Duration(app.cfg.CheckTxDelayMS) * time.Millisecond)
	}

	return &abci.ResponseCheckTx{Code: code.CodeTypeOK, GasWanted: 1}, nil
}

func (app *Application) txResults(nTransactions int) (txs []*abci.ExecTxResult) {
	txs = make([]*abci.ExecTxResult, nTransactions)
	for i := 0; i < nTransactions; i++ {
		txs[i] = &abci.ExecTxResult{Code: code.CodeTypeOK}
	}

	return txs
}

// FinalizeBlock implements ABCI.
func (app *Application) FinalizeBlock(_ context.Context, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	var txs = make([]*abci.ExecTxResult, len(req.Txs))

	app.mu.Lock()
	defer app.mu.Unlock()

	for i, tx := range req.Txs {
		key, value, err := parseTx(tx)
		if err != nil {
			panic(err) // shouldn't happen since we verified it in CheckTx
		}
		app.state.Set(key, value)

		txs[i] = &abci.ExecTxResult{Code: code.CodeTypeOK}
	}

	resp := abci.ResponseFinalizeBlock{}

	if app.cfg.FinalizeBlockDelayMS != 0 {
		time.Sleep(time.Duration(app.cfg.FinalizeBlockDelayMS) * time.Millisecond)
	}

	resp.Events = []abci.Event{
		{
			Type: "val_updates",
			Attributes: []abci.EventAttribute{
				{
					Key:   "size",
					Value: strconv.Itoa(len(app.valUpdates.ValidatorUpdates)),
				},
				{
					Key:   "height",
					Value: strconv.Itoa(int(req.Height)),
				},
			},
		},
	}

	return &resp, nil
}

// Commit implements ABCI.
func (app *Application) Commit(_ context.Context) (*abci.ResponseCommit, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	height, hash, err := app.state.Commit()
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
		Data:         hash,
		RetainHeight: retainHeight,
	}, nil
}

// Query implements ABCI.
func (app *Application) Query(_ context.Context, req *abci.RequestQuery) (*abci.ResponseQuery, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	return &abci.ResponseQuery{
		Height: int64(app.state.Height),
		Key:    req.Data,
		Value:  []byte(app.state.Get(string(req.Data))),
	}, nil
}

// ListSnapshots implements ABCI.
func (app *Application) ListSnapshots(_ context.Context, req *abci.RequestListSnapshots) (*abci.ResponseListSnapshots, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	snapshots, err := app.snapshots.List()
	if err != nil {
		panic(err)
	}
	return &abci.ResponseListSnapshots{Snapshots: snapshots}, nil
}

// LoadSnapshotChunk implements ABCI.
func (app *Application) LoadSnapshotChunk(_ context.Context, req *abci.RequestLoadSnapshotChunk) (*abci.ResponseLoadSnapshotChunk, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	chunk, err := app.snapshots.LoadChunk(req.Height, req.Format, req.Chunk)
	if err != nil {
		panic(err)
	}
	return &abci.ResponseLoadSnapshotChunk{Chunk: chunk}, nil
}

// OfferSnapshot implements ABCI.
func (app *Application) OfferSnapshot(_ context.Context, req *abci.RequestOfferSnapshot) (*abci.ResponseOfferSnapshot, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	if app.restoreSnapshot != nil {
		panic("A snapshot is already being restored")
	}
	app.restoreSnapshot = req.Snapshot
	app.restoreChunks = [][]byte{}
	return &abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_ACCEPT}, nil
}

// ApplySnapshotChunk implements ABCI.
func (app *Application) ApplySnapshotChunk(_ context.Context, req *abci.RequestApplySnapshotChunk) (*abci.ResponseApplySnapshotChunk, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

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
func (app *Application) PrepareProposal(_ context.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
	app.mu.Lock()
	defer app.mu.Unlock()

	validatorSetUpdate, err := app.validatorSetUpdates(uint64(req.Height))
	if err != nil {
		panic(err)
	}
	app.valUpdates = validatorSetUpdate

	nextCoreChainLockUpdate, err := app.chainLockUpdate(uint64(req.Height))
	if err != nil {
		panic(err)
	}

	extCount := len(req.LocalLastCommit.ThresholdVoteExtensions)
	txRecords, err := app.processTxs(req.Txs, req.MaxTxBytes, extCount)
	if err != nil {
		return &abci.ResponsePrepareProposal{}, err
	}

	if app.cfg.PrepareProposalDelayMS != 0 {
		time.Sleep(time.Duration(app.cfg.PrepareProposalDelayMS) * time.Millisecond)
	}

	return &abci.ResponsePrepareProposal{
		TxRecords:           txRecords,
		TxResults:           app.txResults(len(txRecords)),
		ValidatorSetUpdate:  validatorSetUpdate,
		CoreChainLockUpdate: nextCoreChainLockUpdate,
		AppHash:             app.state.Hash,
	}, nil
}

func (app *Application) processTxs(txs [][]byte, maxTxBytes int64, extCount int) ([]*abci.TxRecord, error) {
	var (
		totalBytes int64
		txRecords  []*abci.TxRecord
	)

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
		if totalBytes+int64(len(extTx)) < maxTxBytes {
			txRecords = append(txRecords, &abci.TxRecord{
				Action: abci.TxRecord_ADDED,
				Tx:     extTx,
			})
		}
	} else {
		app.logger.Info(
			"too many txs to include special vote extension-generated tx",
			"totalBytes", totalBytes,
			"MaxTxBytes", maxTxBytes,
			"extTx", extTx,
			"extTxLen", len(extTx),
		)
	}

	return txRecords, nil
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

	validatorSetUpdate, err := app.validatorSetUpdates(uint64(req.Height))
	if err != nil {
		panic(err)
	}
	nextCoreChainLockUpdate, err := app.chainLockUpdate(uint64(req.Height))
	if err != nil {
		panic(err)
	}

	app.valUpdates = validatorSetUpdate

	if app.cfg.ProcessProposalDelayMS != 0 {
		time.Sleep(time.Duration(app.cfg.ProcessProposalDelayMS) * time.Millisecond)
	}

	return &abci.ResponseProcessProposal{
		Status:              abci.ResponseProcessProposal_ACCEPT,
		TxResults:           app.txResults(len(req.Txs)),
		ValidatorSetUpdate:  validatorSetUpdate,
		CoreChainLockUpdate: nextCoreChainLockUpdate,
		AppHash:             app.state.Hash,
	}, nil
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
	currentHeight := app.state.Height
	if currentHeight == 0 {
		currentHeight = app.state.initialHeight
	}
	if req.Height != int64(currentHeight)+1 {
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
	app.logger.Info("generated vote extension",
		"num", num,
		"ext", fmt.Sprintf("%x", ext[:extLen]),
		"state.Height", app.state.Height,
	)
	return &abci.ResponseExtendVote{
		VoteExtensions: []*abci.ExtendVoteExtension{
			{
				Type:      types1.VoteExtensionType_DEFAULT,
				Extension: ext[:extLen],
			},
			{
				Type:      types1.VoteExtensionType_THRESHOLD_RECOVER,
				Extension: []byte(fmt.Sprintf("threshold-%d", app.state.Height)),
			},
		},
	}, nil
}

// VerifyVoteExtension simply validates vote extensions from other validators
// without doing anything about them. In this case, it just makes sure that the
// vote extension is a well-formed integer value.
func (app *Application) VerifyVoteExtension(_ context.Context, req *abci.RequestVerifyVoteExtension) (*abci.ResponseVerifyVoteExtension, error) {
	// We allow vote extensions to be optional
	if len(req.VoteExtensions) == 0 {
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

	nums := make([]int64, 0, len(req.VoteExtensions))
	for _, ext := range req.VoteExtensions {
		num, err := parseVoteExtension(ext.Extension)
		if err != nil {
			app.logger.Error("failed to verify vote extension", "req", req, "err", err)
			return &abci.ResponseVerifyVoteExtension{
				Status: abci.ResponseVerifyVoteExtension_REJECT,
			}, nil
		}
		nums = append(nums, num)
	}

	if app.cfg.VoteExtensionDelayMS != 0 {
		time.Sleep(time.Duration(app.cfg.VoteExtensionDelayMS) * time.Millisecond)
	}

	app.logger.Info("verified vote extension value", "req", req, "nums", nums)
	return &abci.ResponseVerifyVoteExtension{
		Status: abci.ResponseVerifyVoteExtension_ACCEPT,
	}, nil
}

func (app *Application) Rollback() error {
	app.mu.Lock()
	defer app.mu.Unlock()

	return app.state.Rollback()
}

// validatorSetUpdates generates a validator set update.
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
	abciThresholdPublicKeyUpdate := encoding.MustPubKeyToProto(thresholdPublicKeyUpdate)

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
	for proTxHashString, updateBase64 := range updates {
		validator, err := parseValidatorUpdate(updateBase64)
		if err != nil {
			return nil, err
		}
		proTxHashBytes, err := hex.DecodeString(proTxHashString)
		if err != nil {
			return nil, fmt.Errorf("invalid hex proTxHash value %q: %w", proTxHashBytes, err)
		}
		if !bytes.Equal(proTxHashBytes, validator.ProTxHash) {
			return nil, fmt.Errorf("proTxHash mismatch for key %s: %x != %x",
				proTxHashString, proTxHashBytes, validator.ProTxHash)
		}

		valUpdates = append(valUpdates, validator)
	}

	// the validator updates could be returned in arbitrary order,
	// and that seems potentially bad. This orders the validator
	// set.
	sort.Slice(valUpdates, func(i, j int) bool {
		return valUpdates[i].PubKey.Compare(valUpdates[j].PubKey) < 0
	})

	valSetUpdates.ValidatorUpdates = valUpdates
	valSetUpdates.ThresholdPublicKey = abciThresholdPublicKeyUpdate
	valSetUpdates.QuorumHash = quorumHashUpdate
	return &valSetUpdates, nil
}

func parseValidatorUpdate(validatorUpdateBase64 string) (abci.ValidatorUpdate, error) {
	validator := abci.ValidatorUpdate{}

	validatorBytes, err := base64.StdEncoding.DecodeString(validatorUpdateBase64)
	if err != nil {
		return validator, fmt.Errorf("invalid base64 validator update %q: %w", validatorUpdateBase64, err)
	}

	err = proto.Unmarshal(validatorBytes, &validator)
	if err != nil {
		return validator, fmt.Errorf("cannot parse validator update protobuf %q: %w", validatorBytes, err)
	}

	return validator, nil
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
