package types

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"github.com/tendermint/tendermint/crypto/sr25519"
	tmstrings "github.com/tendermint/tendermint/internal/libs/strings"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

const (
	// MaxBlockSizeBytes is the maximum permitted size of the blocks.
	MaxBlockSizeBytes = 104857600 // 100MB

	// BlockPartSizeBytes is the size of one block part.
	BlockPartSizeBytes uint32 = 65536 // 64kB

	// MaxBlockPartsCount is the maximum number of block parts.
	MaxBlockPartsCount = (MaxBlockSizeBytes / BlockPartSizeBytes) + 1

	ABCIPubKeyTypeEd25519   = ed25519.KeyType
	ABCIPubKeyTypeSecp256k1 = secp256k1.KeyType
	ABCIPubKeyTypeSr25519   = sr25519.KeyType
)

var ABCIPubKeyTypesToNames = map[string]string{
	ABCIPubKeyTypeEd25519:   ed25519.PubKeyName,
	ABCIPubKeyTypeSecp256k1: secp256k1.PubKeyName,
	ABCIPubKeyTypeSr25519:   sr25519.PubKeyName,
}

// ConsensusParams contains consensus critical parameters that determine the
// validity of blocks.
type ConsensusParams struct {
	Block     BlockParams     `json:"block"`
	Evidence  EvidenceParams  `json:"evidence"`
	Validator ValidatorParams `json:"validator"`
	Version   VersionParams   `json:"version"`
	Synchrony SynchronyParams `json:"synchrony"`
	Timeout   TimeoutParams   `json:"timeout"`
	ABCI      ABCIParams      `json:"abci"`
}

// HashedParams is a subset of ConsensusParams.
// It is amino encoded and hashed into
// the Header.ConsensusHash.
type HashedParams struct {
	BlockMaxBytes int64
	BlockMaxGas   int64
}

// BlockParams define limits on the block size and gas plus minimum time
// between blocks.
type BlockParams struct {
	MaxBytes int64 `json:"max_bytes,string"`
	MaxGas   int64 `json:"max_gas,string"`
}

// EvidenceParams determine how we handle evidence of malfeasance.
type EvidenceParams struct {
	MaxAgeNumBlocks int64         `json:"max_age_num_blocks,string"` // only accept new evidence more recent than this
	MaxAgeDuration  time.Duration `json:"max_age_duration,string"`
	MaxBytes        int64         `json:"max_bytes,string"`
}

// ValidatorParams restrict the public key types validators can use.
// NOTE: uses ABCI pubkey naming, not Amino names.
type ValidatorParams struct {
	PubKeyTypes []string `json:"pub_key_types"`
}

type VersionParams struct {
	AppVersion uint64 `json:"app_version,string"`
}

// SynchronyParams influence the validity of block timestamps.
// For more information on the relationship of the synchrony parameters to
// block validity, see the Proposer-Based Timestamps specification:
// https://github.com/tendermint/tendermint/blob/master/spec/consensus/proposer-based-timestamp/README.md
type SynchronyParams struct {
	Precision    time.Duration `json:"precision,string"`
	MessageDelay time.Duration `json:"message_delay,string"`
}

// TimeoutParams configure the timings of the steps of the Tendermint consensus algorithm.
type TimeoutParams struct {
	Propose             time.Duration `json:"propose,string"`
	ProposeDelta        time.Duration `json:"propose_delta,string"`
	Vote                time.Duration `json:"vote,string"`
	VoteDelta           time.Duration `json:"vote_delta,string"`
	Commit              time.Duration `json:"commit,string"`
	BypassCommitTimeout bool          `json:"bypass_commit_timeout"`
}

// ABCIParams configure ABCI functionality specific to the Application Blockchain
// Interface.
type ABCIParams struct {
	VoteExtensionsEnableHeight int64 `json:"vote_extensions_enable_height"`
	RecheckTx                  bool  `json:"recheck_tx"`
}

// VoteExtensionsEnabled returns true if vote extensions are enabled at height h
// and false otherwise.
func (a ABCIParams) VoteExtensionsEnabled(h int64) bool {
	if a.VoteExtensionsEnableHeight == 0 {
		return false
	}
	return a.VoteExtensionsEnableHeight <= h
}

// DefaultConsensusParams returns a default ConsensusParams.
func DefaultConsensusParams() *ConsensusParams {
	return &ConsensusParams{
		Block:     DefaultBlockParams(),
		Evidence:  DefaultEvidenceParams(),
		Validator: DefaultValidatorParams(),
		Version:   DefaultVersionParams(),
		Synchrony: DefaultSynchronyParams(),
		Timeout:   DefaultTimeoutParams(),
		ABCI:      DefaultABCIParams(),
	}
}

// DefaultBlockParams returns a default BlockParams.
func DefaultBlockParams() BlockParams {
	return BlockParams{
		MaxBytes: 22020096, // 21MB
		MaxGas:   -1,
	}
}

// DefaultEvidenceParams returns a default EvidenceParams.
func DefaultEvidenceParams() EvidenceParams {
	return EvidenceParams{
		MaxAgeNumBlocks: 100000, // 27.8 hrs at 1block/s
		MaxAgeDuration:  48 * time.Hour,
		MaxBytes:        1048576, // 1MB
	}
}

// DefaultValidatorParams returns a default ValidatorParams, which allows
// only ed25519 pubkeys.
func DefaultValidatorParams() ValidatorParams {
	return ValidatorParams{
		PubKeyTypes: []string{ABCIPubKeyTypeEd25519},
	}
}

func DefaultVersionParams() VersionParams {
	return VersionParams{
		AppVersion: 0,
	}
}

func DefaultSynchronyParams() SynchronyParams {
	return SynchronyParams{
		// 505ms was selected as the default to enable chains that have validators in
		// mixed leap-second handling environments.
		// For more information, see: https://github.com/tendermint/tendermint/issues/7724
		Precision:    505 * time.Millisecond,
		MessageDelay: 12 * time.Second,
	}
}

// SynchronyParamsOrDefaults returns the SynchronyParams, filling in any zero values
// with the Tendermint defined default values.
func (s SynchronyParams) SynchronyParamsOrDefaults() SynchronyParams {
	// TODO: Remove this method and all uses once development on v0.37 begins.
	// See: https://github.com/tendermint/tendermint/issues/8187

	defaults := DefaultSynchronyParams()
	if s.Precision == 0 {
		s.Precision = defaults.Precision
	}
	if s.MessageDelay == 0 {
		s.MessageDelay = defaults.MessageDelay
	}
	return s
}

func DefaultTimeoutParams() TimeoutParams {
	return TimeoutParams{
		Propose:             3000 * time.Millisecond,
		ProposeDelta:        500 * time.Millisecond,
		Vote:                1000 * time.Millisecond,
		VoteDelta:           500 * time.Millisecond,
		Commit:              1000 * time.Millisecond,
		BypassCommitTimeout: false,
	}
}

func DefaultABCIParams() ABCIParams {
	return ABCIParams{
		// When set to 0, vote extensions are not required.
		VoteExtensionsEnableHeight: 0,
		// When true, run CheckTx on each transaction in the mempool after each height.
		RecheckTx: true,
	}
}

// TimeoutParamsOrDefaults returns the SynchronyParams, filling in any zero values
// with the Tendermint defined default values.
func (t TimeoutParams) TimeoutParamsOrDefaults() TimeoutParams {
	// TODO: Remove this method and all uses once development on v0.37 begins.
	// See: https://github.com/tendermint/tendermint/issues/8187

	defaults := DefaultTimeoutParams()
	if t.Propose == 0 {
		t.Propose = defaults.Propose
	}
	if t.ProposeDelta == 0 {
		t.ProposeDelta = defaults.ProposeDelta
	}
	if t.Vote == 0 {
		t.Vote = defaults.Vote
	}
	if t.VoteDelta == 0 {
		t.VoteDelta = defaults.VoteDelta
	}
	if t.Commit == 0 {
		t.Commit = defaults.Commit
	}
	return t
}

// ProposeTimeout returns the amount of time to wait for a proposal.
func (t TimeoutParams) ProposeTimeout(round int32) time.Duration {
	return time.Duration(
		t.Propose.Nanoseconds()+t.ProposeDelta.Nanoseconds()*int64(round),
	) * time.Nanosecond
}

// VoteTimeout returns the amount of time to wait for remaining votes after receiving any +2/3 votes.
func (t TimeoutParams) VoteTimeout(round int32) time.Duration {
	return time.Duration(
		t.Vote.Nanoseconds()+t.VoteDelta.Nanoseconds()*int64(round),
	) * time.Nanosecond
}

// CommitTime accepts ti, the time at which the consensus engine received +2/3
// precommits for a block and returns the point in time at which the consensus
// engine should begin consensus on the next block.
func (t TimeoutParams) CommitTime(ti time.Time) time.Time {
	return ti.Add(t.Commit)
}

func (val *ValidatorParams) IsValidPubkeyType(pubkeyType string) bool {
	for i := 0; i < len(val.PubKeyTypes); i++ {
		if val.PubKeyTypes[i] == pubkeyType {
			return true
		}
	}
	return false
}

func (params *ConsensusParams) Complete() {
	if params.Synchrony == (SynchronyParams{}) {
		params.Synchrony = DefaultSynchronyParams()
	}
	if params.Timeout == (TimeoutParams{}) {
		params.Timeout = DefaultTimeoutParams()
	}
}

// Validate validates the ConsensusParams to ensure all values are within their
// allowed limits, and returns an error if they are not.
func (params ConsensusParams) ValidateConsensusParams() error {
	if params.Block.MaxBytes <= 0 {
		return fmt.Errorf("block.MaxBytes must be greater than 0. Got %d",
			params.Block.MaxBytes)
	}
	if params.Block.MaxBytes > MaxBlockSizeBytes {
		return fmt.Errorf("block.MaxBytes is too big. %d > %d",
			params.Block.MaxBytes, MaxBlockSizeBytes)
	}

	if params.Block.MaxGas < -1 {
		return fmt.Errorf("block.MaxGas must be greater or equal to -1. Got %d",
			params.Block.MaxGas)
	}

	if params.Evidence.MaxAgeNumBlocks <= 0 {
		return fmt.Errorf("evidence.MaxAgeNumBlocks must be greater than 0. Got %d",
			params.Evidence.MaxAgeNumBlocks)
	}

	if params.Evidence.MaxAgeDuration <= 0 {
		return fmt.Errorf("evidence.MaxAgeDuration must be greater than 0 if provided, Got %v",
			params.Evidence.MaxAgeDuration)
	}

	if params.Evidence.MaxBytes > params.Block.MaxBytes {
		return fmt.Errorf("evidence.MaxBytesEvidence is greater than upper bound, %d > %d",
			params.Evidence.MaxBytes, params.Block.MaxBytes)
	}

	if params.Evidence.MaxBytes < 0 {
		return fmt.Errorf("evidence.MaxBytes must be non negative. Got: %d",
			params.Evidence.MaxBytes)
	}

	if params.Synchrony.MessageDelay <= 0 {
		return fmt.Errorf("synchrony.MessageDelay must be greater than 0. Got: %d",
			params.Synchrony.MessageDelay)
	}

	if params.Synchrony.Precision <= 0 {
		return fmt.Errorf("synchrony.Precision must be greater than 0. Got: %d",
			params.Synchrony.Precision)
	}

	if params.Timeout.Propose <= 0 {
		return fmt.Errorf("timeout.ProposeDelta must be greater than 0. Got: %d", params.Timeout.Propose)
	}

	if params.Timeout.ProposeDelta <= 0 {
		return fmt.Errorf("timeout.ProposeDelta must be greater than 0. Got: %d", params.Timeout.ProposeDelta)
	}

	if params.Timeout.Vote <= 0 {
		return fmt.Errorf("timeout.Vote must be greater than 0. Got: %d", params.Timeout.Vote)
	}

	if params.Timeout.VoteDelta <= 0 {
		return fmt.Errorf("timeout.VoteDelta must be greater than 0. Got: %d", params.Timeout.VoteDelta)
	}

	if params.Timeout.Commit <= 0 {
		return fmt.Errorf("timeout.Commit must be greater than 0. Got: %d", params.Timeout.Commit)
	}
	if params.ABCI.VoteExtensionsEnableHeight < 0 {
		return fmt.Errorf("ABCI.VoteExtensionsEnableHeight cannot be negative. Got: %d", params.ABCI.VoteExtensionsEnableHeight)
	}

	if len(params.Validator.PubKeyTypes) == 0 {
		return errors.New("len(Validator.PubKeyTypes) must be greater than 0")
	}

	// Check if keyType is a known ABCIPubKeyType
	for i := 0; i < len(params.Validator.PubKeyTypes); i++ {
		keyType := params.Validator.PubKeyTypes[i]
		if _, ok := ABCIPubKeyTypesToNames[keyType]; !ok {
			return fmt.Errorf("params.Validator.PubKeyTypes[%d], %s, is an unknown pubkey type",
				i, keyType)
		}
	}

	return nil
}

func (params ConsensusParams) ValidateUpdate(updated *tmproto.ConsensusParams, h int64) error {
	if updated.Abci == nil {
		return nil
	}
	if params.ABCI.VoteExtensionsEnableHeight == updated.Abci.VoteExtensionsEnableHeight {
		return nil
	}
	if params.ABCI.VoteExtensionsEnableHeight != 0 && updated.Abci.VoteExtensionsEnableHeight == 0 {
		return errors.New("vote extensions cannot be disabled once enabled")
	}
	if updated.Abci.VoteExtensionsEnableHeight <= h {
		return fmt.Errorf("VoteExtensionsEnableHeight cannot be updated to a past height, "+
			"initial height: %d, current height %d",
			params.ABCI.VoteExtensionsEnableHeight, h)
	}
	if params.ABCI.VoteExtensionsEnableHeight <= h {
		return fmt.Errorf("VoteExtensionsEnableHeight cannot be updated modified once"+
			"the initial height has occurred, "+
			"initial height: %d, current height %d",
			params.ABCI.VoteExtensionsEnableHeight, h)
	}
	return nil
}

// Hash returns a hash of a subset of the parameters to store in the block header.
// Only the Block.MaxBytes and Block.MaxGas are included in the hash.
// This allows the ConsensusParams to evolve more without breaking the block
// protocol. No need for a Merkle tree here, just a small struct to hash.
// TODO: We should hash the other parameters as well
func (params ConsensusParams) HashConsensusParams() []byte {
	hp := tmproto.HashedParams{
		BlockMaxBytes: params.Block.MaxBytes,
		BlockMaxGas:   params.Block.MaxGas,
	}

	bz, err := hp.Marshal()
	if err != nil {
		panic(err)
	}

	sum := sha256.Sum256(bz)

	return sum[:]
}

func (params *ConsensusParams) Equals(params2 *ConsensusParams) bool {
	return params.Block == params2.Block &&
		params.Evidence == params2.Evidence &&
		params.Version == params2.Version &&
		params.Synchrony == params2.Synchrony &&
		params.Timeout == params2.Timeout &&
		params.ABCI == params2.ABCI &&
		tmstrings.StringSliceEqual(params.Validator.PubKeyTypes, params2.Validator.PubKeyTypes)
}

// Update returns a copy of the params with updates from the non-zero fields of p2.
// NOTE: note: must not modify the original
func (params ConsensusParams) UpdateConsensusParams(params2 *tmproto.ConsensusParams) ConsensusParams {
	res := params // explicit copy

	if params2 == nil {
		return res
	}

	// we must defensively consider any structs may be nil
	if params2.Block != nil {
		res.Block.MaxBytes = params2.Block.MaxBytes
		res.Block.MaxGas = params2.Block.MaxGas
	}
	if params2.Evidence != nil {
		res.Evidence.MaxAgeNumBlocks = params2.Evidence.MaxAgeNumBlocks
		res.Evidence.MaxAgeDuration = params2.Evidence.MaxAgeDuration
		res.Evidence.MaxBytes = params2.Evidence.MaxBytes
	}
	if params2.Validator != nil {
		// Copy params2.Validator.PubkeyTypes, and set result's value to the copy.
		// This avoids having to initialize the slice to 0 values, and then write to it again.
		res.Validator.PubKeyTypes = append([]string{}, params2.Validator.PubKeyTypes...)
	}
	if params2.Version != nil {
		res.Version.AppVersion = params2.Version.AppVersion
	}
	if params2.Synchrony != nil {
		if params2.Synchrony.MessageDelay != nil {
			res.Synchrony.MessageDelay = *params2.Synchrony.GetMessageDelay()
		}
		if params2.Synchrony.Precision != nil {
			res.Synchrony.Precision = *params2.Synchrony.GetPrecision()
		}
	}
	if params2.Timeout != nil {
		if params2.Timeout.Propose != nil {
			res.Timeout.Propose = *params2.Timeout.GetPropose()
		}
		if params2.Timeout.ProposeDelta != nil {
			res.Timeout.ProposeDelta = *params2.Timeout.GetProposeDelta()
		}
		if params2.Timeout.Vote != nil {
			res.Timeout.Vote = *params2.Timeout.GetVote()
		}
		if params2.Timeout.VoteDelta != nil {
			res.Timeout.VoteDelta = *params2.Timeout.GetVoteDelta()
		}
		if params2.Timeout.Commit != nil {
			res.Timeout.Commit = *params2.Timeout.GetCommit()
		}
		res.Timeout.BypassCommitTimeout = params2.Timeout.GetBypassCommitTimeout()
	}
	if params2.Abci != nil {
		res.ABCI.VoteExtensionsEnableHeight = params2.Abci.GetVoteExtensionsEnableHeight()
		res.ABCI.RecheckTx = params2.Abci.GetRecheckTx()
	}
	return res
}

func (params *ConsensusParams) ToProto() tmproto.ConsensusParams {
	return tmproto.ConsensusParams{
		Block: &tmproto.BlockParams{
			MaxBytes: params.Block.MaxBytes,
			MaxGas:   params.Block.MaxGas,
		},
		Evidence: &tmproto.EvidenceParams{
			MaxAgeNumBlocks: params.Evidence.MaxAgeNumBlocks,
			MaxAgeDuration:  params.Evidence.MaxAgeDuration,
			MaxBytes:        params.Evidence.MaxBytes,
		},
		Validator: &tmproto.ValidatorParams{
			PubKeyTypes: params.Validator.PubKeyTypes,
		},
		Version: &tmproto.VersionParams{
			AppVersion: params.Version.AppVersion,
		},
		Synchrony: &tmproto.SynchronyParams{
			MessageDelay: &params.Synchrony.MessageDelay,
			Precision:    &params.Synchrony.Precision,
		},
		Timeout: &tmproto.TimeoutParams{
			Propose:             &params.Timeout.Propose,
			ProposeDelta:        &params.Timeout.ProposeDelta,
			Vote:                &params.Timeout.Vote,
			VoteDelta:           &params.Timeout.VoteDelta,
			Commit:              &params.Timeout.Commit,
			BypassCommitTimeout: params.Timeout.BypassCommitTimeout,
		},
		Abci: &tmproto.ABCIParams{
			VoteExtensionsEnableHeight: params.ABCI.VoteExtensionsEnableHeight,
			RecheckTx:                  params.ABCI.RecheckTx,
		},
	}
}

func ConsensusParamsFromProto(pbParams tmproto.ConsensusParams) ConsensusParams {
	c := ConsensusParams{
		Block: BlockParams{
			MaxBytes: pbParams.Block.MaxBytes,
			MaxGas:   pbParams.Block.MaxGas,
		},
		Evidence: EvidenceParams{
			MaxAgeNumBlocks: pbParams.Evidence.MaxAgeNumBlocks,
			MaxAgeDuration:  pbParams.Evidence.MaxAgeDuration,
			MaxBytes:        pbParams.Evidence.MaxBytes,
		},
		Validator: ValidatorParams{
			PubKeyTypes: pbParams.Validator.PubKeyTypes,
		},
		Version: VersionParams{
			AppVersion: pbParams.Version.AppVersion,
		},
	}
	if pbParams.Synchrony != nil {
		if pbParams.Synchrony.MessageDelay != nil {
			c.Synchrony.MessageDelay = *pbParams.Synchrony.GetMessageDelay()
		}
		if pbParams.Synchrony.Precision != nil {
			c.Synchrony.Precision = *pbParams.Synchrony.GetPrecision()
		}
	}
	if pbParams.Timeout != nil {
		if pbParams.Timeout.Propose != nil {
			c.Timeout.Propose = *pbParams.Timeout.GetPropose()
		}
		if pbParams.Timeout.ProposeDelta != nil {
			c.Timeout.ProposeDelta = *pbParams.Timeout.GetProposeDelta()
		}
		if pbParams.Timeout.Vote != nil {
			c.Timeout.Vote = *pbParams.Timeout.GetVote()
		}
		if pbParams.Timeout.VoteDelta != nil {
			c.Timeout.VoteDelta = *pbParams.Timeout.GetVoteDelta()
		}
		if pbParams.Timeout.Commit != nil {
			c.Timeout.Commit = *pbParams.Timeout.GetCommit()
		}
		c.Timeout.BypassCommitTimeout = pbParams.Timeout.BypassCommitTimeout
	}
	if pbParams.Abci != nil {
		c.ABCI.VoteExtensionsEnableHeight = pbParams.Abci.GetVoteExtensionsEnableHeight()
		c.ABCI.RecheckTx = pbParams.Abci.GetRecheckTx()
	}
	return c
}
