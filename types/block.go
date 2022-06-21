package types

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/gogo/protobuf/proto"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/rs/zerolog"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/merkle"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmmath "github.com/tendermint/tendermint/libs/math"
	tmcons "github.com/tendermint/tendermint/proto/tendermint/consensus"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

const (
	// MaxHeaderBytes is a maximum header size.
	// NOTE: Because app hash can be of arbitrary size, the header is therefore not
	// capped in size and thus this number should be seen as a soft max
	MaxHeaderBytes       int64 = 646
	MaxCoreChainLockSize int64 = 132
	MaxCommitSize        int64 = 374

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
	mtx sync.Mutex

	Header        `json:"header"`
	Data          `json:"data"`
	CoreChainLock *CoreChainLock `json:"core_chain_lock"`
	Evidence      EvidenceList   `json:"evidence"`
	LastCommit    *Commit        `json:"last_commit"`
}

// StateID returns a state ID of this block
func (b *Block) StateID() StateID {
	return StateID{Height: b.Height, LastAppHash: b.AppHash}
}

// BlockID returns a block ID of this block
func (b *Block) BlockID() (BlockID, error) {
	parSet, err := b.MakePartSet(BlockPartSizeBytes)
	if err != nil {
		return BlockID{}, err
	}
	return BlockID{Hash: b.Hash(), PartSetHeader: parSet.Header()}, nil
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
		return fmt.Errorf("wrong LastPrecommits: %w", err)
	}

	if w, g := b.LastCommit.Hash(), b.LastCommitHash; !bytes.Equal(w, g) {
		return fmt.Errorf("wrong Header.LastCommitHash. Expected %X, got %X", w, g)
	}

	// NOTE: b.Data.Txs may be nil, but b.Data.Hash() still works fine.
	if w, g := b.Data.Hash(), b.DataHash; !bytes.Equal(w, g) {
		return fmt.Errorf("wrong Header.DataHash. Expected %X, got %X", w, g)
	}

	// NOTE: b.Evidence may be nil, but we're just looping.
	for i, ev := range b.Evidence {
		if err := ev.ValidateBasic(); err != nil {
			return fmt.Errorf("invalid evidence (#%d): %v", i, err)
		}
	}

	if w, g := b.Evidence.Hash(), b.EvidenceHash; !bytes.Equal(w, g) {
		return fmt.Errorf("wrong Header.EvidenceHash. Expected %X, got %X", w, g)
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
func (b *Block) MakePartSet(partSize uint32) (*PartSet, error) {
	if b == nil {
		return nil, errors.New("nil block")
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()

	pbb, err := b.ToProto()
	if err != nil {
		return nil, err
	}
	bz, err := proto.Marshal(pbb)
	if err != nil {
		return nil, err
	}
	return NewPartSetFromData(bz, partSize), nil
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
		MaxCommitOverheadBytes -
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
func MaxDataBytesNoEvidence(maxBytes int64) int64 {
	maxDataBytes := maxBytes -
		MaxOverheadForBlock -
		MaxHeaderBytes -
		MaxCoreChainLockSize -
		MaxCommitOverheadBytes

	if maxDataBytes < 0 {
		panic(fmt.Sprintf(
			"Negative MaxDataBytesUnknownEvidence. Block.MaxBytes=%d is too small to accommodate header&lastCommit&evidence=%d",
			maxBytes,
			-(maxDataBytes - maxBytes),
		))
	}

	return maxDataBytes
}

// MakeBlock returns a new block with an empty header, except what can be
// computed from itself.
// It populates the same set of fields validated by ValidateBasic.
func MakeBlock(height int64, coreHeight uint32, coreChainLock *CoreChainLock, txs []Tx, lastCommit *Commit,
	evidence []Evidence, proposedAppVersion uint64) *Block {
	block := &Block{
		Header: Header{
			Version:               version.Consensus{Block: version.BlockProtocol, App: 0},
			Height:                height,
			CoreChainLockedHeight: coreHeight,
			ProposedAppVersion:    proposedAppVersion,
		},
		Data: Data{
			Txs: txs,
		},
		CoreChainLock: coreChainLock,
		Evidence:      evidence,
		LastCommit:    lastCommit,
	}
	block.fillHeader()
	return block
}

//-----------------------------------------------------------------------------

// Header defines the structure of a Tenderdash block header.
// NOTE: changes to the Header should be duplicated in:
// - header.Hash()
// - abci.Header
// - https://github.com/tendermint/tendermint/blob/master/spec/core/data_structures.md
type Header struct {
	// basic block info
	Version               version.Consensus `json:"version"`
	ChainID               string            `json:"chain_id"`
	Height                int64             `json:"height,string"`
	CoreChainLockedHeight uint32            `json:"core_chain_locked_height"`
	Time                  time.Time         `json:"time"`

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

	ProposedAppVersion uint64 `json:"proposed_protocol_version"` // proposer's latest available app protocol version
}

// Populate the Header with state-derived data.
// Call this after MakeBlock to complete the Header.
func (h *Header) Populate(
	version version.Consensus, chainID string,
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
		return fmt.Errorf("wrong LastCommitHash: %w", err)
	}

	if err := ValidateHash(h.DataHash); err != nil {
		return fmt.Errorf("wrong DataHash: %w", err)
	}

	if err := ValidateHash(h.EvidenceHash); err != nil {
		return fmt.Errorf("wrong EvidenceHash: %w", err)
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
		return fmt.Errorf("wrong ValidatorsHash: %w", err)
	}
	if err := ValidateHash(h.NextValidatorsHash); err != nil {
		return fmt.Errorf("wrong NextValidatorsHash: %w", err)
	}
	if err := ValidateHash(h.ConsensusHash); err != nil {
		return fmt.Errorf("wrong ConsensusHash: %w", err)
	}
	// NOTE: AppHash is arbitrary length
	if err := ValidateHash(h.LastResultsHash); err != nil {
		return fmt.Errorf("wrong LastResultsHash: %w", err)
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
	hpb := h.Version.ToProto()
	hbz, err := hpb.Marshal()
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
		cdcEncode(h.ProposedAppVersion),
	})
}

// StringIndented returns an indented string representation of the header.
func (h *Header) StringIndented(indent string) string {
	if h == nil {
		return "nil-Header"
	}
	return fmt.Sprintf(`Header{
%s  Version:                 %v
%s  ChainID:                 %v
%s  Height:                  %v
%s  CoreCLHeight:            %v
%s  Time:                    %v
%s  LastBlockID:             %v
%s  LastCommitHash:          %v
%s  Data:                    %v
%s  Validators:              %v
%s  NextValidators:          %v
%s  App:                     %v
%s  Consensus:               %v
%s  Results:                 %v
%s  Evidence:                %v
%s  Proposer:                %v
%s  ProposedAppVersion:      %v
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
		indent, h.ProposedAppVersion,
		indent, h.Hash(),
	)
}

// ToProto converts Header to protobuf
func (h *Header) ToProto() *tmproto.Header {
	if h == nil {
		return nil
	}

	return &tmproto.Header{
		Version:               h.Version.ToProto(),
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
		ProposedAppVersion:    h.ProposedAppVersion,
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

	h.Version = version.Consensus{Block: ph.Version.Block, App: ph.Version.App}
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
	h.ProposedAppVersion = ph.ProposedAppVersion

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
	// MaxCommitOverheadBytes is the max size of commit -> 82 for BlockID, 34 for StateID, 8 for Height, 4 for Round.
	// 96 for Block signature, 96 for State Signature and -> 3 bytes overhead
	MaxCommitOverheadBytes int64 = 329
)

//-------------------------------------

// Commit contains the evidence that a block was committed by a set of validators.
// NOTE: Commit is empty for height 1, but never nil.
type Commit struct {
	// NOTE: The signatures are in order of proTxHash to preserve the bonded
	// ValidatorSet order.
	// Any peer with a block can gossip signatures by index with a peer without
	// recalculating the active ValidatorSet.
	Height                  int64             `json:"height"`
	Round                   int32             `json:"round"`
	BlockID                 BlockID           `json:"block_id"`
	StateID                 StateID           `json:"state_id"`
	QuorumHash              crypto.QuorumHash `json:"quorum_hash"`
	ThresholdBlockSignature []byte            `json:"threshold_block_signature"`
	ThresholdStateSignature []byte            `json:"threshold_state_signature"`
	// ThresholdVoteExtensions keeps the list of recovered threshold signatures for vote-extensions
	ThresholdVoteExtensions []ThresholdExtensionSign `json:"threshold_vote_extensions"`

	// Memoized in first call to corresponding method.
	// NOTE: can't memoize in constructor because constructor isn't used for
	// unmarshaling.
	hash tmbytes.HexBytes
}

// NewCommit returns a new Commit.
func NewCommit(height int64, round int32, blockID BlockID, stateID StateID, commitSigns *CommitSigns) *Commit {
	commit := &Commit{
		Height:  height,
		Round:   round,
		BlockID: blockID,
		StateID: stateID,
	}
	if commitSigns != nil {
		commitSigns.CopyToCommit(commit)
	}
	return commit
}

// GetCanonicalVote returns the message that is being voted on in the form of a vote without signatures.
func (commit *Commit) GetCanonicalVote() *Vote {
	return &Vote{
		Type:    tmproto.PrecommitType,
		Height:  commit.Height,
		Round:   commit.Round,
		BlockID: commit.BlockID,
	}
}

// VoteBlockRequestID returns the requestId Hash of the Vote corresponding to valIdx for
// signing.
//
// Panics if valIdx >= commit.Size().
//
func (commit *Commit) VoteBlockRequestID() []byte {
	requestIDMessage := []byte("dpbvote")
	heightByteArray := make([]byte, 8)
	binary.LittleEndian.PutUint64(heightByteArray, uint64(commit.Height))
	roundByteArray := make([]byte, 4)
	binary.LittleEndian.PutUint32(roundByteArray, uint32(commit.Round))

	requestIDMessage = append(requestIDMessage, heightByteArray...)
	requestIDMessage = append(requestIDMessage, roundByteArray...)

	hash := sha256.Sum256(requestIDMessage)
	return hash[:]
}

// CanonicalVoteVerifySignBytes returns the bytes of the Canonical Vote that is threshold signed.
//
func (commit *Commit) CanonicalVoteVerifySignBytes(chainID string) []byte {
	voteCanonical := commit.GetCanonicalVote()
	vCanonical := voteCanonical.ToProto()
	return VoteBlockSignBytes(chainID, vCanonical)
}

// CanonicalVoteVerifySignID returns the signID bytes of the Canonical Vote that is threshold signed.
//
func (commit *Commit) CanonicalVoteVerifySignID(chainID string, quorumType btcjson.LLMQType, quorumHash []byte) []byte {
	voteCanonical := commit.GetCanonicalVote()
	vCanonical := voteCanonical.ToProto()
	return VoteBlockSignID(chainID, vCanonical, quorumType, quorumHash)
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
	if commit == nil {
		return -1
	}
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
		if commit.BlockID.IsNil() {
			return errors.New("commit cannot be for nil block")
		}
		if len(commit.ThresholdBlockSignature) != SignatureSize {
			return fmt.Errorf(
				"block threshold signature is wrong size (wanted: %d, received: %d)",
				SignatureSize,
				len(commit.ThresholdBlockSignature),
			)
		}
		if len(commit.ThresholdStateSignature) != SignatureSize {
			return fmt.Errorf(
				"state threshold signature is wrong size (wanted: %d, received: %d)",
				SignatureSize,
				len(commit.ThresholdStateSignature),
			)
		}
	}
	return nil
}

// Broadcasts HasCommitMessage to peers that care.
func (commit *Commit) HasCommitMessage() *tmcons.HasCommit {
	return &tmcons.HasCommit{
		Height: commit.Height,
		Round:  commit.Round,
	}
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

// String returns a string representation of the block
//
// See StringIndented.
func (commit *Commit) String() string {
	if commit == nil {
		return "nil-Commit"
	}
	return fmt.Sprintf(
		`Commit{H: %d, R: %d, BlockID: %v, StateID: %v, QuorumHash %v, BlockSignature: %v, StateSignature: %v}#%v`,
		commit.Height,
		commit.Round,
		commit.BlockID,
		commit.StateID,
		commit.QuorumHash,
		base64.StdEncoding.EncodeToString(commit.ThresholdBlockSignature),
		base64.StdEncoding.EncodeToString(commit.ThresholdStateSignature),
		commit.hash)
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
		indent, base64.StdEncoding.EncodeToString(commit.ThresholdBlockSignature),
		indent, base64.StdEncoding.EncodeToString(commit.ThresholdStateSignature),
		indent, commit.hash)
}

// MarshalZerologObject formats this object for logging purposes
func (commit *Commit) MarshalZerologObject(e *zerolog.Event) {
	if commit != nil {
		e.Int64("height", commit.Height)
		e.Int32("round", commit.Round)
		e.Str("BlockID.Hash", commit.BlockID.Hash.String())
		e.Str("StateID", commit.StateID.String())
		e.Str("BlockSignature", hex.EncodeToString(commit.ThresholdBlockSignature))
		e.Str("StateSignature", hex.EncodeToString(commit.ThresholdStateSignature))
	}
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
	c.ThresholdVoteExtensions = ThresholdExtensionSignToProto(commit.ThresholdVoteExtensions)
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
	commit.ThresholdVoteExtensions = ThresholdExtensionSignFromProto(cp.ThresholdVoteExtensions)

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
		return fmt.Errorf("wrong Hash: %w", err)
	}
	if err := blockID.PartSetHeader.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong PartSetHeader: %w", err)
	}
	return nil
}

// IsNil returns true if this is the BlockID of a nil block.
func (blockID BlockID) IsNil() bool {
	return len(blockID.Hash) == 0 &&
		blockID.PartSetHeader.IsZero()
}

// IsComplete returns true if this is a valid BlockID of a non-nil block.
func (blockID BlockID) IsComplete() bool {
	return len(blockID.Hash) == crypto.HashSize &&
		blockID.PartSetHeader.Total > 0 &&
		len(blockID.PartSetHeader.Hash) == crypto.HashSize
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
