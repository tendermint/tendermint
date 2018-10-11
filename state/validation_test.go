package state

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

// TODO(#2589):
// - generalize this past the first height
// - add txs and build up full State properly
// - test block.Time (see #2587 - there are no conditions on time for the first height)
func TestValidateBlockHeader(t *testing.T) {
	var height int64 = 1 // TODO(#2589): generalize
	state, stateDB := state(1, int(height))

	blockExec := NewBlockExecutor(stateDB, log.TestingLogger(), nil, nil, nil)

	// A good block passes.
	block := makeBlock(state, height)
	err := blockExec.ValidateBlock(state, block)
	require.NoError(t, err)

	wrongHash := tmhash.Sum([]byte("this hash is wrong"))

	// Manipulation of any header field causes failure.
	testCases := []struct {
		name          string
		malleateBlock func(block *types.Block)
	}{
		{"ChainID wrong", func(block *types.Block) { block.ChainID = "not-the-real-one" }}, // wrong chain id
		{"Height wrong", func(block *types.Block) { block.Height += 10 }},                  // wrong height
		// TODO(#2589) (#2587) : {"Time", func(block *types.Block) { block.Time.Add(-time.Second * 3600 * 24) }}, // wrong time
		{"NumTxs wrong", func(block *types.Block) { block.NumTxs += 10 }},     // wrong num txs
		{"TotalTxs wrong", func(block *types.Block) { block.TotalTxs += 10 }}, // wrong total txs

		{"LastBlockID wrong", func(block *types.Block) { block.LastBlockID.PartsHeader.Total += 10 }},
		{"LastCommitHash wrong", func(block *types.Block) { block.LastCommitHash = wrongHash }},
		{"DataHash wrong", func(block *types.Block) { block.DataHash = wrongHash }},

		{"ValidatorsHash wrong", func(block *types.Block) { block.ValidatorsHash = wrongHash }},
		{"NextValidatorsHash wrong", func(block *types.Block) { block.NextValidatorsHash = wrongHash }},
		{"ConsensusHash wrong", func(block *types.Block) { block.ConsensusHash = wrongHash }},
		{"AppHash wrong", func(block *types.Block) { block.AppHash = wrongHash }},
		{"LastResultsHash wrong", func(block *types.Block) { block.LastResultsHash = wrongHash }},

		{"EvidenceHash wrong", func(block *types.Block) { block.EvidenceHash = wrongHash }},
		{"Proposer wrong", func(block *types.Block) { block.ProposerAddress = ed25519.GenPrivKey().PubKey().Address() }},
		{"Proposer invalid", func(block *types.Block) { block.ProposerAddress = []byte("wrong size") }},
	}

	for _, tc := range testCases {
		block := makeBlock(state, height)
		tc.malleateBlock(block)
		err := blockExec.ValidateBlock(state, block)
		require.Error(t, err, tc.name)
	}
}

/*
	TODO(#2589):
	- test Block.Data.Hash() == Block.DataHash
	- test len(Block.Data.Txs) == Block.NumTxs
*/
func TestValidateBlockData(t *testing.T) {
}

/*
	TODO(#2589):
	- test len(block.LastCommit.Precommits) == state.LastValidators.Size()
	- test state.LastValidators.VerifyCommit
*/
func TestValidateBlockCommit(t *testing.T) {
}

/*
	TODO(#2589):
	- test good/bad evidence in block
*/
func TestValidateBlockEvidence(t *testing.T) {
	var height int64 = 1 // TODO(#2589): generalize
	state, stateDB := state(1, int(height))

	blockExec := NewBlockExecutor(stateDB, log.TestingLogger(), nil, nil, nil)

	// make some evidence
	addr, _ := state.Validators.GetByIndex(0)
	goodEvidence := types.NewMockGoodEvidence(height, 0, addr)

	// A block with a couple pieces of evidence passes.
	block := makeBlock(state, height)
	block.Evidence.Evidence = []types.Evidence{goodEvidence, goodEvidence}
	block.EvidenceHash = block.Evidence.Hash()
	err := blockExec.ValidateBlock(state, block)
	require.NoError(t, err)

	// A block with too much evidence fails.
	maxBlockSize := state.ConsensusParams.BlockSize.MaxBytes
	maxEvidenceBytes := types.MaxEvidenceBytesPerBlock(maxBlockSize)
	maxEvidence := maxEvidenceBytes / types.MaxEvidenceBytes
	require.True(t, maxEvidence > 2)
	for i := int64(0); i < maxEvidence; i++ {
		block.Evidence.Evidence = append(block.Evidence.Evidence, goodEvidence)
	}
	block.EvidenceHash = block.Evidence.Hash()
	err = blockExec.ValidateBlock(state, block)
	require.Error(t, err)
	_, ok := err.(*types.ErrEvidenceOverflow)
	require.True(t, ok)
}

/*
	TODO(#2589):
	- test unmarshalling BlockParts that are too big into a Block that
		(note this logic happens in the consensus, not in the validation here).
	- test making blocks from the types.MaxXXX functions works/fails as expected
*/
func TestValidateBlockSize(t *testing.T) {
}
