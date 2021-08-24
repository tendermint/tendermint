package state

import (
	"github.com/tendermint/tendermint/pkg/block"
	"github.com/tendermint/tendermint/pkg/consensus"
	"github.com/tendermint/tendermint/pkg/evidence"
	"github.com/tendermint/tendermint/pkg/metadata"
)

//------------------------------------------------------
// blockchain services types
// NOTE: Interfaces used by RPC must be thread safe!
//------------------------------------------------------

//go:generate ../scripts/mockery_generate.sh BlockStore

//------------------------------------------------------
// blockstore

// BlockStore defines the interface used by the ConsensusState.
type BlockStore interface {
	Base() int64
	Height() int64
	Size() int64

	LoadBaseMeta() *block.BlockMeta
	LoadBlockMeta(height int64) *block.BlockMeta
	LoadBlock(height int64) *block.Block

	SaveBlock(block *block.Block, blockParts *metadata.PartSet, seenCommit *metadata.Commit)

	PruneBlocks(height int64) (uint64, error)

	LoadBlockByHash(hash []byte) *block.Block
	LoadBlockPart(height int64, index int) *metadata.Part

	LoadBlockCommit(height int64) *metadata.Commit
	LoadSeenCommit() *metadata.Commit
}

//-----------------------------------------------------------------------------
// evidence pool

//go:generate ../scripts/mockery_generate.sh EvidencePool

// EvidencePool defines the EvidencePool interface used by State.
type EvidencePool interface {
	PendingEvidence(maxBytes int64) (ev []evidence.Evidence, size int64)
	AddEvidence(evidence.Evidence) error
	Update(State, evidence.EvidenceList)
	CheckEvidence(evidence.EvidenceList) error
}

// EmptyEvidencePool is an empty implementation of EvidencePool, useful for testing. It also complies
// to the consensus evidence pool interface
type EmptyEvidencePool struct{}

func (EmptyEvidencePool) PendingEvidence(maxBytes int64) (ev []evidence.Evidence, size int64) {
	return nil, 0
}
func (EmptyEvidencePool) AddEvidence(evidence.Evidence) error                 { return nil }
func (EmptyEvidencePool) Update(State, evidence.EvidenceList)                 {}
func (EmptyEvidencePool) CheckEvidence(evList evidence.EvidenceList) error    { return nil }
func (EmptyEvidencePool) ReportConflictingVotes(voteA, voteB *consensus.Vote) {}
