package state_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abciclient "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/eventbus"
	mpmocks "github.com/tendermint/tendermint/internal/mempool/mocks"
	"github.com/tendermint/tendermint/internal/proxy"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/state/mocks"
	statefactory "github.com/tendermint/tendermint/internal/state/test/factory"
	"github.com/tendermint/tendermint/internal/store"
	testfactory "github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
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
	nodeProTxHash := state.Validators.Validators[0].ProTxHash
	nextChainLock := &types.CoreChainLock{
		CoreBlockHeight: 100,
		CoreBlockHash:   tmrand.Bytes(32),
		Signature:       tmrand.Bytes(96),
	}
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
	blockExec.SetNextCoreChainLock(nextChainLock)
	lastCommit := types.NewCommit(0, 0, types.BlockID{}, types.StateID{}, nil)

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
		{"Core Height does not match chain lock", func(block *types.Block) {
			block.CoreChainLockedHeight -= 10
		}},
		{"Time wrong", func(block *types.Block) { block.Time = block.Time.Add(-time.Second * 1) }},

		{
			"LastBlockID wrong",
			func(block *types.Block) { block.LastBlockID.PartSetHeader.Total += 10 },
		},
		{"LastCommitHash wrong", func(block *types.Block) { block.LastCommitHash = wrongHash }},
		{"DataHash wrong", func(block *types.Block) { block.DataHash = wrongHash }},

		{"ValidatorsHash wrong", func(block *types.Block) { block.ValidatorsHash = wrongHash }},
		{
			"NextValidatorsHash wrong",
			func(block *types.Block) { block.NextValidatorsHash = wrongHash },
		},
		{"ConsensusHash wrong", func(block *types.Block) { block.ConsensusHash = wrongHash }},
		{"AppHash wrong", func(block *types.Block) { block.AppHash = wrongHash }},
		{"LastResultsHash wrong", func(block *types.Block) { block.LastResultsHash = wrongHash }},

		{"EvidenceHash wrong", func(block *types.Block) { block.EvidenceHash = wrongHash }},
		{
			"Proposer wrong",
			func(block *types.Block) { block.ProposerProTxHash = crypto.RandProTxHash() },
		},
		{
			"Proposer invalid",
			func(block *types.Block) { block.ProposerProTxHash = []byte("wrong size") },
		},
		// Set appVersion to 2 allow "invalid proposed app version" case
		{
			"Proposed app version is invalid",
			func(block *types.Block) { block.ProposedAppVersion = 1; state.Version.Consensus.App = 2 },
		},
	}

	// Set appVersion to 2 allow "invalid proposed app version" case

	// Build up state for multiple heights
	for height := int64(1); height < validationTestsStopHeight; height++ {
		proposerProTxHash := state.Validators.GetProposer().ProTxHash
		/*
			Invalid blocks don't pass
		*/
		for _, tc := range testCases {
			block, err := statefactory.MakeBlock(state, height, lastCommit, nextChainLock, 0)
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
			state, nodeProTxHash, height, lastCommit, proposerProTxHash, blockExec, privVals, nil, 3)
	}

	nextHeight := validationTestsStopHeight
	block, err := statefactory.MakeBlock(state, nextHeight, lastCommit, nextChainLock, 0)
	require.NoError(t, err)
	state.InitialHeight = nextHeight + 1
	err = blockExec.ValidateBlock(ctx, state, block)
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
	nodeProTxHash := state.Validators.Validators[0].ProTxHash
	stateStore := sm.NewStore(stateDB)
	nextChainLock := &types.CoreChainLock{
		CoreBlockHeight: 100,
		CoreBlockHash:   tmrand.Bytes(32),
		Signature:       tmrand.Bytes(96),
	}

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
	blockExec.SetNextCoreChainLock(nextChainLock)
	lastCommit := types.NewCommit(0, 0, types.BlockID{}, types.StateID{}, nil)
	wrongVoteMessageSignedCommit := types.NewCommit(1, 0, types.BlockID{}, types.StateID{}, nil)
	badPrivValQuorumHash := crypto.RandQuorumHash()
	badPrivVal := types.NewMockPVForQuorum(badPrivValQuorumHash)

	for height := int64(1); height < validationTestsStopHeight; height++ {
		stateID := state.StateID()
		proTxHash := state.Validators.GetProposer().ProTxHash
		if height > 1 {
			/*
				#2589: ensure state.LastValidators.VerifyCommit fails here
			*/
			// should be height-1 instead of height
			wrongHeightVote, err := testfactory.MakeVote(ctx,
				privVals[proTxHash.String()],
				state.Validators,
				chainID,
				1,
				height,
				0,
				2,
				state.LastBlockID,
				stateID,
			)
			require.NoError(t, err)
			thresholdSigns, err := types.NewSignsRecoverer([]*types.Vote{wrongHeightVote}).Recover()
			require.NoError(t, err)
			wrongHeightCommit := types.NewCommit(
				wrongHeightVote.Height,
				wrongHeightVote.Round,
				state.LastBlockID,
				stateID,
				&types.CommitSigns{
					QuorumSigns: *thresholdSigns,
					QuorumHash:  state.Validators.QuorumHash,
				},
			)
			block, err := statefactory.MakeBlock(state, height, wrongHeightCommit, nextChainLock, 0)
			require.NoError(t, err)
			err = blockExec.ValidateBlock(ctx, state, block)
			var wantErr types.ErrInvalidCommitHeight
			require.True(t, errors.As(err, &wantErr), "expected ErrInvalidCommitHeight at height %d but got: %v", height, err)

			/*
				Test that the threshold block signatures are good
			*/
			block, err = statefactory.MakeBlock(state, height, wrongVoteMessageSignedCommit, nextChainLock, 0)
			require.NoError(t, err)
			err = blockExec.ValidateBlock(ctx, state, block)
			require.True(
				t,
				strings.HasPrefix(
					err.Error(),
					"error validating block: invalid commit signatures for quorum",
				),
				"expected error on block threshold signature at height %d, but got: %v",
				height,
				err,
			)
		}

		/*
			A good block passes
		*/
		var blockID types.BlockID
		state, blockID, lastCommit = makeAndCommitGoodBlock(ctx, t,
			state,
			nodeProTxHash,
			height,
			lastCommit,
			proTxHash,
			blockExec,
			privVals,
			nil,
			0,
		)

		/*
			wrongSigsCommit is fine except for the extra bad precommit
		*/

		proTxHashString := proTxHash.String()
		goodVote, err := testfactory.MakeVote(ctx,
			privVals[proTxHashString],
			state.Validators,
			chainID,
			1,
			height,
			0,
			2,
			blockID,
			stateID,
		)
		require.NoError(t, err, "height %d", height)

		bpvProTxHash, err := badPrivVal.GetProTxHash(ctx)
		require.NoError(t, err)

		badVote := &types.Vote{
			ValidatorProTxHash: bpvProTxHash,
			ValidatorIndex:     0,
			Height:             height,
			Round:              0,
			Type:               tmproto.PrecommitType,
			BlockID:            blockID,
		}

		g := goodVote.ToProto()
		b := badVote.ToProto()

		err = badPrivVal.SignVote(ctx, chainID,
			state.Validators.QuorumType,
			badPrivValQuorumHash,
			g,
			stateID,
			nil,
		)
		require.NoError(t, err, "height %d", height)
		err = badPrivVal.SignVote(ctx,
			chainID,
			state.Validators.QuorumType,
			badPrivValQuorumHash,
			b,
			stateID,
			nil,
		)
		require.NoError(t, err, "height %d", height)

		goodVote.BlockSignature, badVote.BlockSignature = g.BlockSignature, b.BlockSignature
		goodVote.StateSignature, badVote.StateSignature = g.StateSignature, b.StateSignature
		goodVote.VoteExtensions = types.VoteExtensionsFromProto(g.VoteExtensions)
		badVote.VoteExtensions = types.VoteExtensionsFromProto(b.VoteExtensions)

		thresholdSigns, err := types.NewSignsRecoverer([]*types.Vote{badVote}).Recover()
		require.NoError(t, err)
		quorumSigns := &types.CommitSigns{
			QuorumSigns: *thresholdSigns,
			QuorumHash:  state.Validators.QuorumHash,
		}
		wrongVoteMessageSignedCommit = types.NewCommit(goodVote.Height, goodVote.Round,
			blockID, stateID, quorumSigns)
	}
}

func TestValidateBlockEvidence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	proxyApp := proxy.New(abciclient.NewLocalClient(logger, &testApp{}), logger, proxy.NopMetrics())
	require.NoError(t, proxyApp.Start(ctx))

	state, stateDB, privVals := makeState(t, 4, 1)
	nodeProTxHash := state.Validators.Validators[0].ProTxHash
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
	lastCommit := types.NewCommit(0, 0, types.BlockID{}, types.StateID{}, nil)

	for height := int64(1); height < validationTestsStopHeight; height++ {
		proposerProTxHash := state.Validators.GetProposer().ProTxHash
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
					privVals[proposerProTxHash.String()], chainID, state.Validators.QuorumType,
					state.Validators.QuorumHash)
				require.NoError(t, err)
				evidence = append(evidence, newEv)
				currentBytes += int64(len(newEv.Bytes()))
			}
			block := state.MakeBlock(
				height,
				nil,
				testfactory.MakeNTxs(height, 10),
				lastCommit,
				evidence,
				proposerProTxHash,
				0,
			)

			err := blockExec.ValidateBlock(ctx, state, block)
			if assert.Error(t, err) {
				_, ok := err.(*types.ErrEvidenceOverflow)
				require.True(
					t,
					ok,
					"expected error to be of type ErrEvidenceOverflow at height %d but got %v",
					height,
					err,
				)
			}
		}

		/*
			A good block with several pieces of good evidence passes
		*/
		evidence := make([]types.Evidence, 0)
		var currentBytes int64
		// precisely the amount of allowed evidence
		for {
			propProTxHashStr := proposerProTxHash.String()
			newEv, err := types.NewMockDuplicateVoteEvidenceWithValidator(
				ctx,
				height,
				defaultEvidenceTime,
				privVals[propProTxHashStr],
				chainID,
				state.Validators.QuorumType,
				state.Validators.QuorumHash,
			)
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
			nodeProTxHash,
			height,
			lastCommit,
			proposerProTxHash,
			blockExec,
			privVals,
			evidence,
			0,
		)
	}
}
