package types

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/merkle"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmproto "github.com/tendermint/tendermint/proto/types"
	"github.com/tendermint/tendermint/version"
)

// Header defines the structure of a Tendermint block header.
// NOTE: changes to the Header should be duplicated in:
// - header.Hash()
// - abci.Header
// - https://github.com/tendermint/spec/blob/master/spec/blockchain/blockchain.md
type Header struct {
	// basic block info
	Version version.Consensus `json:"version"`
	ChainID string            `json:"chain_id"`
	Height  int64             `json:"height"`
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
	return merkle.SimpleHashFromByteSlices([][]byte{
		cdcEncode(h.Version),
		cdcEncode(h.ChainID),
		cdcEncode(h.Height),
		cdcEncode(h.Time),
		cdcEncode(h.LastBlockID),
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

// StringIndented returns a string representation of the header
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
		indent, h.Hash())
}

func (h Header) ValidateBasic() error {
	if len(h.ChainID) > MaxChainIDLen {
		return fmt.Errorf("chainID is too long. Max is %d, got %d", MaxChainIDLen, len(h.ChainID))
	}

	if h.Height < 0 {
		return errors.New("negative Header.Height")
	} else if h.Height == 0 {
		return errors.New("zero Header.Height")
	}

	if err := h.LastBlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong Header.LastBlockID: %v", err)
	}
	if err := ValidateHash(h.LastCommitHash); err != nil {
		return fmt.Errorf("wrong Header.LastCommitHash: %v", err)
	}

	// Basic validation of hashes related to application data.
	// Will validate fully against state in state#ValidateBlock.
	if err := ValidateHash(h.ValidatorsHash); err != nil {
		return fmt.Errorf("wrong Header.ValidatorsHash: %v", err)
	}
	if err := ValidateHash(h.NextValidatorsHash); err != nil {
		return fmt.Errorf("wrong Header.NextValidatorsHash: %v", err)
	}
	if err := ValidateHash(h.ConsensusHash); err != nil {
		return fmt.Errorf("wrong Header.ConsensusHash: %v", err)
	}
	// NOTE: AppHash is arbitrary length
	if err := ValidateHash(h.LastResultsHash); err != nil {
		return fmt.Errorf("wrong Header.LastResultsHash: %v", err)
	}

	if err := ValidateHash(h.EvidenceHash); err != nil {
		return fmt.Errorf("wrong Header.EvidenceHash: %v", err)
	}

	if len(h.ProposerAddress) != crypto.AddressSize {
		return fmt.Errorf("expected len(Header.ProposerAddress) to be %d, got %d",
			crypto.AddressSize, len(h.ProposerAddress))
	}

	return nil
}

func (h Header) ToProto() (*tmproto.Header, error) {
	err := h.ValidateBasic()
	if err != nil {
		return nil, err
	}
	ph := tmproto.Header{
		Version: tmproto.Version{
			Block: h.Version.Block.Uint64(),
			App:   h.Version.App.Uint64(),
		},
		ChainId: h.ChainID,
		Time:    h.Time,
		LastBlockId: tmproto.BlockID{
			Hash: h.LastBlockID.Hash,
			PartsHeader: tmproto.PartSetHeader{
				Hash:  h.LastBlockID.PartsHeader.Hash,
				Total: h.LastBlockID.PartsHeader.Total,
			},
		},
		ValidatorsHash:     h.ValidatorsHash,
		NextValidatorsHash: h.NextValidatorsHash,
		ConsensusHash:      h.ConsensusHash,
		AppHash:            h.AppHash,
		LastResultsHash:    h.LastResultsHash,
		ProposerAddress:    h.ProposerAddress,
	}
	return &ph, nil
}

func (h *Header) FromProto(ph tmproto.Header) {
	h.Version = version.Consensus{
		Block: version.Protocol(ph.Version.Block),
		App:   version.Protocol(ph.Version.App),
	}
	h.ChainID = ph.ChainId
	h.Time = ph.Time
	h.LastBlockID = BlockID{
		Hash: ph.LastBlockId.Hash,
		PartsHeader: PartSetHeader{
			Hash:  ph.LastBlockId.PartsHeader.Hash,
			Total: ph.LastBlockId.PartsHeader.Total,
		},
	}
	h.ValidatorsHash = ph.ValidatorsHash
	h.NextValidatorsHash = ph.NextValidatorsHash
	h.ConsensusHash = ph.ConsensusHash
	h.AppHash = ph.AppHash
	h.LastResultsHash = ph.LastResultsHash
	h.ProposerAddress = ph.ProposerAddress
}
