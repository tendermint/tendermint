package types

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	gogotypes "github.com/gogo/protobuf/types"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/libs/bits"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmmath "github.com/tendermint/tendermint/libs/math"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/version"
)

const (
	// MaxHeaderBytes is a maximum header size.
	// NOTE: Because app hash can be of arbitrary size, the header is therefore not
	// capped in size and thus this number should be seen as a soft max
	MaxHeaderBytes       int64 = 646
	MaxCoreChainLockSize int64 = 132

	// MaxOverheadForBlock - maximum overhead to encode a block (up to
	// MaxBlockSizeBytes in size) not including it's parts except Data.
	// This means it also excludes the overhead for individual transactions.
	//
	// Uvarint length of MaxBlockSizeBytes: 4 bytes
	// 2 fields (2 embedded):               2 bytes
	// Uvarint length of Data.Txs:          4 bytes
	// Data.Txs field:                      1 byte
	MaxOverheadForBlock int64 = 11
)

// Block defines the atomic unit of a Tendermint blockchain.
type Block struct {
	mtx tmsync.Mutex

	Header        `json:"header"`
	Data          `json:"data"`
	CoreChainLock *CoreChainLock `json:"core_chain_lock"`
	Evidence      EvidenceData   `json:"evidence"`
	LastCommit    *Commit        `json:"last_commit"`
}

// ValidateBasic performs basic validation that doesn't involve state data.
// It checks the internal consistency of the block.
// Further validation is done using state#ValidateBlock.
func (b *Block) ValidateBasic() error {
	if b == nil {
		return errors.New("nil block")
	}

	b.mtx.Lock()
	defer b.mtx.Unlock()

	if err := b.Header.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid header: %w", err)
	}

	if b.CoreChainLock != nil {
		if err := b.CoreChainLock.ValidateBasic(); err != nil {
			return fmt.Errorf("invalid chain lock data: %w", err)
		}
	}

	// Validate the last commit and its hash.
	if b.LastCommit == nil {
		return errors.New("nil LastPrecommits")
	}
	if err := b.LastCommit.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong LastPrecommits: %v", err)
	}

	if !bytes.Equal(b.LastCommitHash, b.LastCommit.Hash()) {
		return fmt.Errorf("wrong Header.LastCommitHash. Expected %v, got %v",
			b.LastCommit.Hash(),
			b.LastCommitHash,
		)
	}

	// NOTE: b.Data.Txs may be nil, but b.Data.Hash() still works fine.
	if !bytes.Equal(b.DataHash, b.Data.Hash()) {
		return fmt.Errorf(
			"wrong Header.DataHash. Expected %v, got %v",
			b.Data.Hash(),
			b.DataHash,
		)
	}

	// NOTE: b.Evidence.Evidence may be nil, but we're just looping.
	for i, ev := range b.Evidence.Evidence {
		if err := ev.ValidateBasic(); err != nil {
			return fmt.Errorf("invalid evidence (#%d): %v", i, err)
		}
	}

	if !bytes.Equal(b.EvidenceHash, b.Evidence.Hash()) {
		return fmt.Errorf("wrong Header.EvidenceHash. Expected %v, got %v",
			b.EvidenceHash,
			b.Evidence.Hash(),
		)
	}

	return nil
}

// fillHeader fills in any remaining header fields that are a function of the block data
func (b *Block) fillHeader() {
	if b.LastCommitHash == nil {
		b.LastCommitHash = b.LastCommit.Hash()
	}
	if b.DataHash == nil {
		b.DataHash = b.Data.Hash()
	}
	if b.EvidenceHash == nil {
		b.EvidenceHash = b.Evidence.Hash()
	}
}

// Hash computes and returns the block hash.
// If the block is incomplete, block hash is nil for safety.
func (b *Block) Hash() tmbytes.HexBytes {
	if b == nil {
		return nil
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()

	if b.LastCommit == nil {
		return nil
	}
	b.fillHeader()
	return b.Header.Hash()
}

// MakePartSet returns a PartSet containing parts of a serialized block.
// This is the form in which the block is gossipped to peers.
// CONTRACT: partSize is greater than zero.
func (b *Block) MakePartSet(partSize uint32) *PartSet {
	if b == nil {
		return nil
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()

	pbb, err := b.ToProto()
	if err != nil {
		panic(err)
	}
	bz, err := proto.Marshal(pbb)
	if err != nil {
		panic(err)
	}
	return NewPartSetFromData(bz, partSize)
}

// HashesTo is a convenience function that checks if a block hashes to the given argument.
// Returns false if the block is nil or the hash is empty.
func (b *Block) HashesTo(hash []byte) bool {
	if len(hash) == 0 {
		return false
	}
	if b == nil {
		return false
	}
	return bytes.Equal(b.Hash(), hash)
}

// Size returns size of the block in bytes.
func (b *Block) Size() int {
	pbb, err := b.ToProto()
	if err != nil {
		return 0
	}

	return pbb.Size()
}

// String returns a string representation of the block
//
// See StringIndented.
func (b *Block) String() string {
	return b.StringIndented("")
}

// StringIndented returns an indented String.
//
// Header
// Data
// Evidence
// LastCommit
// Hash
func (b *Block) StringIndented(indent string) string {
	if b == nil {
		return "nil-Block"
	}
	return fmt.Sprintf(`Block{
%s  %v
%s  %v
%s  %v
%s  %v
%s  %v
%s}#%v`,
		indent, b.Header.StringIndented(indent+"  "),
		indent, b.CoreChainLock.StringIndented(indent+"  "),
		indent, b.Data.StringIndented(indent+"  "),
		indent, b.Evidence.StringIndented(indent+"  "),
		indent, b.LastCommit.StringIndented(indent+"  "),
		indent, b.Hash())
}

// StringShort returns a shortened string representation of the block.
func (b *Block) StringShort() string {
	if b == nil {
		return "nil-Block"
	}
	return fmt.Sprintf("Block#%X", b.Hash())
}

// ToProto converts Block to protobuf
func (b *Block) ToProto() (*tmproto.Block, error) {
	if b == nil {
		return nil, errors.New("nil Block")
	}

	pb := new(tmproto.Block)

	pb.Header = *b.Header.ToProto()
	pb.CoreChainLock = b.CoreChainLock.ToProto()
	pb.LastCommit = b.LastCommit.ToProto()
	pb.Data = b.Data.ToProto()

	protoEvidence, err := b.Evidence.ToProto()
	if err != nil {
		return nil, err
	}
	pb.Evidence = *protoEvidence

	return pb, nil
}

// FromProto sets a protobuf Block to the given pointer.
// It returns an error if the block is invalid.
func BlockFromProto(bp *tmproto.Block) (*Block, error) {
	if bp == nil {
		return nil, errors.New("nil block")
	}

	b := new(Block)
	h, err := HeaderFromProto(&bp.Header)
	if err != nil {
		return nil, err
	}
	b.Header = h
	data, err := DataFromProto(&bp.Data)
	if err != nil {
		return nil, err
	}
	b.Data = data
	if err := b.Evidence.FromProto(&bp.Evidence); err != nil {
		return nil, err
	}

	coreChainLock, err := CoreChainLockFromProto(bp.CoreChainLock)
	if err != nil {
		return nil, err
	}

	b.CoreChainLock = coreChainLock

	if bp.LastCommit != nil {
		lc, err := CommitFromProto(bp.LastCommit)
		if err != nil {
			return nil, err
		}
		b.LastCommit = lc
	}

	return b, b.ValidateBasic()
}

//-----------------------------------------------------------------------------

// MaxDataBytes returns the maximum size of block's data.
//
// XXX: Panics on negative result.

func MaxDataBytes(maxBytes int64, keyType crypto.KeyType, evidenceBytes int64, valsCount int) int64 {
	maxDataBytes := maxBytes -
		MaxOverheadForBlock -
		MaxHeaderBytes -
		MaxCoreChainLockSize -
		MaxCommitBytes(valsCount, keyType) -
		evidenceBytes

	if maxDataBytes < 0 {
		panic(fmt.Sprintf(
			"Negative MaxDataBytes. Block.MaxBytes=%d is too small to accommodate header&lastCommit&evidence=%d",
			maxBytes,
			-(maxDataBytes - maxBytes),
		))
	}

	return maxDataBytes
}

// MaxDataBytesNoEvidence returns the maximum size of block's data when
// evidence count is unknown. MaxEvidencePerBlock will be used for the size
// of evidence.
//
// XXX: Panics on negative result.

func MaxDataBytesNoEvidence(maxBytes int64, keyType crypto.KeyType, valsCount int) int64 {
	maxDataBytes := maxBytes -
		MaxOverheadForBlock -
		MaxHeaderBytes -
		MaxCoreChainLockSize -
		MaxCommitBytes(valsCount, keyType)

	if maxDataBytes < 0 {
		panic(fmt.Sprintf(
			"Negative MaxDataBytesUnknownEvidence. Block.MaxBytes=%d is too small to accommodate header&lastCommit&evidence=%d",
			maxBytes,
			-(maxDataBytes - maxBytes),
		))
	}

	return maxDataBytes
}

//-----------------------------------------------------------------------------

// Header defines the structure of a Tendermint block header.
// NOTE: changes to the Header should be duplicated in:
// - header.Hash()
// - abci.Header
// - https://github.com/tendermint/spec/blob/master/spec/blockchain/blockchain.md
type Header struct {
	// basic block info
	Version               tmversion.Consensus `json:"version"`
	ChainID               string              `json:"chain_id"`
	Height                int64               `json:"height"`
	CoreChainLockedHeight uint32              `json:"core_chain_locked_height"`
	Time                  time.Time           `json:"time"`

	// prev block info
	LastBlockID BlockID `json:"last_block_id"`

	// hashes of block data
	LastCommitHash tmbytes.HexBytes `json:"last_commit_hash"` // commit from validators from the last block
	DataHash       tmbytes.HexBytes `json:"data_hash"`        // transactions

	// hashes from the app output from the prev block
	ValidatorsHash     tmbytes.HexBytes `json:"validators_hash"`      // validators for the current block
	NextValidatorsHash tmbytes.HexBytes `json:"next_validators_hash"` // validators for the next block
	ConsensusHash      tmbytes.HexBytes `json:"consensus_hash"`       // consensus params for current block
	AppHash            tmbytes.HexBytes `json:"app_hash"`             // state after txs from the previous block
	// root hash of all results from the txs from the previous block
	// see `deterministicResponseDeliverTx` to understand which parts of a tx is hashed into here
	LastResultsHash tmbytes.HexBytes `json:"last_results_hash"`

	// consensus info
	EvidenceHash      tmbytes.HexBytes `json:"evidence_hash"`        // evidence included in the block
	ProposerProTxHash ProTxHash        `json:"proposer_pro_tx_hash"` // original proposer of the block
}

// Populate the Header with state-derived data.
// Call this after MakeBlock to complete the Header.
func (h *Header) Populate(
	version tmversion.Consensus, chainID string,
	timestamp time.Time, lastBlockID BlockID,
	valHash, nextValHash []byte,
	consensusHash, appHash, lastResultsHash []byte,
	proposerProTxHash ProTxHash,
) {
	h.Version = version
	h.ChainID = chainID
	h.Time = timestamp
	h.LastBlockID = lastBlockID
	h.ValidatorsHash = valHash
	h.NextValidatorsHash = nextValHash
	h.ConsensusHash = consensusHash
	h.AppHash = appHash
	h.LastResultsHash = lastResultsHash
	h.ProposerProTxHash = proposerProTxHash
}

// ValidateBasic performs stateless validation on a Header returning an error
// if any validation fails.
//
// NOTE: Timestamp validation is subtle and handled elsewhere.
func (h Header) ValidateBasic() error {
	if h.Version.Block != version.BlockProtocol {
		return fmt.Errorf("block protocol is incorrect: got: %d, want: %d ", h.Version.Block, version.BlockProtocol)
	}
	if len(h.ChainID) > MaxChainIDLen {
		return fmt.Errorf("chainID is too long; got: %d, max: %d", len(h.ChainID), MaxChainIDLen)
	}

	if h.Height < 0 {
		return errors.New("negative Height")
	} else if h.Height == 0 {
		return errors.New("zero Height")
	}

	if err := h.LastBlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong LastBlockID: %w", err)
	}

	if err := ValidateHash(h.LastCommitHash); err != nil {
		return fmt.Errorf("wrong LastCommitHash: %v", err)
	}

	if err := ValidateHash(h.DataHash); err != nil {
		return fmt.Errorf("wrong DataHash: %v", err)
	}

	if err := ValidateHash(h.EvidenceHash); err != nil {
		return fmt.Errorf("wrong EvidenceHash: %v", err)
	}

	if len(h.ProposerProTxHash) != crypto.DefaultHashSize {
		return fmt.Errorf(
			"invalid ProposerProTxHash length; got: %d, expected: %d",
			len(h.ProposerProTxHash), crypto.DefaultHashSize,
		)
	}

	// Basic validation of hashes related to application data.
	// Will validate fully against state in state#ValidateBlock.
	if err := ValidateHash(h.ValidatorsHash); err != nil {
		return fmt.Errorf("wrong ValidatorsHash: %v", err)
	}
	if err := ValidateHash(h.NextValidatorsHash); err != nil {
		return fmt.Errorf("wrong NextValidatorsHash: %v", err)
	}
	if err := ValidateHash(h.ConsensusHash); err != nil {
		return fmt.Errorf("wrong ConsensusHash: %v", err)
	}
	// NOTE: AppHash is arbitrary length
	if err := ValidateHash(h.LastResultsHash); err != nil {
		return fmt.Errorf("wrong LastResultsHash: %v", err)
	}

	return nil
}

// Hash returns the hash of the header.
// It computes a Merkle tree from the header fields
// ordered as they appear in the Header.
// Returns nil if ValidatorHash is missing,
// since a Header is not valid unless there is
// a ValidatorsHash (corresponding to the validator set).
func (h *Header) Hash() tmbytes.HexBytes {
	if h == nil || len(h.ValidatorsHash) == 0 {
		return nil
	}
	hbz, err := h.Version.Marshal()
	if err != nil {
		return nil
	}

	pbt, err := gogotypes.StdTimeMarshal(h.Time)
	if err != nil {
		return nil
	}

	pbbi := h.LastBlockID.ToProto()
	bzbi, err := pbbi.Marshal()
	if err != nil {
		return nil
	}
	return merkle.HashFromByteSlices([][]byte{
		hbz,
		cdcEncode(h.ChainID),
		cdcEncode(h.Height),
		cdcEncode(h.CoreChainLockedHeight),
		pbt,
		bzbi,
		cdcEncode(h.LastCommitHash),
		cdcEncode(h.DataHash),
		cdcEncode(h.ValidatorsHash),
		cdcEncode(h.NextValidatorsHash),
		cdcEncode(h.ConsensusHash),
		cdcEncode(h.AppHash),
		cdcEncode(h.LastResultsHash),
		cdcEncode(h.EvidenceHash),
		cdcEncode(h.ProposerProTxHash),
	})
}

// StringIndented returns an indented string representation of the header.
func (h *Header) StringIndented(indent string) string {
	if h == nil {
		return "nil-Header"
	}
	return fmt.Sprintf(`Header{
%s  Version:        %v
%s  ChainID:        %v
%s  Height:         %v
%s  CoreCLHeight:   %v
%s  Time:           %v
%s  LastBlockID:    %v
%s  LastPrecommits:     %v
%s  Data:           %v
%s  Validators:     %v
%s  NextValidators: %v
%s  App:            %v
%s  Consensus:      %v
%s  Results:        %v
%s  Evidence:       %v
%s  Proposer:       %v
%s}#%v`,
		indent, h.Version,
		indent, h.ChainID,
		indent, h.Height,
		indent, h.CoreChainLockedHeight,
		indent, h.Time,
		indent, h.LastBlockID,
		indent, h.LastCommitHash,
		indent, h.DataHash,
		indent, h.ValidatorsHash,
		indent, h.NextValidatorsHash,
		indent, h.AppHash,
		indent, h.ConsensusHash,
		indent, h.LastResultsHash,
		indent, h.EvidenceHash,
		indent, h.ProposerProTxHash,
		indent, h.Hash())
}

// ToProto converts Header to protobuf
func (h *Header) ToProto() *tmproto.Header {
	if h == nil {
		return nil
	}

	return &tmproto.Header{
		Version:               h.Version,
		ChainID:               h.ChainID,
		Height:                h.Height,
		CoreChainLockedHeight: h.CoreChainLockedHeight,
		Time:                  h.Time,
		LastBlockId:           h.LastBlockID.ToProto(),
		ValidatorsHash:        h.ValidatorsHash,
		NextValidatorsHash:    h.NextValidatorsHash,
		ConsensusHash:         h.ConsensusHash,
		AppHash:               h.AppHash,
		DataHash:              h.DataHash,
		EvidenceHash:          h.EvidenceHash,
		LastResultsHash:       h.LastResultsHash,
		LastCommitHash:        h.LastCommitHash,
		ProposerProTxHash:     h.ProposerProTxHash,
	}
}

// FromProto sets a protobuf Header to the given pointer.
// It returns an error if the header is invalid.
func HeaderFromProto(ph *tmproto.Header) (Header, error) {
	if ph == nil {
		return Header{}, errors.New("nil Header")
	}

	h := new(Header)

	bi, err := BlockIDFromProto(&ph.LastBlockId)
	if err != nil {
		return Header{}, err
	}

	h.Version = ph.Version
	h.ChainID = ph.ChainID
	h.Height = ph.Height
	h.CoreChainLockedHeight = ph.CoreChainLockedHeight
	h.Time = ph.Time
	h.Height = ph.Height
	h.LastBlockID = *bi
	h.ValidatorsHash = ph.ValidatorsHash
	h.NextValidatorsHash = ph.NextValidatorsHash
	h.ConsensusHash = ph.ConsensusHash
	h.AppHash = ph.AppHash
	h.DataHash = ph.DataHash
	h.EvidenceHash = ph.EvidenceHash
	h.LastResultsHash = ph.LastResultsHash
	h.LastCommitHash = ph.LastCommitHash
	h.ProposerProTxHash = ph.ProposerProTxHash

	return *h, h.ValidateBasic()
}

//-------------------------------------

// BlockIDFlag indicates which BlockID the signature is for.
type BlockIDFlag byte

const (
	// BlockIDFlagAbsent - no vote was received from a validator.
	BlockIDFlagAbsent BlockIDFlag = iota + 1
	// BlockIDFlagCommit - voted for the Commit.BlockID.
	BlockIDFlagCommit
	// BlockIDFlagNil - voted for nil.
	BlockIDFlagNil
)

const (
	// Max size of commit without any commitSigs -> 82 for BlockID, 34 for StateID, 8 for Height, 4 for Round.
	MaxCommitOverheadBytes int64 = 130
	// Commit sig size is made up of 96 bytes for each signature, 32 bytes for the proTxHash,
	// 1 byte for the flag
	MaxCommitSigBytesBLS12381 int64 = 232
)

func MaxCommitSigBytesForKeyType(keyType crypto.KeyType) int64 {
	switch keyType {
	case crypto.BLS12381:
		return MaxCommitSigBytesBLS12381
	default:
		return MaxCommitSigBytesBLS12381
	}
}

func MaxCommitBytes(valCount int, keyType crypto.KeyType) int64 {
	// From the repeated commit sig field
	var maxCommitBytes = MaxCommitSigBytesForKeyType(keyType)
	var protoEncodingOverhead int64
	// protobuff encodes up to signed 128 bits with 2 extra bits, more would take 3 or more. While we could have more
	// than 3 in the case of very large signatures (maybe lattice based signatures), this is good enough for now
	if maxCommitBytes < 128 {
		protoEncodingOverhead = 2
	} else {
		protoEncodingOverhead = 3
	}
	return MaxCommitOverheadBytes + ((maxCommitBytes + protoEncodingOverhead) * int64(valCount))
}

//-------------------------------------

// Commit contains the evidence that a block was committed by a set of validators.
// NOTE: Commit is empty for height 1, but never nil.
type Commit struct {
	// NOTE: The signatures are in order of proTxHash to preserve the bonded
	// ValidatorSet order.
	// Any peer with a block can gossip signatures by index with a peer without
	// recalculating the active ValidatorSet.
	Height                  int64       `json:"height"`
	Round                   int32       `json:"round"`
	BlockID                 BlockID     `json:"block_id"`
	StateID                 StateID     `json:"state_id"`
	QuorumHash              []byte      `json:"quorum_hash"`
	ThresholdBlockSignature []byte      `json:"threshold_block_signature"`
	ThresholdStateSignature []byte      `json:"threshold_state_signature"`

	// Memoized in first call to corresponding method.
	// NOTE: can't memoize in constructor because constructor isn't used for
	// unmarshaling.
	hash     tmbytes.HexBytes
	bitArray *bits.BitArray
}

// NewCommit returns a new Commit.
func NewCommit(height int64, round int32, blockID BlockID, stateID StateID, quorumHash []byte,
	thresholdBlockSignature []byte, thresholdStateSignature []byte) *Commit {
	return &Commit{
		Height:                  height,
		Round:                   round,
		BlockID:                 blockID,
		StateID:                 stateID,
		QuorumHash:              quorumHash,
		ThresholdBlockSignature: thresholdBlockSignature,
		ThresholdStateSignature: thresholdStateSignature,
	}
}

// GetCanonicalVote returns the message that is being voted on in the form of a vote without signatures.
//
func (commit *Commit) GetCanonicalVote() *Vote {
	return &Vote{
		Type:    tmproto.PrecommitType,
		Height:  commit.Height,
		Round:   commit.Round,
		BlockID: commit.BlockID,
		StateID: commit.StateID,
	}
}

// VoteBlockRequestId returns the requestId Hash of the Vote corresponding to valIdx for
// signing.
//
// Panics if valIdx >= commit.Size().
//
func (commit *Commit) VoteBlockRequestId() []byte {
	requestIdMessage := []byte("dpbvote")
	heightByteArray := make([]byte, 8)
	binary.LittleEndian.PutUint64(heightByteArray, uint64(commit.Height))
	roundByteArray := make([]byte, 4)
	binary.LittleEndian.PutUint32(roundByteArray, uint32(commit.Round))

	requestIdMessage = append(requestIdMessage, heightByteArray...)
	requestIdMessage = append(requestIdMessage, roundByteArray...)

	return crypto.Sha256(requestIdMessage)
}

// CanonicalVoteVerifySignBytes returns the bytes of the Canonical Vote that is threshold signed.
//
func (commit *Commit) CanonicalVoteVerifySignBytes(chainID string) []byte {
	voteCanonical := commit.GetCanonicalVote()
	vCanonical := voteCanonical.ToProto()
	return VoteBlockSignBytes(chainID, vCanonical)
}

// CanonicalVoteVerifySignId returns the signId bytes of the Canonical Vote that is threshold signed.
//
func (commit *Commit) CanonicalVoteVerifySignId(chainID string, quorumType btcjson.LLMQType, quorumHash []byte) []byte {
	voteCanonical := commit.GetCanonicalVote()
	vCanonical := voteCanonical.ToProto()
	return VoteBlockSignId(chainID, vCanonical, quorumType, quorumHash)
}

// VoteStateSignId returns the signId bytes of the state for the Vote corresponding to valIdx for
// signing.
//
// Panics if valIdx >= commit.Size().
//
func (commit *Commit) VoteStateSignId(chainID string, quorumType btcjson.LLMQType, quorumHash []byte)  []byte {
	v := commit.GetCanonicalVote()
	return VoteStateSignId(chainID, v.ToProto(), quorumType, quorumHash)
}

// VoteStateRequestId returns the requestId Hash of the Vote corresponding to valIdx for
// signing.
//
// Panics if valIdx >= commit.Size().
//
func (commit *Commit) VoteStateRequestId() []byte {
	requestIdMessage := []byte("dpsvote")
	heightByteArray := make([]byte, 8)
	binary.LittleEndian.PutUint64(heightByteArray, uint64(commit.Height))
	roundByteArray := make([]byte, 4)
	binary.LittleEndian.PutUint32(roundByteArray, uint32(commit.Round))

	requestIdMessage = append(requestIdMessage, heightByteArray...)
	requestIdMessage = append(requestIdMessage, roundByteArray...)

	return crypto.Sha256(requestIdMessage)
}


// CanonicalVoteStateSignBytes returns the bytes of the State corresponding to valIdx for
// signing.
//
// Panics if valIdx >= commit.Size().
//
// See VoteSignBytes
func (commit *Commit) CanonicalVoteStateSignBytes(chainID string) []byte {
	v := commit.GetCanonicalVote()
	return VoteStateSignBytes(chainID, v.ToProto())
}

func (commit *Commit) CanonicalVoteStateSignId(chainID string, quorumType btcjson.LLMQType, quorumHash []byte) []byte {
	v := commit.GetCanonicalVote()
	return VoteStateSignId(chainID, v.ToProto(), quorumType, quorumHash)
}

// Type returns the vote type of the commit, which is always VoteTypePrecommit
// Implements VoteSetReader.
func (commit *Commit) Type() byte {
	return byte(tmproto.PrecommitType)
}

// GetHeight returns height of the commit.
// Implements VoteSetReader.
func (commit *Commit) GetHeight() int64 {
	return commit.Height
}

// GetRound returns height of the commit.
// Implements VoteSetReader.
func (commit *Commit) GetRound() int32 {
	return commit.Round
}

// IsCommit returns true if there is at least one signature.
// Implements VoteSetReader.
func (commit *Commit) IsCommit() bool {
	return len(commit.ThresholdBlockSignature) == SignatureSize
}

// ValidateBasic performs basic validation that doesn't involve state data.
// Does not actually check the cryptographic signatures.
func (commit *Commit) ValidateBasic() error {
	if commit.Height < 0 {
		return errors.New("negative Height")
	}
	if commit.Round < 0 {
		return errors.New("negative Round")
	}

	if commit.Height >= 1 {
		if commit.BlockID.IsZero() {
			return errors.New("commit cannot be for nil block")
		}
		if len(commit.ThresholdBlockSignature) != SignatureSize {
			return fmt.Errorf("block threshold signature is wrong size (wanted: %d, received: %d)", SignatureSize, len(commit.ThresholdBlockSignature))
		}
		if len(commit.ThresholdStateSignature) != SignatureSize {
			return fmt.Errorf("state threshold signature is wrong size (wanted: %d, received: %d)", SignatureSize, len(commit.ThresholdStateSignature))
		}
	}
	return nil
}

// Hash returns the hash of the commit
func (commit *Commit) Hash() tmbytes.HexBytes {
	if commit == nil {
		return nil
	}
	if commit.hash == nil {
		bs := make([][]byte, 2)
		bs[0] = commit.ThresholdBlockSignature
		bs[1] = commit.ThresholdStateSignature
		commit.hash = merkle.HashFromByteSlices(bs)
	}
	return commit.hash
}

// StringIndented returns a string representation of the commit.
func (commit *Commit) StringIndented(indent string) string {
	if commit == nil {
		return "nil-Commit"
	}
	return fmt.Sprintf(`Commit{
%s  Height:     %d
%s  Round:      %d
%s  BlockID:    %v
%s  StateID:    %v
%s  BlockSignature: %v
%s  StateSignature: %v
%s}#%v`,
		indent, commit.Height,
		indent, commit.Round,
		indent, commit.BlockID,
		indent, commit.StateID,
		indent, commit.ThresholdBlockSignature,
		indent, commit.ThresholdStateSignature,
		indent, commit.hash)
}

// ToProto converts Commit to protobuf
func (commit *Commit) ToProto() *tmproto.Commit {
	if commit == nil {
		return nil
	}

	c := new(tmproto.Commit)

	c.Height = commit.Height
	c.Round = commit.Round
	c.BlockID = commit.BlockID.ToProto()
	c.StateID = commit.StateID.ToProto()

	c.ThresholdStateSignature = commit.ThresholdStateSignature
	c.ThresholdBlockSignature = commit.ThresholdBlockSignature

	c.QuorumHash = commit.QuorumHash

	return c
}

// CommitFromProto creates a commit from a protobuf commit message.
// It returns an error if the commit is invalid.
func CommitFromProto(cp *tmproto.Commit) (*Commit, error) {
	if cp == nil {
		return nil, errors.New("nil Commit")
	}

	var (
		commit = new(Commit)
	)

	bi, err := BlockIDFromProto(&cp.BlockID)
	if err != nil {
		return nil, err
	}

	si, err := StateIDFromProto(&cp.StateID)
	if err != nil {
		return nil, err
	}

	commit.QuorumHash = cp.QuorumHash
	commit.ThresholdBlockSignature = cp.ThresholdBlockSignature
	commit.ThresholdStateSignature = cp.ThresholdStateSignature

	commit.Height = cp.Height
	commit.Round = cp.Round
	commit.BlockID = *bi
	commit.StateID = *si

	return commit, commit.ValidateBasic()
}

//-----------------------------------------------------------------------------

// Data contains the set of transactions included in the block
type Data struct {

	// Txs that will be applied by state @ block.Height+1.
	// NOTE: not all txs here are valid.  We're just agreeing on the order first.
	// This means that block.AppHash does not include these txs.
	Txs Txs `json:"txs"`

	// Volatile
	hash tmbytes.HexBytes
}

// Hash returns the hash of the data
func (data *Data) Hash() tmbytes.HexBytes {
	if data == nil {
		return (Txs{}).Hash()
	}
	if data.hash == nil {
		data.hash = data.Txs.Hash() // NOTE: leaves of merkle tree are TxIDs
	}
	return data.hash
}

// StringIndented returns an indented string representation of the transactions.
func (data *Data) StringIndented(indent string) string {
	if data == nil {
		return "nil-Data"
	}
	txStrings := make([]string, tmmath.MinInt(len(data.Txs), 21))
	for i, tx := range data.Txs {
		if i == 20 {
			txStrings[i] = fmt.Sprintf("... (%v total)", len(data.Txs))
			break
		}
		txStrings[i] = fmt.Sprintf("%X (%d bytes)", tx.Hash(), len(tx))
	}
	return fmt.Sprintf(`Data{
%s  %v
%s}#%v`,
		indent, strings.Join(txStrings, "\n"+indent+"  "),
		indent, data.hash)
}

// ToProto converts Data to protobuf
func (data *Data) ToProto() tmproto.Data {
	tp := new(tmproto.Data)

	if len(data.Txs) > 0 {
		txBzs := make([][]byte, len(data.Txs))
		for i := range data.Txs {
			txBzs[i] = data.Txs[i]
		}
		tp.Txs = txBzs
	}

	return *tp
}

// DataFromProto takes a protobuf representation of Data &
// returns the native type.
func DataFromProto(dp *tmproto.Data) (Data, error) {
	if dp == nil {
		return Data{}, errors.New("nil data")
	}
	data := new(Data)

	if len(dp.Txs) > 0 {
		txBzs := make(Txs, len(dp.Txs))
		for i := range dp.Txs {
			txBzs[i] = Tx(dp.Txs[i])
		}
		data.Txs = txBzs
	} else {
		data.Txs = Txs{}
	}

	return *data, nil
}

//-----------------------------------------------------------------------------

// EvidenceData contains any evidence of malicious wrong-doing by validators
type EvidenceData struct {
	Evidence EvidenceList `json:"evidence"`

	// Volatile. Used as cache
	hash     tmbytes.HexBytes
	byteSize int64
}

// Hash returns the hash of the data.
func (data *EvidenceData) Hash() tmbytes.HexBytes {
	if data.hash == nil {
		data.hash = data.Evidence.Hash()
	}
	return data.hash
}

// ByteSize returns the total byte size of all the evidence
func (data *EvidenceData) ByteSize() int64 {
	if data.byteSize == 0 && len(data.Evidence) != 0 {
		pb, err := data.ToProto()
		if err != nil {
			panic(err)
		}
		data.byteSize = int64(pb.Size())
	}
	return data.byteSize
}

// StringIndented returns a string representation of the evidence.
func (data *EvidenceData) StringIndented(indent string) string {
	if data == nil {
		return "nil-Evidence"
	}
	evStrings := make([]string, tmmath.MinInt(len(data.Evidence), 21))
	for i, ev := range data.Evidence {
		if i == 20 {
			evStrings[i] = fmt.Sprintf("... (%v total)", len(data.Evidence))
			break
		}
		evStrings[i] = fmt.Sprintf("Evidence:%v", ev)
	}
	return fmt.Sprintf(`EvidenceData{
%s  %v
%s}#%v`,
		indent, strings.Join(evStrings, "\n"+indent+"  "),
		indent, data.hash)
}

// ToProto converts EvidenceData to protobuf
func (data *EvidenceData) ToProto() (*tmproto.EvidenceList, error) {
	if data == nil {
		return nil, errors.New("nil evidence data")
	}

	evi := new(tmproto.EvidenceList)
	eviBzs := make([]tmproto.Evidence, len(data.Evidence))
	for i := range data.Evidence {
		protoEvi, err := EvidenceToProto(data.Evidence[i])
		if err != nil {
			return nil, err
		}
		eviBzs[i] = *protoEvi
	}
	evi.Evidence = eviBzs

	return evi, nil
}

// FromProto sets a protobuf EvidenceData to the given pointer.
func (data *EvidenceData) FromProto(eviData *tmproto.EvidenceList) error {
	if eviData == nil {
		return errors.New("nil evidenceData")
	}

	eviBzs := make(EvidenceList, len(eviData.Evidence))
	for i := range eviData.Evidence {
		evi, err := EvidenceFromProto(&eviData.Evidence[i])
		if err != nil {
			return err
		}
		eviBzs[i] = evi
	}
	data.Evidence = eviBzs
	data.byteSize = int64(eviData.Size())

	return nil
}

//--------------------------------------------------------------------------------

// BlockID
type BlockID struct {
	Hash          tmbytes.HexBytes `json:"hash"`
	PartSetHeader PartSetHeader    `json:"parts"`
}

// Equals returns true if the BlockID matches the given BlockID
func (blockID BlockID) Equals(other BlockID) bool {
	return bytes.Equal(blockID.Hash, other.Hash) &&
		blockID.PartSetHeader.Equals(other.PartSetHeader)
}

// Key returns a machine-readable string representation of the BlockID
func (blockID BlockID) Key() string {
	pbph := blockID.PartSetHeader.ToProto()
	bz, err := pbph.Marshal()
	if err != nil {
		panic(err)
	}

	return fmt.Sprint(string(blockID.Hash), string(bz))
}

// ValidateBasic performs basic validation.
func (blockID BlockID) ValidateBasic() error {
	// Hash can be empty in case of POLBlockID in Proposal.
	if err := ValidateHash(blockID.Hash); err != nil {
		return fmt.Errorf("wrong Hash")
	}
	if err := blockID.PartSetHeader.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong PartSetHeader: %v", err)
	}
	return nil
}

// IsZero returns true if this is the BlockID of a nil block.
func (blockID BlockID) IsZero() bool {
	return len(blockID.Hash) == 0 &&
		blockID.PartSetHeader.IsZero()
}

// IsComplete returns true if this is a valid BlockID of a non-nil block.
func (blockID BlockID) IsComplete() bool {
	return len(blockID.Hash) == tmhash.Size &&
		blockID.PartSetHeader.Total > 0 &&
		len(blockID.PartSetHeader.Hash) == tmhash.Size
}

// String returns a human readable string representation of the BlockID.
//
// 1. hash
// 2. part set header
//
// See PartSetHeader#String
func (blockID BlockID) String() string {
	return fmt.Sprintf(`%v:%v`, blockID.Hash, blockID.PartSetHeader)
}

// ToProto converts BlockID to protobuf
func (blockID *BlockID) ToProto() tmproto.BlockID {
	if blockID == nil {
		return tmproto.BlockID{}
	}

	return tmproto.BlockID{
		Hash:          blockID.Hash,
		PartSetHeader: blockID.PartSetHeader.ToProto(),
	}
}

// FromProto sets a protobuf BlockID to the given pointer.
// It returns an error if the block id is invalid.
func BlockIDFromProto(bID *tmproto.BlockID) (*BlockID, error) {
	if bID == nil {
		return nil, errors.New("nil BlockID")
	}

	blockID := new(BlockID)
	ph, err := PartSetHeaderFromProto(&bID.PartSetHeader)
	if err != nil {
		return nil, err
	}

	blockID.PartSetHeader = *ph
	blockID.Hash = bID.Hash

	return blockID, blockID.ValidateBasic()
}

//--------------------------------------------------------------------------------

// StateID
type StateID struct {
	LastAppHash tmbytes.HexBytes `json:"last_app_hash"`
}

// Equals returns true if the StateID matches the given StateID
func (stateID StateID) Equals(other StateID) bool {
	return bytes.Equal(stateID.LastAppHash, other.LastAppHash)
}

// Key returns a machine-readable string representation of the StateID
func (stateID StateID) Key() string {
	return string(stateID.LastAppHash)
}

// ValidateBasic performs basic validation.
func (stateID StateID) ValidateBasic() error {
	// LastAppHash can be empty in case of genesis block.
	if err := ValidateAppHash(stateID.LastAppHash); err != nil {
		return fmt.Errorf("wrong app Hash")
	}
	return nil
}

// IsZero returns true if this is the StateID of a nil block.
func (stateID StateID) IsZero() bool {
	return len(stateID.LastAppHash) == 0
}

// IsComplete returns true if this is a valid StateID of a non-nil block.
func (stateID StateID) IsComplete() bool {
	return len(stateID.LastAppHash) == tmhash.Size
}

// String returns a human readable string representation of the StateID.
//
// 1. hash
//
func (stateID StateID) String() string {
	return fmt.Sprintf(`%v`, stateID.LastAppHash)
}

// ToProto converts BlockID to protobuf
func (stateID *StateID) ToProto() tmproto.StateID {
	if stateID == nil {
		return tmproto.StateID{}
	}

	return tmproto.StateID{
		LastAppHash: stateID.LastAppHash,
	}
}

// FromProto sets a protobuf BlockID to the given pointer.
// It returns an error if the block id is invalid.
func StateIDFromProto(sID *tmproto.StateID) (*StateID, error) {
	if sID == nil {
		return nil, errors.New("nil StateID")
	}

	stateID := new(StateID)

	stateID.LastAppHash = sID.LastAppHash

	return stateID, stateID.ValidateBasic()
}
