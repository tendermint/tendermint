package state

import (
	"github.com/tendermint/tendermint/types"
)

//------------------------------------------------------
// blockchain services types
// NOTE: Interfaces used by RPC must be thread safe!
//------------------------------------------------------

//------------------------------------------------------
// blockstore

// BlockStore defines the interface used by the ConsensusState.
type BlockStore interface {
	Base() int64
	Height() int64
	Size() int64

	LoadBlockMeta(height int64) *types.BlockMeta
	LoadBlock(height int64) *types.Block

	SaveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit)

	PruneBlocks(height int64) (uint64, error)

	LoadBlockByHash(hash []byte) *types.Block
	LoadBlockPart(height int64, index int) *types.Part

	LoadBlockCommit(height int64) *types.Commit
	LoadSeenCommit(height int64) *types.Commit
}

//-----------------------------------------------------------------------------
// evidence pool

//go:generate mockery -case underscore -name EvidencePool

// EvidencePool defines the EvidencePool interface used by the ConsensusState.
// Get/Set/Commit
type EvidencePool interface {
	PendingEvidence(int64) []types.Evidence
	AddEvidence(types.Evidence) error
	Update(*types.Block, State)
	IsCommitted(types.Evidence) bool
	IsPending(types.Evidence) bool
}

type MockEvidencePool struct{}

func (me MockEvidencePool) PendingEvidence(int64) []types.Evidence { return nil }
func (me MockEvidencePool) AddEvidence(types.Evidence) error       { return nil }
func (me MockEvidencePool) Update(*types.Block, State)             {}
func (me MockEvidencePool) IsCommitted(types.Evidence) bool        { return false }
func (me MockEvidencePool) IsPending(types.Evidence) bool          { return false }
