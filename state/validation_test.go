package state_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/tmhash"
	memmock "github.com/tendermint/tendermint/internal/mempool/mock"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
	tmtime "github.com/tendermint/tendermint/libs/time"
	"github.com/tendermint/tendermint/pkg/abci"
	types "github.com/tendermint/tendermint/pkg/block"
	"github.com/tendermint/tendermint/pkg/consensus"
	"github.com/tendermint/tendermint/pkg/evidence"
	"github.com/tendermint/tendermint/pkg/metadata"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/mocks"
	sf "github.com/tendermint/tendermint/state/test/factory"
	"github.com/tendermint/tendermint/store"
	dbm "github.com/tendermint/tm-db"
)

const validationTestsStopHeight int64 = 10

func TestValidateBlockHeader(t *testing.T) {
	proxyApp := newTestApp()
	require.NoError(t, proxyApp.Start())
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	state, stateDB, privVals := makeState(3, 1)
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
	lastCommit := metadata.NewCommit(0, 0, metadata.BlockID{}, nil)

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
		{"Time wrong 2", func(block *types.Block) { block.Time = block.Time.Add(time.Second * 1) }},

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
			block.LastCommit = metadata.NewCommit(0, 0, metadata.BlockID{}, []metadata.CommitSig{metadata.NewCommitSigAbsent()})
			block.LastCommitHash = block.LastCommit.Hash()
		}},
	}

	// Build up state for multiple heights
	for height := int64(1); height < validationTestsStopHeight; height++ {
		/*
			Invalid blocks don't pass
		*/
		for _, tc := range testCases {
			block := sf.MakeBlock(state, height, lastCommit)
			tc.malleateBlock(block)
			err := blockExec.ValidateBlock(state, block)
			t.Logf("%s: %v", tc.name, err)
			require.Error(t, err, tc.name)
		}

		/*
			A good block passes
		*/
		var err error
		state, _, lastCommit, err = makeAndCommitGoodBlock(
			state, height, lastCommit, state.Validators.GetProposer().Address, blockExec, privVals, nil)
		require.NoError(t, err, "height %d", height)
	}

	nextHeight := validationTestsStopHeight
	block := sf.MakeBlock(state, nextHeight, lastCommit)
	state.InitialHeight = nextHeight + 1
	err := blockExec.ValidateBlock(state, block)
	require.Error(t, err, "expected an error when state is ahead of block")
	assert.Contains(t, err.Error(), "lower than initial height")
}

func TestValidateBlockCommit(t *testing.T) {
	proxyApp := newTestApp()
	require.NoError(t, proxyApp.Start())
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	state, stateDB, privVals := makeState(1, 1)
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
	lastCommit := metadata.NewCommit(0, 0, metadata.BlockID{}, nil)
	wrongSigsCommit := metadata.NewCommit(1, 0, metadata.BlockID{}, nil)
	badPrivVal := consensus.NewMockPV()

	for height := int64(1); height < validationTestsStopHeight; height++ {
		proposerAddr := state.Validators.GetProposer().Address
		if height > 1 {
			/*
				#2589: ensure state.LastValidators.VerifyCommit fails here
			*/
			// should be height-1 instead of height
			wrongHeightVote, err := factory.MakeVote(
				privVals[proposerAddr.String()],
				chainID,
				1,
				height,
				0,
				2,
				state.LastBlockID,
				time.Now(),
			)
			require.NoError(t, err, "height %d", height)
			wrongHeightCommit := metadata.NewCommit(
				wrongHeightVote.Height,
				wrongHeightVote.Round,
				state.LastBlockID,
				[]metadata.CommitSig{wrongHeightVote.CommitSig()},
			)
			block := sf.MakeBlock(state, height, wrongHeightCommit)
			err = blockExec.ValidateBlock(state, block)
			_, isErrInvalidCommitHeight := err.(consensus.ErrInvalidCommitHeight)
			require.True(t, isErrInvalidCommitHeight, "expected ErrInvalidCommitHeight at height %d but got: %v", height, err)

			/*
				#2589: test len(block.LastCommit.Signatures) == state.LastValidators.Size()
			*/
			block = sf.MakeBlock(state, height, wrongSigsCommit)
			err = blockExec.ValidateBlock(state, block)
			_, isErrInvalidCommitSignatures := err.(consensus.ErrInvalidCommitSignatures)
			require.True(t, isErrInvalidCommitSignatures,
				"expected ErrInvalidCommitSignatures at height %d, but got: %v",
				height,
				err,
			)
		}

		/*
			A good block passes
		*/
		var err error
		var blockID metadata.BlockID
		state, blockID, lastCommit, err = makeAndCommitGoodBlock(
			state,
			height,
			lastCommit,
			proposerAddr,
			blockExec,
			privVals,
			nil,
		)
		require.NoError(t, err, "height %d", height)

		/*
			wrongSigsCommit is fine except for the extra bad precommit
		*/
		goodVote, err := factory.MakeVote(
			privVals[proposerAddr.String()],
			chainID,
			1,
			height,
			0,
			2,
			blockID,
			time.Now(),
		)
		require.NoError(t, err, "height %d", height)

		bpvPubKey, err := badPrivVal.GetPubKey(context.Background())
		require.NoError(t, err)

		badVote := &consensus.Vote{
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

		err = badPrivVal.SignVote(context.Background(), chainID, g)
		require.NoError(t, err, "height %d", height)
		err = badPrivVal.SignVote(context.Background(), chainID, b)
		require.NoError(t, err, "height %d", height)

		goodVote.Signature, badVote.Signature = g.Signature, b.Signature

		wrongSigsCommit = metadata.NewCommit(goodVote.Height, goodVote.Round,
			blockID, []metadata.CommitSig{goodVote.CommitSig(), badVote.CommitSig()})
	}
}

func TestValidateBlockEvidence(t *testing.T) {
	proxyApp := newTestApp()
	require.NoError(t, proxyApp.Start())
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	state, stateDB, privVals := makeState(4, 1)
	stateStore := sm.NewStore(stateDB)
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	defaultEvidenceTime := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

	evpool := &mocks.EvidencePool{}
	evpool.On("CheckEvidence", mock.AnythingOfType("types.EvidenceList")).Return(nil)
	evpool.On("Update", mock.AnythingOfType("state.State"), mock.AnythingOfType("types.EvidenceList")).Return()
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
	lastCommit := metadata.NewCommit(0, 0, metadata.BlockID{}, nil)

	for height := int64(1); height < validationTestsStopHeight; height++ {
		proposerAddr := state.Validators.GetProposer().Address
		maxBytesEvidence := state.ConsensusParams.Evidence.MaxBytes
		if height > 1 {
			/*
				A block with too much evidence fails
			*/
			ev := make([]evidence.Evidence, 0)
			var currentBytes int64 = 0
			// more bytes than the maximum allowed for evidence
			for currentBytes <= maxBytesEvidence {
				newEv := evidence.NewMockDuplicateVoteEvidenceWithValidator(height, time.Now(),
					privVals[proposerAddr.String()], chainID)
				ev = append(ev, newEv)
				currentBytes += int64(len(newEv.Bytes()))
			}
			block, _ := state.MakeBlock(height, factory.MakeTenTxs(height), lastCommit, ev, proposerAddr)
			err := blockExec.ValidateBlock(state, block)
			if assert.Error(t, err) {
				_, ok := err.(*evidence.ErrEvidenceOverflow)
				require.True(t, ok, "expected error to be of type ErrEvidenceOverflow at height %d but got %v", height, err)
			}
		}

		/*
			A good block with several pieces of good evidence passes
		*/
		ev := make([]evidence.Evidence, 0)
		var currentBytes int64 = 0
		// precisely the amount of allowed evidence
		for {
			newEv := evidence.NewMockDuplicateVoteEvidenceWithValidator(height, defaultEvidenceTime,
				privVals[proposerAddr.String()], chainID)
			currentBytes += int64(len(newEv.Bytes()))
			if currentBytes >= maxBytesEvidence {
				break
			}
			ev = append([]evidence.Evidence{}, newEv)
		}

		var err error
		state, _, lastCommit, err = makeAndCommitGoodBlock(
			state,
			height,
			lastCommit,
			proposerAddr,
			blockExec,
			privVals,
			ev,
		)
		require.NoError(t, err, "height %d", height)
	}
}
