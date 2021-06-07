package state_test

import (
	"testing"
	"time"

	tmrand "github.com/tendermint/tendermint/libs/rand"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	memmock "github.com/tendermint/tendermint/mempool/mock"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/mocks"
	"github.com/tendermint/tendermint/types"
)

const validationTestsStopHeight int64 = 10

func TestValidateBlockHeader(t *testing.T) {
	proxyApp := newTestApp()
	require.NoError(t, proxyApp.Start())
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	state, stateDB, privVals := makeState(3, 1)
	stateStore := sm.NewStore(stateDB)
	nextChainLock := &types.CoreChainLock{
		CoreBlockHeight: 100,
		CoreBlockHash:   tmrand.Bytes(32),
		Signature:       tmrand.Bytes(96),
	}
	proTxHash := state.Validators.Validators[0].ProTxHash
	blockExec := sm.NewBlockExecutor(
		&proTxHash,
		stateStore,
		log.TestingLogger(),
		proxyApp.Consensus(),
		proxyApp.Query(),
		memmock.Mempool{},
		sm.EmptyEvidencePool{},
		nextChainLock,
	)
	lastCommit := types.NewCommit(0, 0, types.BlockID{}, types.StateID{}, nil, nil, nil, nil)

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
		{"Core Height does not match chain lock", func(block *types.Block) {
			block.CoreChainLockedHeight -= 10
		}},
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
		{"Proposer wrong", func(block *types.Block) { block.ProposerProTxHash = crypto.RandProTxHash() }},
		{"Proposer invalid", func(block *types.Block) { block.ProposerProTxHash = []byte("wrong size") }},
	}

	// Build up state for multiple heights
	for height := int64(1); height < validationTestsStopHeight; height++ {
		proposerProTxHash := state.Validators.GetProposer().ProTxHash
		/*
			Invalid blocks don't pass
		*/
		for _, tc := range testCases {
			block, _ := state.MakeBlock(height, nextChainLock, makeTxs(height), lastCommit, nil,
				proposerProTxHash)
			tc.malleateBlock(block)
			err := blockExec.ValidateBlock(state, block)
			require.Error(t, err, tc.name)
		}

		/*
			A good block passes
		*/
		var err error
		state, _, _, lastCommit, err = makeAndCommitGoodBlock(state, height, lastCommit, proposerProTxHash,
			blockExec, privVals, nil)
		require.NoError(t, err, "height %d", height)
	}
}

func TestValidateBlockCommit(t *testing.T) {
	proxyApp := newTestApp()
	require.NoError(t, proxyApp.Start())
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	state, stateDB, privVals := makeState(1, 1)
	stateStore := sm.NewStore(stateDB)

	nextChainLock := &types.CoreChainLock{
		CoreBlockHeight: 100,
		CoreBlockHash:   tmrand.Bytes(32),
		Signature:       tmrand.Bytes(96),
	}

	proTxHash := state.Validators.Validators[0].ProTxHash
	blockExec := sm.NewBlockExecutor(
		&proTxHash,
		stateStore,
		log.TestingLogger(),
		proxyApp.Consensus(),
		proxyApp.Query(),
		memmock.Mempool{},
		sm.EmptyEvidencePool{},
		nextChainLock,
	)
	lastCommit := types.NewCommit(0, 0, types.BlockID{}, types.StateID{}, nil, nil,
		nil, nil)
	wrongSigsCommit := types.NewCommit(1, 0, types.BlockID{}, types.StateID{}, nil, nil,
		nil, nil)
	badPrivVal := types.NewMockPV()

	for height := int64(1); height < validationTestsStopHeight; height++ {
		proTxHash := state.Validators.GetProposer().ProTxHash
		if height > 1 {
			/*
				#2589: ensure state.LastValidators.VerifyCommit fails here
			*/
			// should be height-1 instead of height
			wrongHeightVote, err := types.MakeVote(
				height,
				state.LastBlockID,
				types.StateID{LastAppHash: state.AppHash},
				state.Validators,
				privVals[proTxHash.String()],
				chainID,
			)
			require.NoError(t, err, "height %d", height)
			wrongHeightCommit := types.NewCommit(
				wrongHeightVote.Height,
				wrongHeightVote.Round,
				state.LastBlockID,
				types.StateID{LastAppHash: state.AppHash},
				[]types.CommitSig{wrongHeightVote.CommitSig()},
				state.Validators.QuorumHash,
				wrongHeightVote.BlockSignature,
				wrongHeightVote.StateSignature,
			)
			block, _ := state.MakeBlock(height, nextChainLock, makeTxs(height), wrongHeightCommit, nil,
				proTxHash)
			err = blockExec.ValidateBlock(state, block)
			_, isErrInvalidCommitHeight := err.(types.ErrInvalidCommitHeight)
			require.True(t, isErrInvalidCommitHeight, "expected ErrInvalidCommitHeight at height %d but got: %v",
				height, err)

			/*
				#2589: test len(block.LastPrecommits.Signatures) == state.LastValidators.Size()
			*/
			block, _ = state.MakeBlock(height, nextChainLock, makeTxs(height), wrongSigsCommit, nil, proTxHash)
			err = blockExec.ValidateBlock(state, block)
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
		var err error
		var blockID types.BlockID
		var stateID types.StateID
		state, blockID, stateID, lastCommit, err = makeAndCommitGoodBlock(
			state,
			height,
			lastCommit,
			proTxHash,
			blockExec,
			privVals,
			nil,
		)
		require.NoError(t, err, "height %d", height)

		/*
			wrongSigsCommit is fine except for the extra bad precommit
		*/
		proTxHashString := proTxHash.String()
		goodVote, err := types.MakeVote(height,
			blockID,
			stateID,
			state.Validators,
			privVals[proTxHashString],
			chainID,
		)
		require.NoError(t, err, "height %d", height)

		bpvProTxHash, err := badPrivVal.GetProTxHash()
		require.NoError(t, err)

		badVote := &types.Vote{
			ValidatorProTxHash: bpvProTxHash,
			ValidatorIndex:     0,
			Height:             height,
			Round:              0,
			Type:               tmproto.PrecommitType,
			BlockID:            blockID,
			StateID:            stateID,
		}

		g := goodVote.ToProto()
		b := badVote.ToProto()

		err = badPrivVal.SignVote(chainID, state.Validators.QuorumType, state.Validators.QuorumHash, g)
		require.NoError(t, err, "height %d", height)
		err = badPrivVal.SignVote(chainID, state.Validators.QuorumType, state.Validators.QuorumHash, b)
		require.NoError(t, err, "height %d", height)

		goodVote.BlockSignature, badVote.BlockSignature = g.BlockSignature, b.BlockSignature
		goodVote.StateSignature, badVote.StateSignature = g.StateSignature, b.StateSignature

		wrongSigsCommit = types.NewCommit(goodVote.Height, goodVote.Round,
			blockID, stateID, []types.CommitSig{goodVote.CommitSig(), badVote.CommitSig()}, state.Validators.QuorumHash,
			goodVote.BlockSignature, goodVote.StateSignature)
	}
}

func TestValidateBlockEvidence(t *testing.T) {
	proxyApp := newTestApp()
	require.NoError(t, proxyApp.Start())
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	state, stateDB, privVals := makeState(4, 1)
	stateStore := sm.NewStore(stateDB)
	defaultEvidenceTime := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

	evpool := &mocks.EvidencePool{}
	evpool.On("CheckEvidence", mock.AnythingOfType("types.EvidenceList")).Return(nil)
	evpool.On("Update", mock.AnythingOfType("state.State"),
		mock.AnythingOfType("types.EvidenceList")).Return()
	evpool.On("ABCIEvidence", mock.AnythingOfType("int64"),
		mock.AnythingOfType("[]types.Evidence")).Return([]abci.Evidence{})

	state.ConsensusParams.Evidence.MaxBytes = 1000
	proTxHash := state.Validators.Validators[0].ProTxHash
	blockExec := sm.NewBlockExecutor(
		&proTxHash,
		stateStore,
		log.TestingLogger(),
		proxyApp.Consensus(),
		proxyApp.Query(),
		memmock.Mempool{},
		evpool,
		nil,
	)
	lastCommit := types.NewCommit(0, 0, types.BlockID{}, types.StateID{}, nil, nil,
	nil, nil)

	for height := int64(1); height < validationTestsStopHeight; height++ {
		proposerProTxHash := state.Validators.GetProposer().ProTxHash
		maxBytesEvidence := state.ConsensusParams.Evidence.MaxBytes
		if height > 1 {
			/*
				A block with too much evidence fails
			*/
			evidence := make([]types.Evidence, 0)
			var currentBytes int64 = 0
			// more bytes than the maximum allowed for evidence
			for currentBytes <= maxBytesEvidence {
				newEv := types.NewMockDuplicateVoteEvidenceWithValidator(height, time.Now(),
					privVals[proposerProTxHash.String()], chainID, state.Validators.QuorumType,
					state.Validators.QuorumHash)
				evidence = append(evidence, newEv)
				currentBytes += int64(len(newEv.Bytes()))
			}
			block, _ := state.MakeBlock(height, nil, makeTxs(height), lastCommit, evidence, proposerProTxHash)
			err := blockExec.ValidateBlock(state, block)
			if assert.Error(t, err) {
				_, ok := err.(*types.ErrEvidenceOverflow)
				require.True(t, ok, "expected error to be of type ErrEvidenceOverflow at height %d but got %v",
					height, err)
			}
		}

		/*
			A good block with several pieces of good evidence passes
		*/
		evidence := make([]types.Evidence, 0)
		var currentBytes int64 = 0
		// precisely the amount of allowed evidence
		for {
			proposerProTxHashString := proposerProTxHash.String()
			newEv := types.NewMockDuplicateVoteEvidenceWithValidator(height, defaultEvidenceTime,
				privVals[proposerProTxHashString], chainID, state.Validators.QuorumType, state.Validators.QuorumHash)
			currentBytes += int64(len(newEv.Bytes()))
			if currentBytes >= maxBytesEvidence {
				break
			}
			evidence = append(evidence, newEv)
		}

		var err error
		state, _, _, lastCommit, err = makeAndCommitGoodBlock(
			state,
			height,
			lastCommit,
			proposerProTxHash,
			blockExec,
			privVals,
			evidence,
		)
		require.NoError(t, err, "height %d", height)
	}
}
