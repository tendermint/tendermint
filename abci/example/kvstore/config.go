package kvstore

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"

	"github.com/gogo/protobuf/proto"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/crypto/encoding"
)

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

func DefaultConfig(dir string) Config {
	return Config{
		PersistInterval:  1,
		SnapshotInterval: 100,
		Dir:              dir,
	}
}

// validatorSetUpdates generates a validator set update.
func (cfg Config) validatorSetUpdates() (map[int64]abci.ValidatorSetUpdate, error) {
	ret := map[int64]abci.ValidatorSetUpdate{}
	for heightS, updates := range cfg.ValidatorUpdates {
		if len(updates) == 0 {
			continue
		}

		height, err := strconv.ParseInt(heightS, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse height %s: %w", heightS, err)
		}

		thresholdPublicKeyUpdateString := cfg.ThesholdPublicKeyUpdate[fmt.Sprintf("%v", height)]
		if len(thresholdPublicKeyUpdateString) == 0 {
			return nil, fmt.Errorf("thresholdPublicKeyUpdate must be set")
		}
		thresholdPublicKeyUpdateBytes, err := base64.StdEncoding.DecodeString(thresholdPublicKeyUpdateString)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 pubkey value %q: %w", thresholdPublicKeyUpdateString, err)
		}
		thresholdPublicKeyUpdate := bls12381.PubKey(thresholdPublicKeyUpdateBytes)
		abciThresholdPublicKeyUpdate := encoding.MustPubKeyToProto(thresholdPublicKeyUpdate)

		quorumHashUpdateString := cfg.QuorumHashUpdate[fmt.Sprintf("%v", height)]
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

		ret[height] = valSetUpdates
	}
	return ret, nil
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
