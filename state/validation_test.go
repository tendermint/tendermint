package state

import (
	"testing"
	"time"

	"github.com/tendermint/tendermint/mock"
	"github.com/tendermint/tendermint/proxy"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

func TestValidateBlockHeader(t *testing.T) {
	proxyApp := newTestApp()
	require.NoError(t, proxyApp.Start())
	defer proxyApp.Stop()

	state, stateDB, privVals := state(1, 1)
	blockExec := NewBlockExecutor(stateDB, log.TestingLogger(), proxyApp.Consensus(), mock.Mempool{}, MockEvidencePool{})
	// we assume a single validator for this test
	privVal := privVals[0]
	commit := types.NewCommit(types.BlockID{}, nil)

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

	for height := int64(1); height < 10; height++ {
		/*
			Invalid blocks don't pass
		*/
		for _, tc := range testCases {
			block, _ := state.MakeBlock(
				height,
				makeTxs(height),
				commit,
				nil,
				state.Validators.GetProposer().Address,
			)
			tc.malleateBlock(block)
			err := blockExec.ValidateBlock(state, block)
			require.Error(t, err, tc.name)
		}

		/*
			A good block passes
		*/
		block, _ := state.MakeBlock(height, makeTxs(height), commit, nil, state.Validators.GetProposer().Address)
		err := blockExec.ValidateBlock(state, block)
		require.NoError(t, err, "height %d", height)

		/*
			Apply the block to our current state
		*/
		blockID := types.BlockID{Hash: block.Hash(), PartsHeader: types.PartSetHeader{}}
		state, err = blockExec.ApplyBlock(state, blockID, block)
		require.NoError(t, err, "height %d", height)

		/*
			Simulate a commit for this block
		*/
		vote, err := makeVote(height, blockID, state.Validators, privVal)
		require.NoError(t, err, "height %d", height)
		// for the next height
		commit = types.NewCommit(blockID, []*types.CommitSig{vote.CommitSig()})
	}
}

func TestValidateBlockCommit(t *testing.T) {
	proxyApp := newTestApp()
	require.NoError(t, proxyApp.Start())
	defer proxyApp.Stop()

	state, stateDB, privVals := state(1, 1)
	blockExec := NewBlockExecutor(stateDB, log.TestingLogger(), proxyApp.Consensus(), mock.Mempool{}, MockEvidencePool{})
	// we assume a single validator for this test
	privVal := privVals[0]
	commit := types.NewCommit(types.BlockID{}, nil)
	wrongPrecommitsCommit := types.NewCommit(types.BlockID{}, nil)
	badPrivVal := types.NewMockPV()

	for height := int64(1); height < 10; height++ {
		if height > 1 {
			/*
				#2589: ensure state.LastValidators.VerifyCommit fails here
			*/
			// should be height-1 instead of height
			wrongHeightVote, err := makeVote(height, state.LastBlockID, state.Validators, privVal)
			require.NoError(t, err, "height %d", height)
			wrongHeightCommit := types.NewCommit(state.LastBlockID, []*types.CommitSig{wrongHeightVote.CommitSig()})
			block, _ := state.MakeBlock(height, makeTxs(height), wrongHeightCommit, nil, state.Validators.GetProposer().Address)
			err = blockExec.ValidateBlock(state, block)
			_, isErrInvalidCommitHeight := err.(types.ErrInvalidCommitHeight)
			require.True(t, isErrInvalidCommitHeight, "expected ErrInvalidCommitHeight at height %d but got: %v", height, err)

			/*
				#2589: test len(block.LastCommit.Precommits) == state.LastValidators.Size()
			*/
			block, _ = state.MakeBlock(height, makeTxs(height), wrongPrecommitsCommit, nil, state.Validators.GetProposer().Address)
			err = blockExec.ValidateBlock(state, block)
			_, isErrInvalidCommitPrecommits := err.(types.ErrInvalidCommitPrecommits)
			require.True(t, isErrInvalidCommitPrecommits, "expected ErrInvalidCommitPrecommits height %d but got: %v", height, err)
		}

		/*
			A good block passes
		*/
		block, _ := state.MakeBlock(height, makeTxs(height), commit, nil, state.Validators.GetProposer().Address)
		err := blockExec.ValidateBlock(state, block)
		require.NoError(t, err, "height %d", height)

		blockID := types.BlockID{Hash: block.Hash(), PartsHeader: types.PartSetHeader{}}
		// apply the block to our current state
		state, err = blockExec.ApplyBlock(state, blockID, block)
		require.NoError(t, err, "height %d", height)

		/*
			Simulate a correct commit for this block
		*/
		vote, err := makeVote(height, blockID, state.Validators, privVal)
		require.NoError(t, err, "height %d", height)
		// for the next height
		commit = types.NewCommit(blockID, []*types.CommitSig{vote.CommitSig()})

		/*
			wrongPrecommitsCommit is fine except for the extra bad precommit
		*/
		badVote := &types.Vote{
			ValidatorAddress: badPrivVal.GetPubKey().Address(),
			ValidatorIndex:   0,
			Height:           height,
			Round:            0,
			Timestamp:        tmtime.Now(),
			Type:             types.PrecommitType,
			BlockID:          blockID,
		}
		err = badPrivVal.SignVote(chainID, vote)
		require.NoError(t, err, "height %d", height)
		wrongPrecommitsCommit = types.NewCommit(blockID, []*types.CommitSig{vote.CommitSig(), badVote.CommitSig()})
	}
}

/*
	TODO(#2589):
	- test good/bad evidence in block
*/
func TestValidateBlockEvidence(t *testing.T) {
	var height int64 = 1 // TODO(#2589): generalize
	state, stateDB, _ := state(1, int(height))

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
	maxBlockSize := state.ConsensusParams.Block.MaxBytes
	maxNumEvidence, _ := types.MaxEvidencePerBlock(maxBlockSize)
	require.True(t, maxNumEvidence > 2)
	for i := int64(0); i < maxNumEvidence; i++ {
		block.Evidence.Evidence = append(block.Evidence.Evidence, goodEvidence)
	}
	block.EvidenceHash = block.Evidence.Hash()
	err = blockExec.ValidateBlock(state, block)
	require.Error(t, err)
	_, ok := err.(*types.ErrEvidenceOverflow)
	require.True(t, ok)
}

// always returns true if asked if any evidence was already committed.
type mockEvPoolAlwaysCommitted struct{}

func (m mockEvPoolAlwaysCommitted) PendingEvidence(int64) []types.Evidence { return nil }
func (m mockEvPoolAlwaysCommitted) AddEvidence(types.Evidence) error       { return nil }
func (m mockEvPoolAlwaysCommitted) Update(*types.Block, State)             {}
func (m mockEvPoolAlwaysCommitted) IsCommitted(types.Evidence) bool        { return true }

func TestValidateFailBlockOnCommittedEvidence(t *testing.T) {
	var height int64 = 1
	state, stateDB, _ := state(1, int(height))

	blockExec := NewBlockExecutor(stateDB, log.TestingLogger(), nil, nil, mockEvPoolAlwaysCommitted{})
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

/*
	TODO(#2589):
	- test unmarshalling BlockParts that are too big into a Block that
		(note this logic happens in the consensus, not in the validation here).
	- test making blocks from the types.MaxXXX functions works/fails as expected
*/
func TestValidateBlockSize(t *testing.T) {
}

//-----------------------------------------------------------------------------

func newTestApp() proxy.AppConns {
	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	return proxy.NewAppConns(cc)
}

func makeVote(height int64, blockID types.BlockID, valSet *types.ValidatorSet, privVal types.PrivValidator) (*types.Vote, error) {
	addr := privVal.GetPubKey().Address()
	idx, _ := valSet.GetByAddress(addr)
	vote := &types.Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   idx,
		Height:           height,
		Round:            0,
		Timestamp:        tmtime.Now(),
		Type:             types.PrecommitType,
		BlockID:          blockID,
	}
	if err := privVal.SignVote(chainID, vote); err != nil {
		return nil, err
	}
	return vote, nil
}
