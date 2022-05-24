package state_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abciclient "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/internal/eventbus"
	mpmocks "github.com/tendermint/tendermint/internal/mempool/mocks"
	"github.com/tendermint/tendermint/internal/proxy"
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
	logger := log.NewNopLogger()
	proxyApp := proxy.New(abciclient.NewLocalClient(logger, &testApp{}), logger, proxy.NopMetrics())
	require.NoError(t, proxyApp.Start(ctx))

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	state, stateDB, privVals := makeState(t, 3, 1)
	stateStore := sm.NewStore(stateDB)
	mp := &mpmocks.Mempool{}
	mp.On("Lock").Return()
	mp.On("Unlock").Return()
	mp.On("FlushAppConn", mock.Anything).Return(nil)
	mp.On("Update",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(nil)

	blockStore := store.NewBlockStore(dbm.NewMemDB())
	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp,
		mp,
		sm.EmptyEvidencePool{},
		blockStore,
		eventBus,
		sm.NopMetrics(),
	)
	lastCommit := &types.Commit{}
	var lastExtCommit *types.ExtendedCommit

	// some bad values
	wrongHash := crypto.Checksum([]byte("this hash is wrong"))
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
			block.LastCommit = &types.Commit{Signatures: []types.CommitSig{types.NewCommitSigAbsent()}}
			block.LastCommitHash = block.LastCommit.Hash()
		}},
	}

	// Build up state for multiple heights
	for height := int64(1); height < validationTestsStopHeight; height++ {
		/*
			Invalid blocks don't pass
		*/
		for _, tc := range testCases {
			block := statefactory.MakeBlock(state, height, lastCommit)
			tc.malleateBlock(block)
			err := blockExec.ValidateBlock(ctx, state, block)
			t.Logf("%s: %v", tc.name, err)
			require.Error(t, err, tc.name)
		}

		/*
			A good block passes
		*/
		state, _, lastExtCommit = makeAndCommitGoodBlock(ctx, t,
			state, height, lastCommit, state.Validators.GetProposer().Address, blockExec, privVals, nil)
		lastCommit = lastExtCommit.ToCommit()
	}

	nextHeight := validationTestsStopHeight
	block := statefactory.MakeBlock(state, nextHeight, lastCommit)
	state.InitialHeight = nextHeight + 1
	err := blockExec.ValidateBlock(ctx, state, block)
	require.Error(t, err, "expected an error when state is ahead of block")
	assert.Contains(t, err.Error(), "lower than initial height")
}

func TestValidateBlockCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	proxyApp := proxy.New(abciclient.NewLocalClient(logger, &testApp{}), logger, proxy.NopMetrics())
	require.NoError(t, proxyApp.Start(ctx))

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	state, stateDB, privVals := makeState(t, 1, 1)
	stateStore := sm.NewStore(stateDB)
	mp := &mpmocks.Mempool{}
	mp.On("Lock").Return()
	mp.On("Unlock").Return()
	mp.On("FlushAppConn", mock.Anything).Return(nil)
	mp.On("Update",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(nil)

	blockStore := store.NewBlockStore(dbm.NewMemDB())
	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp,
		mp,
		sm.EmptyEvidencePool{},
		blockStore,
		eventBus,
		sm.NopMetrics(),
	)
	lastCommit := &types.Commit{}
	var lastExtCommit *types.ExtendedCommit
	wrongSigsCommit := &types.Commit{Height: 1}
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
			wrongHeightCommit := &types.Commit{
				Height:     wrongHeightVote.Height,
				Round:      wrongHeightVote.Round,
				BlockID:    state.LastBlockID,
				Signatures: []types.CommitSig{wrongHeightVote.CommitSig()},
			}
			block := statefactory.MakeBlock(state, height, wrongHeightCommit)
			err = blockExec.ValidateBlock(ctx, state, block)
			_, isErrInvalidCommitHeight := err.(types.ErrInvalidCommitHeight)
			require.True(t, isErrInvalidCommitHeight, "expected ErrInvalidCommitHeight at height %d but got: %v", height, err)

			/*
				#2589: test len(block.LastCommit.Signatures) == state.LastValidators.Size()
			*/
			block = statefactory.MakeBlock(state, height, wrongSigsCommit)
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
		state, blockID, lastExtCommit = makeAndCommitGoodBlock(
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
		lastCommit = lastExtCommit.ToCommit()

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

		wrongSigsCommit = &types.Commit{
			Height:     goodVote.Height,
			Round:      goodVote.Round,
			BlockID:    blockID,
			Signatures: []types.CommitSig{goodVote.CommitSig(), badVote.CommitSig()},
		}
	}
}

func TestValidateBlockEvidence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	proxyApp := proxy.New(abciclient.NewLocalClient(logger, &testApp{}), logger, proxy.NopMetrics())
	require.NoError(t, proxyApp.Start(ctx))

	state, stateDB, privVals := makeState(t, 4, 1)
	stateStore := sm.NewStore(stateDB)
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	defaultEvidenceTime := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

	evpool := &mocks.EvidencePool{}
	evpool.On("CheckEvidence", ctx, mock.AnythingOfType("types.EvidenceList")).Return(nil)
	evpool.On("Update", ctx, mock.AnythingOfType("state.State"), mock.AnythingOfType("types.EvidenceList")).Return()
	evpool.On("ABCIEvidence", mock.AnythingOfType("int64"), mock.AnythingOfType("[]types.Evidence")).Return(
		[]abci.Misbehavior{})

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))
	mp := &mpmocks.Mempool{}
	mp.On("Lock").Return()
	mp.On("Unlock").Return()
	mp.On("FlushAppConn", mock.Anything).Return(nil)
	mp.On("Update",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(nil)

	state.ConsensusParams.Evidence.MaxBytes = 1000
	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.NewNopLogger(),
		proxyApp,
		mp,
		evpool,
		blockStore,
		eventBus,
		sm.NopMetrics(),
	)
	lastCommit := &types.Commit{}
	var lastExtCommit *types.ExtendedCommit

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
			block := state.MakeBlock(height, testfactory.MakeNTxs(height, 10), lastCommit, evidence, proposerAddr)

			err := blockExec.ValidateBlock(ctx, state, block)
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

		state, _, lastExtCommit = makeAndCommitGoodBlock(
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
		lastCommit = lastExtCommit.ToCommit()

	}
}
