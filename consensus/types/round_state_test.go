package types

import (
	"testing"
	"time"

	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tendermint/libs/common"
)

func BenchmarkRoundStateDeepCopy(b *testing.B) {
	b.StopTimer()

	// Random validators
	nval, ntxs := 100, 100
	vset, _ := types.RandValidatorSet(nval, 1)
	precommits := make([]*types.Vote, nval)
	blockID := types.BlockID{
		Hash: cmn.RandBytes(20),
		PartsHeader: types.PartSetHeader{
			Hash: cmn.RandBytes(20),
		},
	}
	sig := crypto.SignatureEd25519{}
	for i := 0; i < nval; i++ {
		precommits[i] = &types.Vote{
			ValidatorAddress: types.Address(cmn.RandBytes(20)),
			Timestamp:        time.Now(),
			BlockID:          blockID,
			Signature:        sig,
		}
	}
	txs := make([]types.Tx, ntxs)
	for i := 0; i < ntxs; i++ {
		txs[i] = cmn.RandBytes(100)
	}
	// Random block
	block := &types.Block{
		Header: &types.Header{
			ChainID:         cmn.RandStr(12),
			Time:            time.Now(),
			LastBlockID:     blockID,
			LastCommitHash:  cmn.RandBytes(20),
			DataHash:        cmn.RandBytes(20),
			ValidatorsHash:  cmn.RandBytes(20),
			ConsensusHash:   cmn.RandBytes(20),
			AppHash:         cmn.RandBytes(20),
			LastResultsHash: cmn.RandBytes(20),
			EvidenceHash:    cmn.RandBytes(20),
		},
		Data: &types.Data{
			Txs: txs,
		},
		Evidence: types.EvidenceData{},
		LastCommit: &types.Commit{
			BlockID:    blockID,
			Precommits: precommits,
		},
	}
	parts := block.MakePartSet(4096)
	// Random Proposal
	proposal := &types.Proposal{
		Timestamp: time.Now(),
		BlockPartsHeader: types.PartSetHeader{
			Hash: cmn.RandBytes(20),
		},
		POLBlockID: blockID,
		Signature:  sig,
	}
	// Random HeightVoteSet
	// TODO: hvs :=

	rs := &RoundState{
		StartTime:          time.Now(),
		CommitTime:         time.Now(),
		Validators:         vset,
		Proposal:           proposal,
		ProposalBlock:      block,
		ProposalBlockParts: parts,
		LockedBlock:        block,
		LockedBlockParts:   parts,
		ValidBlock:         block,
		ValidBlockParts:    parts,
		Votes:              nil, // TODO
		LastCommit:         nil, // TODO
		LastValidators:     vset,
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		amino.DeepCopy(rs)
	}
}
