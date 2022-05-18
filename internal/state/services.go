package state

import (
	"context"

	"github.com/tendermint/tendermint/types"
)

//------------------------------------------------------
// blockchain services types
// NOTE: Interfaces used by RPC must be thread safe!
//------------------------------------------------------

//go:generate ../../scripts/mockery_generate.sh BlockStore

//------------------------------------------------------
// blockstore

// BlockStore defines the interface used by the ConsensusState.
type BlockStore interface {
	Base() int64
	Height() int64
	Size() int64

	LoadBaseMeta() *types.BlockMeta
	LoadBlockMeta(height int64) *types.BlockMeta
	LoadBlock(height int64) *types.Block

	SaveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit)
	SaveBlockWithExtendedCommit(block *types.Block, blockParts *types.PartSet, seenCommit *types.ExtendedCommit)

	PruneBlocks(height int64) (uint64, error)

	LoadBlockByHash(hash []byte) *types.Block
	LoadBlockMetaByHash(hash []byte) *types.BlockMeta
	LoadBlockPart(height int64, index int) *types.Part

	LoadBlockCommit(height int64) *types.Commit
	LoadSeenCommit() *types.Commit
	LoadBlockExtendedCommit(height int64) *types.ExtendedCommit
}

//-----------------------------------------------------------------------------
// evidence pool

//go:generate ../../scripts/mockery_generate.sh EvidencePool

// EvidencePool defines the EvidencePool interface used by State.
type EvidencePool interface {
	PendingEvidence(maxBytes int64) (ev []types.Evidence, size int64)
	AddEvidence(context.Context, types.Evidence) error
	Update(context.Context, State, types.EvidenceList)
	CheckEvidence(context.Context, types.EvidenceList) error
}

// EmptyEvidencePool is an empty implementation of EvidencePool, useful for testing. It also complies
// to the consensus evidence pool interface
type EmptyEvidencePool struct{}

func (EmptyEvidencePool) PendingEvidence(maxBytes int64) (ev []types.Evidence, size int64) {
	return nil, 0
}
func (EmptyEvidencePool) AddEvidence(context.Context, types.Evidence) error { return nil }
func (EmptyEvidencePool) Update(context.Context, State, types.EvidenceList) {}
func (EmptyEvidencePool) CheckEvidence(ctx context.Context, evList types.EvidenceList) error {
	return nil
}
func (EmptyEvidencePool) ReportConflictingVotes(voteA, voteB *types.Vote) {}
