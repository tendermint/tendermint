package state_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/tmhash"
	memmock "github.com/tendermint/tendermint/internal/mempool/mock"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/state/mocks"
	statefactory "github.com/tendermint/tendermint/internal/state/test/factory"
	"github.com/tendermint/tendermint/internal/store"
	testfactory "github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
	tmtime "github.com/tendermint/tendermint/libs/time"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

const validationTestsStopHeight int64 = 10

func TestValidateBlockHeader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	proxyApp := newTestApp()
	require.NoError(t, proxyApp.Start(ctx))

	state, stateDB, privVals := makeState(t, 3, 1)
	stateStore := sm.NewStore(stateDB)
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.TestingLogger(),
		proxyApp.Consensus(),
		memmock.Mempool{},
		sm.EmptyEvidencePool{},
		blockStore,
	)
	lastCommit := types.NewCommit(0, 0, types.BlockID{}, nil)

	// some bad values
	wrongHash := tmhash.Sum([]byte("this hash is wrong"))
	wrongVersion1 := state.Version.Consensus
	wrongVersion1.Block += 2
	wrongVersion2 := state.Version.Consensus
	wrongVersion2.App += 2

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

		{"LastBlockID wrong", func(block *types.Block) { block.LastBlockID.PartSetHeader.Total += 10 }},
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

		{"first LastCommit contains signatures", func(block *types.Block) {
			block.LastCommit = types.NewCommit(0, 0, types.BlockID{}, []types.CommitSig{types.NewCommitSigAbsent()})
			block.LastCommitHash = block.LastCommit.Hash()
		}},
	}

	// Build up state for multiple heights
	for height := int64(1); height < validationTestsStopHeight; height++ {
		/*
			Invalid blocks don't pass
		*/
		for _, tc := range testCases {
			block, err := statefactory.MakeBlock(state, height, lastCommit)
			require.NoError(t, err)
			tc.malleateBlock(block)
			err = blockExec.ValidateBlock(ctx, state, block)
			t.Logf("%s: %v", tc.name, err)
			require.Error(t, err, tc.name)
		}

		/*
			A good block passes
		*/
		state, _, lastCommit = makeAndCommitGoodBlock(ctx, t,
			state, height, lastCommit, state.Validators.GetProposer().Address, blockExec, privVals, nil)
	}

	nextHeight := validationTestsStopHeight
	block, err := statefactory.MakeBlock(state, nextHeight, lastCommit)
	require.NoError(t, err)
	state.InitialHeight = nextHeight + 1
	err = blockExec.ValidateBlock(ctx, state, block)
	require.Error(t, err, "expected an error when state is ahead of block")
	assert.Contains(t, err.Error(), "lower than initial height")
}

func TestValidateBlockCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	proxyApp := newTestApp()
	require.NoError(t, proxyApp.Start(ctx))

	state, stateDB, privVals := makeState(t, 1, 1)
	stateStore := sm.NewStore(stateDB)
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.TestingLogger(),
		proxyApp.Consensus(),
		memmock.Mempool{},
		sm.EmptyEvidencePool{},
		blockStore,
	)
	lastCommit := types.NewCommit(0, 0, types.BlockID{}, nil)
	wrongSigsCommit := types.NewCommit(1, 0, types.BlockID{}, nil)
	badPrivVal := types.NewMockPV()

	for height := int64(1); height < validationTestsStopHeight; height++ {
		proposerAddr := state.Validators.GetProposer().Address
		if height > 1 {
			/*
				#2589: ensure state.LastValidators.VerifyCommit fails here
			*/
			// should be height-1 instead of height
			wrongHeightVote, err := testfactory.MakeVote(
				ctx,
				privVals[proposerAddr.String()],
				chainID,
				1,
				height,
				0,
				2,
				state.LastBlockID,
				time.Now(),
			)
			require.NoError(t, err)
			wrongHeightCommit := types.NewCommit(
				wrongHeightVote.Height,
				wrongHeightVote.Round,
				state.LastBlockID,
				[]types.CommitSig{wrongHeightVote.CommitSig()},
			)
			block, err := statefactory.MakeBlock(state, height, wrongHeightCommit)
			require.NoError(t, err)
			err = blockExec.ValidateBlock(ctx, state, block)
			_, isErrInvalidCommitHeight := err.(types.ErrInvalidCommitHeight)
			require.True(t, isErrInvalidCommitHeight, "expected ErrInvalidCommitHeight at height %d but got: %v", height, err)

			/*
				#2589: test len(block.LastCommit.Signatures) == state.LastValidators.Size()
			*/
			block, err = statefactory.MakeBlock(state, height, wrongSigsCommit)
			require.NoError(t, err)
			err = blockExec.ValidateBlock(ctx, state, block)
			_, isErrInvalidCommitSignatures := err.(types.ErrInvalidCommitSignatures)
			require.True(t, isErrInvalidCommitSignatures,
				"expected ErrInvalidCommitSignatures at height %d, but got: %v",
				height,
				err,
			)
		}

		/*
			A good block passes
		*/
		var blockID types.BlockID
		state, blockID, lastCommit = makeAndCommitGoodBlock(
			ctx,
			t,
			state,
			height,
			lastCommit,
			proposerAddr,
			blockExec,
			privVals,
			nil,
		)

		/*
			wrongSigsCommit is fine except for the extra bad precommit
		*/
		goodVote, err := testfactory.MakeVote(
			ctx,
			privVals[proposerAddr.String()],
			chainID,
			1,
			height,
			0,
			2,
			blockID,
			time.Now(),
		)
		require.NoError(t, err)
		bpvPubKey, err := badPrivVal.GetPubKey(ctx)
		require.NoError(t, err)

		badVote := &types.Vote{
			ValidatorAddress: bpvPubKey.Address(),
			ValidatorIndex:   0,
			Height:           height,
			Round:            0,
			Timestamp:        tmtime.Now(),
			Type:             tmproto.PrecommitType,
			BlockID:          blockID,
		}

		g := goodVote.ToProto()
		b := badVote.ToProto()

		err = badPrivVal.SignVote(ctx, chainID, g)
		require.NoError(t, err, "height %d", height)
		err = badPrivVal.SignVote(ctx, chainID, b)
		require.NoError(t, err, "height %d", height)

		goodVote.Signature, badVote.Signature = g.Signature, b.Signature

		wrongSigsCommit = types.NewCommit(goodVote.Height, goodVote.Round,
			blockID, []types.CommitSig{goodVote.CommitSig(), badVote.CommitSig()})
	}
}

func TestValidateBlockEvidence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	proxyApp := newTestApp()
	require.NoError(t, proxyApp.Start(ctx))

	state, stateDB, privVals := makeState(t, 4, 1)
	stateStore := sm.NewStore(stateDB)
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	defaultEvidenceTime := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

	evpool := &mocks.EvidencePool{}
	evpool.On("CheckEvidence", ctx, mock.AnythingOfType("types.EvidenceList")).Return(nil)
	evpool.On("Update", ctx, mock.AnythingOfType("state.State"), mock.AnythingOfType("types.EvidenceList")).Return()
	evpool.On("ABCIEvidence", mock.AnythingOfType("int64"), mock.AnythingOfType("[]types.Evidence")).Return(
		[]abci.Evidence{})

	state.ConsensusParams.Evidence.MaxBytes = 1000
	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.TestingLogger(),
		proxyApp.Consensus(),
		memmock.Mempool{},
		evpool,
		blockStore,
	)
	lastCommit := types.NewCommit(0, 0, types.BlockID{}, nil)

	for height := int64(1); height < validationTestsStopHeight; height++ {
		proposerAddr := state.Validators.GetProposer().Address
		maxBytesEvidence := state.ConsensusParams.Evidence.MaxBytes
		if height > 1 {
			/*
				A block with too much evidence fails
			*/
			evidence := make([]types.Evidence, 0)
			var currentBytes int64
			// more bytes than the maximum allowed for evidence
			for currentBytes <= maxBytesEvidence {
				newEv, err := types.NewMockDuplicateVoteEvidenceWithValidator(ctx, height, time.Now(),
					privVals[proposerAddr.String()], chainID)
				require.NoError(t, err)
				evidence = append(evidence, newEv)
				currentBytes += int64(len(newEv.Bytes()))
			}
			block, _, err := state.MakeBlock(height, testfactory.MakeTenTxs(height), lastCommit, evidence, proposerAddr)
			require.NoError(t, err)

			err = blockExec.ValidateBlock(ctx, state, block)
			if assert.Error(t, err) {
				_, ok := err.(*types.ErrEvidenceOverflow)
				require.True(t, ok, "expected error to be of type ErrEvidenceOverflow at height %d but got %v", height, err)
			}
		}

		/*
			A good block with several pieces of good evidence passes
		*/
		evidence := make([]types.Evidence, 0)
		var currentBytes int64
		// precisely the amount of allowed evidence
		for {
			newEv, err := types.NewMockDuplicateVoteEvidenceWithValidator(ctx, height, defaultEvidenceTime,
				privVals[proposerAddr.String()], chainID)
			require.NoError(t, err)
			currentBytes += int64(len(newEv.Bytes()))
			if currentBytes >= maxBytesEvidence {
				break
			}
			evidence = append(evidence, newEv)
		}

		state, _, lastCommit = makeAndCommitGoodBlock(
			ctx,
			t,
			state,
			height,
			lastCommit,
			proposerAddr,
			blockExec,
			privVals,
			evidence,
		)

	}
}
