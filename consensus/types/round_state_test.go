package types

import (
	"testing"

	amino "github.com/tendermint/go-amino"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

func BenchmarkRoundStateDeepCopy(b *testing.B) {
	b.StopTimer()

	// Random validators
	nval, ntxs := 100, 100
	vset, _ := types.RandValidatorSet(nval, 1)
	commitSigs := make([]types.CommitSig, nval)
	blockID := types.BlockID{
		Hash: tmrand.Bytes(tmhash.Size),
		PartsHeader: types.PartSetHeader{
			Hash:  tmrand.Bytes(tmhash.Size),
			Total: 1000,
		},
	}
	sig := make([]byte, ed25519.SignatureSize)
	for i := 0; i < nval; i++ {
		commitSigs[i] = (&types.Vote{
			ValidatorAddress: types.Address(tmrand.Bytes(20)),
			Timestamp:        tmtime.Now(),
			BlockID:          blockID,
			Signature:        sig,
		}).CommitSig()
	}
	txs := make([]types.Tx, ntxs)
	for i := 0; i < ntxs; i++ {
		txs[i] = tmrand.Bytes(100)
	}
	// Random block
	block := &types.Block{
		Header: types.Header{
			ChainID:         tmrand.Str(12),
			Time:            tmtime.Now(),
			LastBlockID:     blockID,
			LastCommitHash:  tmrand.Bytes(20),
			DataHash:        tmrand.Bytes(20),
			ValidatorsHash:  tmrand.Bytes(20),
			ConsensusHash:   tmrand.Bytes(20),
			AppHash:         tmrand.Bytes(20),
			LastResultsHash: tmrand.Bytes(20),
			EvidenceHash:    tmrand.Bytes(20),
		},
		Data: types.Data{
			Txs: txs,
		},
		Evidence:   types.EvidenceData{},
		LastCommit: types.NewCommit(1, 0, blockID, commitSigs),
	}
	parts := block.MakePartSet(4096)
	// Random Proposal
	proposal := &types.Proposal{
		Timestamp: tmtime.Now(),
		BlockID:   blockID,
		Signature: sig,
	}
	// Random HeightVoteSet
	// TODO: hvs :=

	rs := &RoundState{
		StartTime:          tmtime.Now(),
		CommitTime:         tmtime.Now(),
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
