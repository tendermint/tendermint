package types

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	gogotypes "github.com/gogo/protobuf/types"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/libs/bits"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmmath "github.com/tendermint/tendermint/libs/math"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

const (
	// MaxHeaderBytes is a maximum header size.
	// NOTE: Because app hash can be of arbitrary size, the header is therefore not
	// capped in size and thus this number should be seen as a soft max
	MaxHeaderBytes int64 = 626

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

	Header     `json:"header"`
	Data       `json:"data"`
	Evidence   EvidenceList `json:"evidence"`
	LastCommit *Commit      `json:"last_commit"`
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

	// Validate the last commit and its hash.
	if b.LastCommit == nil {
		return errors.New("nil LastCommit")
	}
	if err := b.LastCommit.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong LastCommit: %w", err)
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
%s}#%v`,
		indent, b.Header.StringIndented(indent+"  "),
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
func MaxDataBytes(maxBytes, evidenceBytes int64, valsCount int) int64 {
	maxDataBytes := maxBytes -
		MaxOverheadForBlock -
		MaxHeaderBytes -
		MaxCommitBytes(valsCount) -
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
func MaxDataBytesNoEvidence(maxBytes int64, valsCount int) int64 {
	maxDataBytes := maxBytes -
		MaxOverheadForBlock -
		MaxHeaderBytes -
		MaxCommitBytes(valsCount)

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
func MakeBlock(height int64, txs []Tx, lastCommit *Commit, evidence []Evidence) *Block {
	block := &Block{
		Header: Header{
			Version: version.Consensus{Block: version.BlockProtocol, App: 0},
			Height:  height,
		},
		Data: Data{
			Txs: txs,
		},
		Evidence:   evidence,
		LastCommit: lastCommit,
	}
	block.fillHeader()
	return block
}

//-----------------------------------------------------------------------------

// Header defines the structure of a Tendermint block header.
// NOTE: changes to the Header should be duplicated in:
// - header.Hash()
// - abci.Header
// - https://github.com/tendermint/tendermint/blob/master/spec/core/data_structures.md
type Header struct {
	// basic block info
	Version version.Consensus `json:"version"`
	ChainID string            `json:"chain_id"`
	Height  int64             `json:"height,string"`
	Time    time.Time         `json:"time"`

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
	EvidenceHash    tmbytes.HexBytes `json:"evidence_hash"`    // evidence included in the block
	ProposerAddress Address          `json:"proposer_address"` // original proposer of the block
}

// Populate the Header with state-derived data.
// Call this after MakeBlock to complete the Header.
func (h *Header) Populate(
	version version.Consensus, chainID string,
	timestamp time.Time, lastBlockID BlockID,
	valHash, nextValHash []byte,
	consensusHash, appHash, lastResultsHash []byte,
	proposerAddress Address,
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
	h.ProposerAddress = proposerAddress
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

	if len(h.ProposerAddress) != crypto.AddressSize {
		return fmt.Errorf(
			"invalid ProposerAddress length; got: %d, expected: %d",
			len(h.ProposerAddress), crypto.AddressSize,
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
		cdcEncode(h.ProposerAddress),
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
%s  Time:           %v
%s  LastBlockID:    %v
%s  LastCommit:     %v
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
		indent, h.ProposerAddress,
		indent, h.Hash(),
	)
}

// ToProto converts Header to protobuf
func (h *Header) ToProto() *tmproto.Header {
	if h == nil {
		return nil
	}

	return &tmproto.Header{
		Version:            h.Version.ToProto(),
		ChainID:            h.ChainID,
		Height:             h.Height,
		Time:               h.Time,
		LastBlockId:        h.LastBlockID.ToProto(),
		ValidatorsHash:     h.ValidatorsHash,
		NextValidatorsHash: h.NextValidatorsHash,
		ConsensusHash:      h.ConsensusHash,
		AppHash:            h.AppHash,
		DataHash:           h.DataHash,
		EvidenceHash:       h.EvidenceHash,
		LastResultsHash:    h.LastResultsHash,
		LastCommitHash:     h.LastCommitHash,
		ProposerAddress:    h.ProposerAddress,
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
	h.ProposerAddress = ph.ProposerAddress

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
	// Max size of commit without any commitSigs -> 82 for BlockID, 8 for Height, 4 for Round.
	MaxCommitOverheadBytes int64 = 94
	// Commit sig size is made up of 64 bytes for the signature, 20 bytes for the address,
	// 1 byte for the flag and 14 bytes for the timestamp
	MaxCommitSigBytes int64 = 109
)

// CommitSig is a part of the Vote included in a Commit.
type CommitSig struct {
	BlockIDFlag      BlockIDFlag `json:"block_id_flag"`
	ValidatorAddress Address     `json:"validator_address"`
	Timestamp        time.Time   `json:"timestamp"`
	Signature        []byte      `json:"signature"`
}

func MaxCommitBytes(valCount int) int64 {
	// From the repeated commit sig field
	var protoEncodingOverhead int64 = 2
	return MaxCommitOverheadBytes + ((MaxCommitSigBytes + protoEncodingOverhead) * int64(valCount))
}

// NewCommitSigAbsent returns new CommitSig with BlockIDFlagAbsent. Other
// fields are all empty.
func NewCommitSigAbsent() CommitSig {
	return CommitSig{
		BlockIDFlag: BlockIDFlagAbsent,
	}
}

// CommitSig returns a string representation of CommitSig.
//
// 1. first 6 bytes of signature
// 2. first 6 bytes of validator address
// 3. block ID flag
// 4. timestamp
func (cs CommitSig) String() string {
	return fmt.Sprintf("CommitSig{%X by %X on %v @ %s}",
		tmbytes.Fingerprint(cs.Signature),
		tmbytes.Fingerprint(cs.ValidatorAddress),
		cs.BlockIDFlag,
		CanonicalTime(cs.Timestamp))
}

// BlockID returns the Commit's BlockID if CommitSig indicates signing,
// otherwise - empty BlockID.
func (cs CommitSig) BlockID(commitBlockID BlockID) BlockID {
	var blockID BlockID
	switch cs.BlockIDFlag {
	case BlockIDFlagAbsent:
		blockID = BlockID{}
	case BlockIDFlagCommit:
		blockID = commitBlockID
	case BlockIDFlagNil:
		blockID = BlockID{}
	default:
		panic(fmt.Sprintf("Unknown BlockIDFlag: %v", cs.BlockIDFlag))
	}
	return blockID
}

// ValidateBasic performs basic validation.
func (cs CommitSig) ValidateBasic() error {
	switch cs.BlockIDFlag {
	case BlockIDFlagAbsent:
	case BlockIDFlagCommit:
	case BlockIDFlagNil:
	default:
		return fmt.Errorf("unknown BlockIDFlag: %v", cs.BlockIDFlag)
	}

	switch cs.BlockIDFlag {
	case BlockIDFlagAbsent:
		if len(cs.ValidatorAddress) != 0 {
			return errors.New("validator address is present")
		}
		if !cs.Timestamp.IsZero() {
			return errors.New("time is present")
		}
		if len(cs.Signature) != 0 {
			return errors.New("signature is present")
		}
	default:
		if len(cs.ValidatorAddress) != crypto.AddressSize {
			return fmt.Errorf("expected ValidatorAddress size to be %d bytes, got %d bytes",
				crypto.AddressSize,
				len(cs.ValidatorAddress),
			)
		}
		// NOTE: Timestamp validation is subtle and handled elsewhere.
		if len(cs.Signature) == 0 {
			return errors.New("signature is missing")
		}
		if len(cs.Signature) > MaxSignatureSize {
			return fmt.Errorf("signature is too big (max: %d)", MaxSignatureSize)
		}
	}

	return nil
}

// ToProto converts CommitSig to protobuf
func (cs *CommitSig) ToProto() *tmproto.CommitSig {
	if cs == nil {
		return nil
	}

	return &tmproto.CommitSig{
		BlockIdFlag:      tmproto.BlockIDFlag(cs.BlockIDFlag),
		ValidatorAddress: cs.ValidatorAddress,
		Timestamp:        cs.Timestamp,
		Signature:        cs.Signature,
	}
}

// FromProto sets a protobuf CommitSig to the given pointer.
// It returns an error if the CommitSig is invalid.
func (cs *CommitSig) FromProto(csp tmproto.CommitSig) error {
	cs.BlockIDFlag = BlockIDFlag(csp.BlockIdFlag)
	cs.ValidatorAddress = csp.ValidatorAddress
	cs.Timestamp = csp.Timestamp
	cs.Signature = csp.Signature

	return cs.ValidateBasic()
}

//-------------------------------------

// ExtendedCommitSig contains a commit signature along with its corresponding
// vote extension and vote extension signature.
type ExtendedCommitSig struct {
	CommitSig                 // Commit signature
	Extension          []byte // Vote extension
	ExtensionSignature []byte // Vote extension signature
}

// NewExtendedCommitSigAbsent returns new ExtendedCommitSig with
// BlockIDFlagAbsent. Other fields are all empty.
func NewExtendedCommitSigAbsent() ExtendedCommitSig {
	return ExtendedCommitSig{CommitSig: NewCommitSigAbsent()}
}

// String returns a string representation of an ExtendedCommitSig.
//
// 1. commit sig
// 2. first 6 bytes of vote extension
// 3. first 6 bytes of vote extension signature
func (ecs ExtendedCommitSig) String() string {
	return fmt.Sprintf("ExtendedCommitSig{%s with %X %X}",
		ecs.CommitSig,
		tmbytes.Fingerprint(ecs.Extension),
		tmbytes.Fingerprint(ecs.ExtensionSignature),
	)
}

// ValidateBasic checks whether the structure is well-formed.
func (ecs ExtendedCommitSig) ValidateBasic() error {
	if err := ecs.CommitSig.ValidateBasic(); err != nil {
		return err
	}

	if ecs.BlockIDFlag == BlockIDFlagCommit {
		if len(ecs.Extension) > MaxVoteExtensionSize {
			return fmt.Errorf("vote extension is too big (max: %d)", MaxVoteExtensionSize)
		}
		if len(ecs.ExtensionSignature) > MaxSignatureSize {
			return fmt.Errorf("vote extension signature is too big (max: %d)", MaxSignatureSize)
		}
		return nil
	}

	if len(ecs.ExtensionSignature) == 0 && len(ecs.Extension) != 0 {
		return errors.New("vote extension signature absent on vote with extension")
	}
	return nil
}

// EnsureExtensions validates that a vote extensions signature is present for
// this ExtendedCommitSig.
func (ecs ExtendedCommitSig) EnsureExtension() error {
	if ecs.BlockIDFlag == BlockIDFlagCommit && len(ecs.ExtensionSignature) == 0 {
		return errors.New("vote extension data is missing")
	}
	return nil
}

// ToProto converts the ExtendedCommitSig to its Protobuf representation.
func (ecs *ExtendedCommitSig) ToProto() *tmproto.ExtendedCommitSig {
	if ecs == nil {
		return nil
	}

	return &tmproto.ExtendedCommitSig{
		BlockIdFlag:        tmproto.BlockIDFlag(ecs.BlockIDFlag),
		ValidatorAddress:   ecs.ValidatorAddress,
		Timestamp:          ecs.Timestamp,
		Signature:          ecs.Signature,
		Extension:          ecs.Extension,
		ExtensionSignature: ecs.ExtensionSignature,
	}
}

// FromProto populates the ExtendedCommitSig with values from the given
// Protobuf representation. Returns an error if the ExtendedCommitSig is
// invalid.
func (ecs *ExtendedCommitSig) FromProto(ecsp tmproto.ExtendedCommitSig) error {
	ecs.BlockIDFlag = BlockIDFlag(ecsp.BlockIdFlag)
	ecs.ValidatorAddress = ecsp.ValidatorAddress
	ecs.Timestamp = ecsp.Timestamp
	ecs.Signature = ecsp.Signature
	ecs.Extension = ecsp.Extension
	ecs.ExtensionSignature = ecsp.ExtensionSignature

	return ecs.ValidateBasic()
}

//-------------------------------------

// Commit contains the evidence that a block was committed by a set of validators.
// NOTE: Commit is empty for height 1, but never nil.
type Commit struct {
	// NOTE: The signatures are in order of address to preserve the bonded
	// ValidatorSet order.
	// Any peer with a block can gossip signatures by index with a peer without
	// recalculating the active ValidatorSet.
	Height     int64       `json:"height,string"`
	Round      int32       `json:"round"`
	BlockID    BlockID     `json:"block_id"`
	Signatures []CommitSig `json:"signatures"`

	// Memoized in first call to corresponding method.
	// NOTE: can't memoize in constructor because constructor isn't used for
	// unmarshaling.
	hash tmbytes.HexBytes
}

// GetVote converts the CommitSig for the given valIdx to a Vote. Commits do
// not contain vote extensions, so the vote extension and vote extension
// signature will not be present in the returned vote.
// Returns nil if the precommit at valIdx is nil.
// Panics if valIdx >= commit.Size().
func (commit *Commit) GetVote(valIdx int32) *Vote {
	commitSig := commit.Signatures[valIdx]
	return &Vote{
		Type:             tmproto.PrecommitType,
		Height:           commit.Height,
		Round:            commit.Round,
		BlockID:          commitSig.BlockID(commit.BlockID),
		Timestamp:        commitSig.Timestamp,
		ValidatorAddress: commitSig.ValidatorAddress,
		ValidatorIndex:   valIdx,
		Signature:        commitSig.Signature,
	}
}

// VoteSignBytes returns the bytes of the Vote corresponding to valIdx for
// signing.
//
// The only unique part is the Timestamp - all other fields signed over are
// otherwise the same for all validators.
//
// Panics if valIdx >= commit.Size().
//
// See VoteSignBytes
func (commit *Commit) VoteSignBytes(chainID string, valIdx int32) []byte {
	v := commit.GetVote(valIdx).ToProto()
	return VoteSignBytes(chainID, v)
}

// Size returns the number of signatures in the commit.
func (commit *Commit) Size() int {
	if commit == nil {
		return 0
	}
	return len(commit.Signatures)
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

		if len(commit.Signatures) == 0 {
			return errors.New("no signatures in commit")
		}
		for i, commitSig := range commit.Signatures {
			if err := commitSig.ValidateBasic(); err != nil {
				return fmt.Errorf("wrong CommitSig #%d: %v", i, err)
			}
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
		bs := make([][]byte, len(commit.Signatures))
		for i, commitSig := range commit.Signatures {
			pbcs := commitSig.ToProto()
			bz, err := pbcs.Marshal()
			if err != nil {
				panic(err)
			}

			bs[i] = bz
		}
		commit.hash = merkle.HashFromByteSlices(bs)
	}
	return commit.hash
}

// WrappedExtendedCommit wraps a commit as an ExtendedCommit.
// The VoteExtension fields of the resulting value will by nil.
// Wrapping a Commit as an ExtendedCommit is useful when an API
// requires an ExtendedCommit wire type but does not
// need the VoteExtension data.
func (commit *Commit) WrappedExtendedCommit() *ExtendedCommit {
	cs := make([]ExtendedCommitSig, len(commit.Signatures))
	for idx, s := range commit.Signatures {
		cs[idx] = ExtendedCommitSig{
			CommitSig: s,
		}
	}
	return &ExtendedCommit{
		Height:             commit.Height,
		Round:              commit.Round,
		BlockID:            commit.BlockID,
		ExtendedSignatures: cs,
	}
}

// StringIndented returns a string representation of the commit.
func (commit *Commit) StringIndented(indent string) string {
	if commit == nil {
		return "nil-Commit"
	}
	commitSigStrings := make([]string, len(commit.Signatures))
	for i, commitSig := range commit.Signatures {
		commitSigStrings[i] = commitSig.String()
	}
	return fmt.Sprintf(`Commit{
%s  Height:     %d
%s  Round:      %d
%s  BlockID:    %v
%s  Signatures:
%s    %v
%s}#%v`,
		indent, commit.Height,
		indent, commit.Round,
		indent, commit.BlockID,
		indent,
		indent, strings.Join(commitSigStrings, "\n"+indent+"    "),
		indent, commit.hash)
}

// ToProto converts Commit to protobuf
func (commit *Commit) ToProto() *tmproto.Commit {
	if commit == nil {
		return nil
	}

	c := new(tmproto.Commit)
	sigs := make([]tmproto.CommitSig, len(commit.Signatures))
	for i := range commit.Signatures {
		sigs[i] = *commit.Signatures[i].ToProto()
	}
	c.Signatures = sigs

	c.Height = commit.Height
	c.Round = commit.Round
	c.BlockID = commit.BlockID.ToProto()

	return c
}

// FromProto sets a protobuf Commit to the given pointer.
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

	sigs := make([]CommitSig, len(cp.Signatures))
	for i := range cp.Signatures {
		if err := sigs[i].FromProto(cp.Signatures[i]); err != nil {
			return nil, err
		}
	}
	commit.Signatures = sigs

	commit.Height = cp.Height
	commit.Round = cp.Round
	commit.BlockID = *bi

	return commit, commit.ValidateBasic()
}

//-------------------------------------

// ExtendedCommit is similar to Commit, except that its signatures also retain
// their corresponding vote extensions and vote extension signatures.
type ExtendedCommit struct {
	Height             int64
	Round              int32
	BlockID            BlockID
	ExtendedSignatures []ExtendedCommitSig

	bitArray *bits.BitArray
}

// Clone creates a deep copy of this extended commit.
func (ec *ExtendedCommit) Clone() *ExtendedCommit {
	sigs := make([]ExtendedCommitSig, len(ec.ExtendedSignatures))
	copy(sigs, ec.ExtendedSignatures)
	ecc := *ec
	ecc.ExtendedSignatures = sigs
	return &ecc
}

// ToExtendedVoteSet constructs a VoteSet from the Commit and validator set.
// Panics if signatures from the ExtendedCommit can't be added to the voteset.
// Panics if any of the votes have invalid or absent vote extension data.
// Inverse of VoteSet.MakeExtendedCommit().
func (ec *ExtendedCommit) ToExtendedVoteSet(chainID string, vals *ValidatorSet) *VoteSet {
	voteSet := NewExtendedVoteSet(chainID, ec.Height, ec.Round, tmproto.PrecommitType, vals)
	ec.addSigsToVoteSet(voteSet)
	return voteSet
}

// ToVoteSet constructs a VoteSet from the Commit and validator set.
// Panics if signatures from the ExtendedCommit can't be added to the voteset.
// Inverse of VoteSet.MakeExtendedCommit().
func (ec *ExtendedCommit) ToVoteSet(chainID string, vals *ValidatorSet) *VoteSet {
	voteSet := NewVoteSet(chainID, ec.Height, ec.Round, tmproto.PrecommitType, vals)
	ec.addSigsToVoteSet(voteSet)
	return voteSet
}

// addSigsToVoteSet adds all of the signature to voteSet.
func (ec *ExtendedCommit) addSigsToVoteSet(voteSet *VoteSet) {
	for idx, ecs := range ec.ExtendedSignatures {
		if ecs.BlockIDFlag == BlockIDFlagAbsent {
			continue // OK, some precommits can be missing.
		}
		vote := ec.GetExtendedVote(int32(idx))
		if err := vote.ValidateBasic(); err != nil {
			panic(fmt.Errorf("failed to validate vote reconstructed from LastCommit: %w", err))
		}
		added, err := voteSet.AddVote(vote)
		if !added || err != nil {
			panic(fmt.Errorf("failed to reconstruct vote set from extended commit: %w", err))
		}
	}
}

// ToVoteSet constructs a VoteSet from the Commit and validator set.
// Panics if signatures from the commit can't be added to the voteset.
// Inverse of VoteSet.MakeCommit().
func (commit *Commit) ToVoteSet(chainID string, vals *ValidatorSet) *VoteSet {
	voteSet := NewVoteSet(chainID, commit.Height, commit.Round, tmproto.PrecommitType, vals)
	for idx, cs := range commit.Signatures {
		if cs.BlockIDFlag == BlockIDFlagAbsent {
			continue // OK, some precommits can be missing.
		}
		vote := commit.GetVote(int32(idx))
		if err := vote.ValidateBasic(); err != nil {
			panic(fmt.Errorf("failed to validate vote reconstructed from commit: %w", err))
		}
		added, err := voteSet.AddVote(vote)
		if !added || err != nil {
			panic(fmt.Errorf("failed to reconstruct vote set from commit: %w", err))
		}
	}
	return voteSet
}

// EnsureExtensions validates that a vote extensions signature is present for
// every ExtendedCommitSig in the ExtendedCommit.
func (ec *ExtendedCommit) EnsureExtensions() error {
	for _, ecs := range ec.ExtendedSignatures {
		if err := ecs.EnsureExtension(); err != nil {
			return err
		}
	}
	return nil
}

// StripExtensions removes all VoteExtension data from an ExtendedCommit. This
// is useful when dealing with an ExendedCommit but vote extension data is
// expected to be absent.
func (ec *ExtendedCommit) StripExtensions() bool {
	stripped := false
	for idx := range ec.ExtendedSignatures {
		if len(ec.ExtendedSignatures[idx].Extension) > 0 || len(ec.ExtendedSignatures[idx].ExtensionSignature) > 0 {
			stripped = true
		}
		ec.ExtendedSignatures[idx].Extension = nil
		ec.ExtendedSignatures[idx].ExtensionSignature = nil
	}
	return stripped
}

// ToCommit converts an ExtendedCommit to a Commit by removing all vote
// extension-related fields.
func (ec *ExtendedCommit) ToCommit() *Commit {
	cs := make([]CommitSig, len(ec.ExtendedSignatures))
	for idx, ecs := range ec.ExtendedSignatures {
		cs[idx] = ecs.CommitSig
	}
	return &Commit{
		Height:     ec.Height,
		Round:      ec.Round,
		BlockID:    ec.BlockID,
		Signatures: cs,
	}
}

// GetExtendedVote converts the ExtendedCommitSig for the given validator
// index to a Vote with a vote extensions.
// It panics if valIndex is out of range.
func (ec *ExtendedCommit) GetExtendedVote(valIndex int32) *Vote {
	ecs := ec.ExtendedSignatures[valIndex]
	return &Vote{
		Type:               tmproto.PrecommitType,
		Height:             ec.Height,
		Round:              ec.Round,
		BlockID:            ecs.BlockID(ec.BlockID),
		Timestamp:          ecs.Timestamp,
		ValidatorAddress:   ecs.ValidatorAddress,
		ValidatorIndex:     valIndex,
		Signature:          ecs.Signature,
		Extension:          ecs.Extension,
		ExtensionSignature: ecs.ExtensionSignature,
	}
}

// Type returns the vote type of the extended commit, which is always
// VoteTypePrecommit
// Implements VoteSetReader.
func (ec *ExtendedCommit) Type() byte { return byte(tmproto.PrecommitType) }

// GetHeight returns height of the extended commit.
// Implements VoteSetReader.
func (ec *ExtendedCommit) GetHeight() int64 { return ec.Height }

// GetRound returns height of the extended commit.
// Implements VoteSetReader.
func (ec *ExtendedCommit) GetRound() int32 { return ec.Round }

// Size returns the number of signatures in the extended commit.
// Implements VoteSetReader.
func (ec *ExtendedCommit) Size() int {
	if ec == nil {
		return 0
	}
	return len(ec.ExtendedSignatures)
}

// BitArray returns a BitArray of which validators voted for BlockID or nil in
// this extended commit.
// Implements VoteSetReader.
func (ec *ExtendedCommit) BitArray() *bits.BitArray {
	if ec.bitArray == nil {
		ec.bitArray = bits.NewBitArray(len(ec.ExtendedSignatures))
		for i, extCommitSig := range ec.ExtendedSignatures {
			// TODO: need to check the BlockID otherwise we could be counting conflicts,
			//       not just the one with +2/3 !
			ec.bitArray.SetIndex(i, extCommitSig.BlockIDFlag != BlockIDFlagAbsent)
		}
	}
	return ec.bitArray
}

// GetByIndex returns the vote corresponding to a given validator index.
// Panics if `index >= extCommit.Size()`.
// Implements VoteSetReader.
func (ec *ExtendedCommit) GetByIndex(valIdx int32) *Vote {
	return ec.GetExtendedVote(valIdx)
}

// IsCommit returns true if there is at least one signature.
// Implements VoteSetReader.
func (ec *ExtendedCommit) IsCommit() bool {
	return len(ec.ExtendedSignatures) != 0
}

// ValidateBasic checks whether the extended commit is well-formed. Does not
// actually check the cryptographic signatures.
func (ec *ExtendedCommit) ValidateBasic() error {
	if ec.Height < 0 {
		return errors.New("negative Height")
	}
	if ec.Round < 0 {
		return errors.New("negative Round")
	}

	if ec.Height >= 1 {
		if ec.BlockID.IsNil() {
			return errors.New("commit cannot be for nil block")
		}

		if len(ec.ExtendedSignatures) == 0 {
			return errors.New("no signatures in commit")
		}
		for i, extCommitSig := range ec.ExtendedSignatures {
			if err := extCommitSig.ValidateBasic(); err != nil {
				return fmt.Errorf("wrong ExtendedCommitSig #%d: %v", i, err)
			}
		}
	}
	return nil
}

// ToProto converts ExtendedCommit to protobuf
func (ec *ExtendedCommit) ToProto() *tmproto.ExtendedCommit {
	if ec == nil {
		return nil
	}

	c := new(tmproto.ExtendedCommit)
	sigs := make([]tmproto.ExtendedCommitSig, len(ec.ExtendedSignatures))
	for i := range ec.ExtendedSignatures {
		sigs[i] = *ec.ExtendedSignatures[i].ToProto()
	}
	c.ExtendedSignatures = sigs

	c.Height = ec.Height
	c.Round = ec.Round
	c.BlockID = ec.BlockID.ToProto()

	return c
}

// ExtendedCommitFromProto constructs an ExtendedCommit from the given Protobuf
// representation. It returns an error if the extended commit is invalid.
func ExtendedCommitFromProto(ecp *tmproto.ExtendedCommit) (*ExtendedCommit, error) {
	if ecp == nil {
		return nil, errors.New("nil ExtendedCommit")
	}

	extCommit := new(ExtendedCommit)

	bi, err := BlockIDFromProto(&ecp.BlockID)
	if err != nil {
		return nil, err
	}

	sigs := make([]ExtendedCommitSig, len(ecp.ExtendedSignatures))
	for i := range ecp.ExtendedSignatures {
		if err := sigs[i].FromProto(ecp.ExtendedSignatures[i]); err != nil {
			return nil, err
		}
	}
	extCommit.ExtendedSignatures = sigs
	extCommit.Height = ecp.Height
	extCommit.Round = ecp.Round
	extCommit.BlockID = *bi

	return extCommit, extCommit.ValidateBasic()
}

//-------------------------------------

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

// ProtoBlockIDIsNil is similar to the IsNil function on BlockID, but for the
// Protobuf representation.
func ProtoBlockIDIsNil(bID *tmproto.BlockID) bool {
	return len(bID.Hash) == 0 && ProtoPartSetHeaderIsZero(&bID.PartSetHeader)
}
