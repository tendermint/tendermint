package state_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/mock"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/libs/log"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

const validationTestsStopHeight int64 = 10

func TestValidateBlockHeader(t *testing.T) {
	proxyApp := newTestApp()
	require.NoError(t, proxyApp.Start())
	defer proxyApp.Stop()

	state, stateDB, privVals := makeState(3, 1)
	blockExec := sm.NewBlockExecutor(stateDB, log.TestingLogger(), proxyApp.Consensus(), mock.Mempool{}, sm.MockEvidencePool{})
	lastCommit := types.NewCommit(types.BlockID{}, nil)

	// some bad values
	wrongHash := tmhash.Sum([]byte("this hash is wrong"))
	wrongVersion1 := state.Version.Consensus
	wrongVersion1.Block += 1
	wrongVersion2 := state.Version.Consensus
	wrongVersion2.App += 1

	// Manipulation of any header field causes failure.
	testCases := []struct {
		name          string
		malleateBlock func(block *types.Block)
	}{
		{"Version wrong1", func(block *types.Block) { block.Version = wrongVersion1 }},
		{"Version wrong2", func(block *types.Block) { block.Version = wrongVersion2 }},
		{"ChainID wrong", func(block *types.Block) { block.ChainID = "not-the-real-one" }},
		{"Height wrong", func(block *types.Block) { block.Height += 10 }},
		{"Time wrong", func(block *types.Block) { block.Time = block.Time.Add(-time.Second * 1) }},
		{"NumTxs wrong", func(block *types.Block) { block.NumTxs += 10 }},
		{"TotalTxs wrong", func(block *types.Block) { block.TotalTxs += 10 }},

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

	// Build up state for multiple heights
	for height := int64(1); height < validationTestsStopHeight; height++ {
		proposerAddr := state.Validators.GetProposer().Address
		/*
			Invalid blocks don't pass
		*/
		for _, tc := range testCases {
			block, _ := state.MakeBlock(height, makeTxs(height), lastCommit, nil, proposerAddr)
			tc.malleateBlock(block)
			err := blockExec.ValidateBlock(state, block)
			require.Error(t, err, tc.name)
		}

		/*
			A good block passes
		*/
		var err error
		state, _, lastCommit, err = makeAndCommitGoodBlock(state, height, lastCommit, proposerAddr, blockExec, privVals, nil)
		require.NoError(t, err, "height %d", height)
	}
}

func TestValidateBlockCommit(t *testing.T) {
	proxyApp := newTestApp()
	require.NoError(t, proxyApp.Start())
	defer proxyApp.Stop()

	state, stateDB, privVals := makeState(1, 1)
	blockExec := sm.NewBlockExecutor(stateDB, log.TestingLogger(), proxyApp.Consensus(), mock.Mempool{}, sm.MockEvidencePool{})
	lastCommit := types.NewCommit(types.BlockID{}, nil)
	wrongPrecommitsCommit := types.NewCommit(types.BlockID{}, nil)
	badPrivVal := types.NewMockPV()

	for height := int64(1); height < validationTestsStopHeight; height++ {
		proposerAddr := state.Validators.GetProposer().Address
		if height > 1 {
			/*
				#2589: ensure state.LastValidators.VerifyCommit fails here
			*/
			// should be height-1 instead of height
			wrongHeightVote, err := types.MakeVote(height, state.LastBlockID, state.Validators, privVals[proposerAddr.String()], chainID)
			require.NoError(t, err, "height %d", height)
			wrongHeightCommit := types.NewCommit(state.LastBlockID, []*types.CommitSig{wrongHeightVote.CommitSig()})
			block, _ := state.MakeBlock(height, makeTxs(height), wrongHeightCommit, nil, proposerAddr)
			err = blockExec.ValidateBlock(state, block)
			_, isErrInvalidCommitHeight := err.(types.ErrInvalidCommitHeight)
			require.True(t, isErrInvalidCommitHeight, "expected ErrInvalidCommitHeight at height %d but got: %v", height, err)

			/*
				#2589: test len(block.LastCommit.Precommits) == state.LastValidators.Size()
			*/
			block, _ = state.MakeBlock(height, makeTxs(height), wrongPrecommitsCommit, nil, proposerAddr)
			err = blockExec.ValidateBlock(state, block)
			_, isErrInvalidCommitPrecommits := err.(types.ErrInvalidCommitPrecommits)
			require.True(t, isErrInvalidCommitPrecommits, "expected ErrInvalidCommitPrecommits at height %d but got: %v", height, err)
		}

		/*
			A good block passes
		*/
		var err error
		var blockID types.BlockID
		state, blockID, lastCommit, err = makeAndCommitGoodBlock(state, height, lastCommit, proposerAddr, blockExec, privVals, nil)
		require.NoError(t, err, "height %d", height)

		/*
			wrongPrecommitsCommit is fine except for the extra bad precommit
		*/
		goodVote, err := types.MakeVote(height, blockID, state.Validators, privVals[proposerAddr.String()], chainID)
		require.NoError(t, err, "height %d", height)
		badVote := &types.Vote{
			ValidatorAddress: badPrivVal.GetPubKey().Address(),
			ValidatorIndex:   0,
			Height:           height,
			Round:            0,
			Timestamp:        tmtime.Now(),
			Type:             types.PrecommitType,
			BlockID:          blockID,
		}
		err = badPrivVal.SignVote(chainID, goodVote)
		require.NoError(t, err, "height %d", height)
		wrongPrecommitsCommit = types.NewCommit(blockID, []*types.CommitSig{goodVote.CommitSig(), badVote.CommitSig()})
	}
}

func TestValidateBlockEvidence(t *testing.T) {
	proxyApp := newTestApp()
	require.NoError(t, proxyApp.Start())
	defer proxyApp.Stop()

	state, stateDB, privVals := makeState(3, 1)
	blockExec := sm.NewBlockExecutor(stateDB, log.TestingLogger(), proxyApp.Consensus(), mock.Mempool{}, sm.MockEvidencePool{})
	lastCommit := types.NewCommit(types.BlockID{}, nil)

	for height := int64(1); height < validationTestsStopHeight; height++ {
		proposerAddr := state.Validators.GetProposer().Address
		proposerIdx, _ := state.Validators.GetByAddress(proposerAddr)
		goodEvidence := types.NewMockGoodEvidence(height, proposerIdx, proposerAddr)
		if height > 1 {
			/*
				A block with too much evidence fails
			*/
			maxBlockSize := state.ConsensusParams.Block.MaxBytes
			maxNumEvidence, _ := types.MaxEvidencePerBlock(maxBlockSize)
			require.True(t, maxNumEvidence > 2)
			evidence := make([]types.Evidence, 0)
			// one more than the maximum allowed evidence
			for i := int64(0); i <= maxNumEvidence; i++ {
				evidence = append(evidence, goodEvidence)
			}
			block, _ := state.MakeBlock(height, makeTxs(height), lastCommit, evidence, proposerAddr)
			err := blockExec.ValidateBlock(state, block)
			_, ok := err.(*types.ErrEvidenceOverflow)
			require.True(t, ok, "expected error to be of type ErrEvidenceOverflow at height %d", height)
		}

		/*
			A good block with several pieces of good evidence passes
		*/
		maxBlockSize := state.ConsensusParams.Block.MaxBytes
		maxNumEvidence, _ := types.MaxEvidencePerBlock(maxBlockSize)
		require.True(t, maxNumEvidence > 2)
		evidence := make([]types.Evidence, 0)
		// precisely the amount of allowed evidence
		for i := int64(0); i < maxNumEvidence; i++ {
			evidence = append(evidence, goodEvidence)
		}

		var err error
		state, _, lastCommit, err = makeAndCommitGoodBlock(state, height, lastCommit, proposerAddr, blockExec, privVals, evidence)
		require.NoError(t, err, "height %d", height)
	}
}

func TestValidateFailBlockOnCommittedEvidence(t *testing.T) {
	var height int64 = 1
	state, stateDB, _ := makeState(1, int(height))

	blockExec := sm.NewBlockExecutor(stateDB, log.TestingLogger(), nil, nil, mockEvPoolAlwaysCommitted{})
	// A block with a couple pieces of evidence passes.
	block := makeBlock(state, height)
	addr, _ := state.Validators.GetByIndex(0)
	alreadyCommittedEvidence := types.NewMockGoodEvidence(height, 0, addr)
	block.Evidence.Evidence = []types.Evidence{alreadyCommittedEvidence}
	block.EvidenceHash = block.Evidence.Hash()
	err := blockExec.ValidateBlock(state, block)

	require.Error(t, err)
	require.IsType(t, err, &types.ErrEvidenceInvalid{})
}
