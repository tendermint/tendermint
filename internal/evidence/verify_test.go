package evidence_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/evidence"
	"github.com/tendermint/tendermint/internal/evidence/mocks"
	sm "github.com/tendermint/tendermint/internal/state"
	smmocks "github.com/tendermint/tendermint/internal/state/mocks"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

const (
	defaultVotingPower = 10
)

func TestVerifyLightClientAttack_Lunatic(t *testing.T) {
	const (
		height       int64 = 10
		commonHeight int64 = 4
		totalVals          = 10
		byzVals            = 4
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	attackTime := defaultEvidenceTime.Add(1 * time.Hour)
	// create valid lunatic evidence
	ev, trusted, common := makeLunaticEvidence(ctx,
		t, height, commonHeight, totalVals, byzVals, totalVals-byzVals, defaultEvidenceTime, attackTime)
	require.NoError(t, ev.ValidateBasic())

	// good pass -> no error
	err := evidence.VerifyLightClientAttack(ev, common.SignedHeader, trusted.SignedHeader, common.ValidatorSet,
		defaultEvidenceTime.Add(2*time.Hour), 3*time.Hour)
	assert.NoError(t, err)

	// trusted and conflicting hashes are the same -> an error should be returned
	err = evidence.VerifyLightClientAttack(ev, common.SignedHeader, ev.ConflictingBlock.SignedHeader, common.ValidatorSet,
		defaultEvidenceTime.Add(2*time.Hour), 3*time.Hour)
	assert.Error(t, err)

	// evidence with different total validator power should fail
	ev.TotalVotingPower = 1 * defaultVotingPower
	err = evidence.VerifyLightClientAttack(ev, common.SignedHeader, trusted.SignedHeader, common.ValidatorSet,
		defaultEvidenceTime.Add(2*time.Hour), 3*time.Hour)
	assert.NoError(t, err)
	assert.Error(t, ev.ValidateABCI(common.ValidatorSet, trusted.SignedHeader, defaultEvidenceTime))

	// evidence without enough malicious votes should fail
	ev, trusted, common = makeLunaticEvidence(ctx,
		t, height, commonHeight, totalVals, byzVals-1, totalVals-byzVals, defaultEvidenceTime, attackTime)
	err = evidence.VerifyLightClientAttack(ev, common.SignedHeader, trusted.SignedHeader, common.ValidatorSet,
		defaultEvidenceTime.Add(2*time.Hour), 3*time.Hour)
	assert.Error(t, err)
}

func TestVerify_LunaticAttackAgainstState(t *testing.T) {
	const (
		height       int64 = 10
		commonHeight int64 = 4
		totalVals          = 10
		byzVals            = 4
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := log.NewNopLogger()

	attackTime := defaultEvidenceTime.Add(1 * time.Hour)
	// create valid lunatic evidence
	ev, trusted, common := makeLunaticEvidence(ctx,
		t, height, commonHeight, totalVals, byzVals, totalVals-byzVals, defaultEvidenceTime, attackTime)

	// now we try to test verification against state
	state := sm.State{
		LastBlockTime:   defaultEvidenceTime.Add(2 * time.Hour),
		LastBlockHeight: height + 1,
		ConsensusParams: *types.DefaultConsensusParams(),
	}
	stateStore := &smmocks.Store{}
	stateStore.On("LoadValidators", commonHeight).Return(common.ValidatorSet, nil)
	stateStore.On("Load").Return(state, nil)
	blockStore := &mocks.BlockStore{}
	blockStore.On("LoadBlockMeta", commonHeight).Return(&types.BlockMeta{Header: *common.Header})
	blockStore.On("LoadBlockMeta", height).Return(&types.BlockMeta{Header: *trusted.Header})
	blockStore.On("LoadBlockCommit", commonHeight).Return(common.Commit)
	blockStore.On("LoadBlockCommit", height).Return(trusted.Commit)
	pool := evidence.NewPool(log.NewNopLogger(), dbm.NewMemDB(), stateStore, blockStore, evidence.NopMetrics(), nil)

	evList := types.EvidenceList{ev}
	// check that the evidence pool correctly verifies the evidence
	assert.NoError(t, pool.CheckEvidence(ctx, evList))

	// as it was not originally in the pending bucket, it should now have been added
	pendingEvs, _ := pool.PendingEvidence(state.ConsensusParams.Evidence.MaxBytes)
	assert.Equal(t, 1, len(pendingEvs))
	assert.Equal(t, ev, pendingEvs[0])

	// if we submit evidence only against a single byzantine validator when we see there are more validators then this
	// should return an error
	ev.ByzantineValidators = ev.ByzantineValidators[:1]
	assert.Error(t, pool.CheckEvidence(ctx, evList))
	// restore original byz vals
	ev.ByzantineValidators = ev.GetByzantineValidators(common.ValidatorSet, trusted.SignedHeader)

	// duplicate evidence should be rejected
	evList = types.EvidenceList{ev, ev}
	pool = evidence.NewPool(logger, dbm.NewMemDB(), stateStore, blockStore, evidence.NopMetrics(), nil)
	assert.Error(t, pool.CheckEvidence(ctx, evList))

	// If evidence is submitted with an altered timestamp it should return an error
	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	ev.Timestamp = defaultEvidenceTime.Add(1 * time.Minute)
	pool = evidence.NewPool(logger, dbm.NewMemDB(), stateStore, blockStore, evidence.NopMetrics(), eventBus)

	err := pool.AddEvidence(ctx, ev)
	assert.Error(t, err)
	ev.Timestamp = defaultEvidenceTime

	// Evidence submitted with a different validator power should fail
	ev.TotalVotingPower = 1
	pool = evidence.NewPool(logger, dbm.NewMemDB(), stateStore, blockStore, evidence.NopMetrics(), nil)
	err = pool.AddEvidence(ctx, ev)
	assert.Error(t, err)
	ev.TotalVotingPower = common.ValidatorSet.TotalVotingPower()
}

func TestVerify_ForwardLunaticAttack(t *testing.T) {
	const (
		nodeHeight   int64 = 8
		attackHeight int64 = 10
		commonHeight int64 = 4
		totalVals          = 10
		byzVals            = 5
	)
	attackTime := defaultEvidenceTime.Add(1 * time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	// create a forward lunatic attack
	ev, trusted, common := makeLunaticEvidence(ctx,
		t, attackHeight, commonHeight, totalVals, byzVals, totalVals-byzVals, defaultEvidenceTime, attackTime)

	// now we try to test verification against state
	state := sm.State{
		LastBlockTime:   defaultEvidenceTime.Add(2 * time.Hour),
		LastBlockHeight: nodeHeight,
		ConsensusParams: *types.DefaultConsensusParams(),
	}

	// modify trusted light block so that it is of a height less than the conflicting one
	trusted.Header.Height = state.LastBlockHeight
	trusted.Header.Time = state.LastBlockTime

	stateStore := &smmocks.Store{}
	stateStore.On("LoadValidators", commonHeight).Return(common.ValidatorSet, nil)
	stateStore.On("Load").Return(state, nil)
	blockStore := &mocks.BlockStore{}
	blockStore.On("LoadBlockMeta", commonHeight).Return(&types.BlockMeta{Header: *common.Header})
	blockStore.On("LoadBlockMeta", nodeHeight).Return(&types.BlockMeta{Header: *trusted.Header})
	blockStore.On("LoadBlockMeta", attackHeight).Return(nil)
	blockStore.On("LoadBlockCommit", commonHeight).Return(common.Commit)
	blockStore.On("LoadBlockCommit", nodeHeight).Return(trusted.Commit)
	blockStore.On("Height").Return(nodeHeight)

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	pool := evidence.NewPool(logger, dbm.NewMemDB(), stateStore, blockStore, evidence.NopMetrics(), eventBus)

	// check that the evidence pool correctly verifies the evidence
	assert.NoError(t, pool.CheckEvidence(ctx, types.EvidenceList{ev}))

	// now we use a time which isn't able to contradict the FLA - thus we can't verify the evidence
	oldBlockStore := &mocks.BlockStore{}
	oldHeader := trusted.Header
	oldHeader.Time = defaultEvidenceTime
	oldBlockStore.On("LoadBlockMeta", commonHeight).Return(&types.BlockMeta{Header: *common.Header})
	oldBlockStore.On("LoadBlockMeta", nodeHeight).Return(&types.BlockMeta{Header: *oldHeader})
	oldBlockStore.On("LoadBlockMeta", attackHeight).Return(nil)
	oldBlockStore.On("LoadBlockCommit", commonHeight).Return(common.Commit)
	oldBlockStore.On("LoadBlockCommit", nodeHeight).Return(trusted.Commit)
	oldBlockStore.On("Height").Return(nodeHeight)
	require.Equal(t, defaultEvidenceTime, oldBlockStore.LoadBlockMeta(nodeHeight).Header.Time)

	pool = evidence.NewPool(logger, dbm.NewMemDB(), stateStore, oldBlockStore, evidence.NopMetrics(), nil)
	assert.Error(t, pool.CheckEvidence(ctx, types.EvidenceList{ev}))
}

func TestVerifyLightClientAttack_Equivocation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	conflictingVals, conflictingPrivVals := factory.ValidatorSet(ctx, t, 5, 10)

	conflictingHeader := factory.MakeHeader(t, &types.Header{
		ChainID:        evidenceChainID,
		Height:         10,
		Time:           defaultEvidenceTime,
		ValidatorsHash: conflictingVals.Hash(),
	})

	trustedHeader := factory.MakeHeader(t, &types.Header{
		ChainID:            evidenceChainID,
		Height:             10,
		Time:               defaultEvidenceTime,
		ValidatorsHash:     conflictingHeader.ValidatorsHash,
		NextValidatorsHash: conflictingHeader.NextValidatorsHash,
		ConsensusHash:      conflictingHeader.ConsensusHash,
		AppHash:            conflictingHeader.AppHash,
		LastResultsHash:    conflictingHeader.LastResultsHash,
	})

	// we are simulating a duplicate vote attack where all the validators in the conflictingVals set
	// except the last validator vote twice
	blockID := factory.MakeBlockIDWithHash(conflictingHeader.Hash())
	voteSet := types.NewExtendedVoteSet(evidenceChainID, 10, 1, tmproto.SignedMsgType(2), conflictingVals)
	extCommit, err := factory.MakeExtendedCommit(ctx, blockID, 10, 1, voteSet, conflictingPrivVals[:4], defaultEvidenceTime)
	require.NoError(t, err)
	commit := extCommit.ToCommit()

	ev := &types.LightClientAttackEvidence{
		ConflictingBlock: &types.LightBlock{
			SignedHeader: &types.SignedHeader{
				Header: conflictingHeader,
				Commit: commit,
			},
			ValidatorSet: conflictingVals,
		},
		CommonHeight:        10,
		ByzantineValidators: conflictingVals.Validators[:4],
		TotalVotingPower:    50,
		Timestamp:           defaultEvidenceTime,
	}

	trustedBlockID := makeBlockID(trustedHeader.Hash(), 1000, []byte("partshash"))
	trustedVoteSet := types.NewExtendedVoteSet(evidenceChainID, 10, 1, tmproto.SignedMsgType(2), conflictingVals)
	trustedExtCommit, err := factory.MakeExtendedCommit(ctx, trustedBlockID, 10, 1,
		trustedVoteSet, conflictingPrivVals, defaultEvidenceTime)
	require.NoError(t, err)
	trustedCommit := trustedExtCommit.ToCommit()

	trustedSignedHeader := &types.SignedHeader{
		Header: trustedHeader,
		Commit: trustedCommit,
	}

	// good pass -> no error
	require.NoError(t, evidence.VerifyLightClientAttack(ev, trustedSignedHeader, trustedSignedHeader, conflictingVals,
		defaultEvidenceTime.Add(1*time.Minute), 2*time.Hour))

	// trusted and conflicting hashes are the same -> an error should be returned
	assert.Error(t, evidence.VerifyLightClientAttack(ev, trustedSignedHeader, ev.ConflictingBlock.SignedHeader, conflictingVals,
		defaultEvidenceTime.Add(1*time.Minute), 2*time.Hour))

	// conflicting header has different next validators hash which should have been correctly derived from
	// the previous round
	ev.ConflictingBlock.Header.NextValidatorsHash = crypto.CRandBytes(crypto.HashSize)
	assert.Error(t, evidence.VerifyLightClientAttack(ev, trustedSignedHeader, trustedSignedHeader, nil,
		defaultEvidenceTime.Add(1*time.Minute), 2*time.Hour))

	// revert next validators hash
	ev.ConflictingBlock.Header.NextValidatorsHash = trustedHeader.NextValidatorsHash

	state := sm.State{
		LastBlockTime:   defaultEvidenceTime.Add(1 * time.Minute),
		LastBlockHeight: 11,
		ConsensusParams: *types.DefaultConsensusParams(),
	}
	stateStore := &smmocks.Store{}
	stateStore.On("LoadValidators", int64(10)).Return(conflictingVals, nil)
	stateStore.On("Load").Return(state, nil)
	blockStore := &mocks.BlockStore{}
	blockStore.On("LoadBlockMeta", int64(10)).Return(&types.BlockMeta{Header: *trustedHeader})
	blockStore.On("LoadBlockCommit", int64(10)).Return(trustedCommit)

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	pool := evidence.NewPool(logger, dbm.NewMemDB(), stateStore, blockStore, evidence.NopMetrics(), eventBus)

	evList := types.EvidenceList{ev}
	err = pool.CheckEvidence(ctx, evList)
	assert.NoError(t, err)

	pendingEvs, _ := pool.PendingEvidence(state.ConsensusParams.Evidence.MaxBytes)
	assert.Equal(t, 1, len(pendingEvs))
}

func TestVerifyLightClientAttack_Amnesia(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	var height int64 = 10
	conflictingVals, conflictingPrivVals := factory.ValidatorSet(ctx, t, 5, 10)

	conflictingHeader := factory.MakeHeader(t, &types.Header{
		ChainID:        evidenceChainID,
		Height:         height,
		Time:           defaultEvidenceTime,
		ValidatorsHash: conflictingVals.Hash(),
	})

	trustedHeader := factory.MakeHeader(t, &types.Header{
		ChainID:            evidenceChainID,
		Height:             height,
		Time:               defaultEvidenceTime,
		ValidatorsHash:     conflictingHeader.ValidatorsHash,
		NextValidatorsHash: conflictingHeader.NextValidatorsHash,
		ConsensusHash:      conflictingHeader.ConsensusHash,
		AppHash:            conflictingHeader.AppHash,
		LastResultsHash:    conflictingHeader.LastResultsHash,
	})

	// we are simulating an amnesia attack where all the validators in the conflictingVals set
	// except the last validator vote twice. However this time the commits are of different rounds.
	blockID := makeBlockID(conflictingHeader.Hash(), 1000, []byte("partshash"))
	voteSet := types.NewExtendedVoteSet(evidenceChainID, height, 0, tmproto.SignedMsgType(2), conflictingVals)
	extCommit, err := factory.MakeExtendedCommit(ctx, blockID, height, 0, voteSet, conflictingPrivVals, defaultEvidenceTime)
	require.NoError(t, err)
	commit := extCommit.ToCommit()

	ev := &types.LightClientAttackEvidence{
		ConflictingBlock: &types.LightBlock{
			SignedHeader: &types.SignedHeader{
				Header: conflictingHeader,
				Commit: commit,
			},
			ValidatorSet: conflictingVals,
		},
		CommonHeight:        height,
		ByzantineValidators: nil, // with amnesia evidence no validators are submitted as abci evidence
		TotalVotingPower:    50,
		Timestamp:           defaultEvidenceTime,
	}

	trustedBlockID := makeBlockID(trustedHeader.Hash(), 1000, []byte("partshash"))
	trustedVoteSet := types.NewExtendedVoteSet(evidenceChainID, height, 1, tmproto.SignedMsgType(2), conflictingVals)
	trustedExtCommit, err := factory.MakeExtendedCommit(ctx, trustedBlockID, height, 1,
		trustedVoteSet, conflictingPrivVals, defaultEvidenceTime)
	require.NoError(t, err)
	trustedCommit := trustedExtCommit.ToCommit()

	trustedSignedHeader := &types.SignedHeader{
		Header: trustedHeader,
		Commit: trustedCommit,
	}

	// good pass -> no error
	require.NoError(t, evidence.VerifyLightClientAttack(ev, trustedSignedHeader, trustedSignedHeader, conflictingVals,
		defaultEvidenceTime.Add(1*time.Minute), 2*time.Hour))

	// trusted and conflicting hashes are the same -> an error should be returned
	assert.Error(t, evidence.VerifyLightClientAttack(ev, trustedSignedHeader, ev.ConflictingBlock.SignedHeader, conflictingVals,
		defaultEvidenceTime.Add(1*time.Minute), 2*time.Hour))

	state := sm.State{
		LastBlockTime:   defaultEvidenceTime.Add(1 * time.Minute),
		LastBlockHeight: 11,
		ConsensusParams: *types.DefaultConsensusParams(),
	}
	stateStore := &smmocks.Store{}
	stateStore.On("LoadValidators", int64(10)).Return(conflictingVals, nil)
	stateStore.On("Load").Return(state, nil)
	blockStore := &mocks.BlockStore{}
	blockStore.On("LoadBlockMeta", int64(10)).Return(&types.BlockMeta{Header: *trustedHeader})
	blockStore.On("LoadBlockCommit", int64(10)).Return(trustedCommit)

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	pool := evidence.NewPool(logger, dbm.NewMemDB(), stateStore, blockStore, evidence.NopMetrics(), eventBus)

	evList := types.EvidenceList{ev}
	err = pool.CheckEvidence(ctx, evList)
	assert.NoError(t, err)

	pendingEvs, _ := pool.PendingEvidence(state.ConsensusParams.Evidence.MaxBytes)
	assert.Equal(t, 1, len(pendingEvs))
}

type voteData struct {
	vote1 *types.Vote
	vote2 *types.Vote
	valid bool
}

func TestVerifyDuplicateVoteEvidence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	val := types.NewMockPV()
	val2 := types.NewMockPV()
	valSet := types.NewValidatorSet([]*types.Validator{val.ExtractIntoValidator(ctx, 1)})

	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	blockID2 := makeBlockID([]byte("blockhash2"), 1000, []byte("partshash"))
	blockID3 := makeBlockID([]byte("blockhash"), 10000, []byte("partshash"))
	blockID4 := makeBlockID([]byte("blockhash"), 10000, []byte("partshash2"))

	const chainID = "mychain"

	vote1 := makeVote(ctx, t, val, chainID, 0, 10, 2, 1, blockID, defaultEvidenceTime)
	v1 := vote1.ToProto()
	err := val.SignVote(ctx, chainID, v1)
	require.NoError(t, err)
	badVote := makeVote(ctx, t, val, chainID, 0, 10, 2, 1, blockID, defaultEvidenceTime)
	bv := badVote.ToProto()
	err = val2.SignVote(ctx, chainID, bv)
	require.NoError(t, err)

	vote1.Signature = v1.Signature
	badVote.Signature = bv.Signature

	cases := []voteData{
		{vote1, makeVote(ctx, t, val, chainID, 0, 10, 2, 1, blockID2, defaultEvidenceTime), true}, // different block ids
		{vote1, makeVote(ctx, t, val, chainID, 0, 10, 2, 1, blockID3, defaultEvidenceTime), true},
		{vote1, makeVote(ctx, t, val, chainID, 0, 10, 2, 1, blockID4, defaultEvidenceTime), true},
		{vote1, makeVote(ctx, t, val, chainID, 0, 10, 2, 1, blockID, defaultEvidenceTime), false},     // wrong block id
		{vote1, makeVote(ctx, t, val, "mychain2", 0, 10, 2, 1, blockID2, defaultEvidenceTime), false}, // wrong chain id
		{vote1, makeVote(ctx, t, val, chainID, 0, 11, 2, 1, blockID2, defaultEvidenceTime), false},    // wrong height
		{vote1, makeVote(ctx, t, val, chainID, 0, 10, 3, 1, blockID2, defaultEvidenceTime), false},    // wrong round
		{vote1, makeVote(ctx, t, val, chainID, 0, 10, 2, 2, blockID2, defaultEvidenceTime), false},    // wrong step
		{vote1, makeVote(ctx, t, val2, chainID, 0, 10, 2, 1, blockID2, defaultEvidenceTime), false},   // wrong validator
		// a different vote time doesn't matter
		{vote1, makeVote(ctx, t, val, chainID, 0, 10, 2, 1, blockID2, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)), true},
		{vote1, badVote, false}, // signed by wrong key
	}

	require.NoError(t, err)
	for _, c := range cases {
		ev := &types.DuplicateVoteEvidence{
			VoteA:            c.vote1,
			VoteB:            c.vote2,
			ValidatorPower:   1,
			TotalVotingPower: 1,
			Timestamp:        defaultEvidenceTime,
		}
		if c.valid {
			assert.Nil(t, evidence.VerifyDuplicateVote(ev, chainID, valSet), "evidence should be valid")
		} else {
			assert.NotNil(t, evidence.VerifyDuplicateVote(ev, chainID, valSet), "evidence should be invalid")
		}
	}

	// create good evidence and correct validator power
	goodEv, err := types.NewMockDuplicateVoteEvidenceWithValidator(ctx, 10, defaultEvidenceTime, val, chainID)
	require.NoError(t, err)
	goodEv.ValidatorPower = 1
	goodEv.TotalVotingPower = 1
	badEv, err := types.NewMockDuplicateVoteEvidenceWithValidator(ctx, 10, defaultEvidenceTime, val, chainID)
	require.NoError(t, err)
	badTimeEv, err := types.NewMockDuplicateVoteEvidenceWithValidator(ctx, 10, defaultEvidenceTime.Add(1*time.Minute), val, chainID)
	require.NoError(t, err)
	badTimeEv.ValidatorPower = 1
	badTimeEv.TotalVotingPower = 1
	state := sm.State{
		ChainID:         chainID,
		LastBlockTime:   defaultEvidenceTime.Add(1 * time.Minute),
		LastBlockHeight: 11,
		ConsensusParams: *types.DefaultConsensusParams(),
	}
	stateStore := &smmocks.Store{}
	stateStore.On("LoadValidators", int64(10)).Return(valSet, nil)
	stateStore.On("Load").Return(state, nil)
	blockStore := &mocks.BlockStore{}
	blockStore.On("LoadBlockMeta", int64(10)).Return(&types.BlockMeta{Header: types.Header{Time: defaultEvidenceTime}})

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	pool := evidence.NewPool(logger, dbm.NewMemDB(), stateStore, blockStore, evidence.NopMetrics(), eventBus)
	startPool(t, pool, stateStore)

	evList := types.EvidenceList{goodEv}
	err = pool.CheckEvidence(ctx, evList)
	assert.NoError(t, err)

	// evidence with a different validator power should fail
	evList = types.EvidenceList{badEv}
	err = pool.CheckEvidence(ctx, evList)
	assert.Error(t, err)

	// evidence with a different timestamp should fail
	evList = types.EvidenceList{badTimeEv}
	err = pool.CheckEvidence(ctx, evList)
	assert.Error(t, err)
}

func makeLunaticEvidence(
	ctx context.Context,
	t *testing.T,
	height, commonHeight int64,
	totalVals, byzVals, phantomVals int,
	commonTime, attackTime time.Time,
) (ev *types.LightClientAttackEvidence, trusted *types.LightBlock, common *types.LightBlock) {
	t.Helper()

	commonValSet, commonPrivVals := factory.ValidatorSet(ctx, t, totalVals, defaultVotingPower)

	require.Greater(t, totalVals, byzVals)

	// extract out the subset of byzantine validators in the common validator set
	byzValSet, byzPrivVals := commonValSet.Validators[:byzVals], commonPrivVals[:byzVals]

	phantomValSet, phantomPrivVals := factory.ValidatorSet(ctx, t, phantomVals, defaultVotingPower)

	conflictingVals := phantomValSet.Copy()
	require.NoError(t, conflictingVals.UpdateWithChangeSet(byzValSet))
	conflictingPrivVals := append(phantomPrivVals, byzPrivVals...)

	conflictingPrivVals = orderPrivValsByValSet(ctx, t, conflictingVals, conflictingPrivVals)

	commonHeader := factory.MakeHeader(t, &types.Header{
		ChainID: evidenceChainID,
		Height:  commonHeight,
		Time:    commonTime,
	})

	trustedHeader := factory.MakeHeader(t, &types.Header{
		ChainID: evidenceChainID,
		Height:  height,
		Time:    defaultEvidenceTime,
	})

	conflictingHeader := factory.MakeHeader(t, &types.Header{
		ChainID:        evidenceChainID,
		Height:         height,
		Time:           attackTime,
		ValidatorsHash: conflictingVals.Hash(),
	})

	blockID := factory.MakeBlockIDWithHash(conflictingHeader.Hash())
	voteSet := types.NewExtendedVoteSet(evidenceChainID, height, 1, tmproto.SignedMsgType(2), conflictingVals)
	extCommit, err := factory.MakeExtendedCommit(ctx, blockID, height, 1, voteSet, conflictingPrivVals, defaultEvidenceTime)
	require.NoError(t, err)
	commit := extCommit.ToCommit()

	ev = &types.LightClientAttackEvidence{
		ConflictingBlock: &types.LightBlock{
			SignedHeader: &types.SignedHeader{
				Header: conflictingHeader,
				Commit: commit,
			},
			ValidatorSet: conflictingVals,
		},
		CommonHeight:        commonHeight,
		TotalVotingPower:    commonValSet.TotalVotingPower(),
		ByzantineValidators: byzValSet,
		Timestamp:           commonTime,
	}

	common = &types.LightBlock{
		SignedHeader: &types.SignedHeader{
			Header: commonHeader,
			// we can leave this empty because we shouldn't be checking this
			Commit: &types.Commit{},
		},
		ValidatorSet: commonValSet,
	}
	trustedBlockID := factory.MakeBlockIDWithHash(trustedHeader.Hash())
	trustedVals, privVals := factory.ValidatorSet(ctx, t, totalVals, defaultVotingPower)
	trustedVoteSet := types.NewExtendedVoteSet(evidenceChainID, height, 1, tmproto.SignedMsgType(2), trustedVals)
	trustedExtCommit, err := factory.MakeExtendedCommit(ctx, trustedBlockID, height, 1, trustedVoteSet, privVals, defaultEvidenceTime)
	require.NoError(t, err)
	trustedCommit := trustedExtCommit.ToCommit()

	trusted = &types.LightBlock{
		SignedHeader: &types.SignedHeader{
			Header: trustedHeader,
			Commit: trustedCommit,
		},
		ValidatorSet: trustedVals,
	}
	return ev, trusted, common
}

func makeVote(
	ctx context.Context,
	t *testing.T, val types.PrivValidator, chainID string, valIndex int32, height int64,
	round int32, step int, blockID types.BlockID, time time.Time,
) *types.Vote {
	pubKey, err := val.GetPubKey(ctx)
	require.NoError(t, err)
	v := &types.Vote{
		ValidatorAddress: pubKey.Address(),
		ValidatorIndex:   valIndex,
		Height:           height,
		Round:            round,
		Type:             tmproto.SignedMsgType(step),
		BlockID:          blockID,
		Timestamp:        time,
	}

	vpb := v.ToProto()
	err = val.SignVote(ctx, chainID, vpb)
	require.NoError(t, err)
	v.Signature = vpb.Signature
	return v
}

func makeBlockID(hash []byte, partSetSize uint32, partSetHash []byte) types.BlockID {
	var (
		h   = make([]byte, crypto.HashSize)
		psH = make([]byte, crypto.HashSize)
	)
	copy(h, hash)
	copy(psH, partSetHash)
	return types.BlockID{
		Hash: h,
		PartSetHeader: types.PartSetHeader{
			Total: partSetSize,
			Hash:  psH,
		},
	}
}

func orderPrivValsByValSet(ctx context.Context, t *testing.T, vals *types.ValidatorSet, privVals []types.PrivValidator) []types.PrivValidator {
	output := make([]types.PrivValidator, len(privVals))
	for idx, v := range vals.Validators {
		for _, p := range privVals {
			pubKey, err := p.GetPubKey(ctx)
			require.NoError(t, err)
			if bytes.Equal(v.Address, pubKey.Address()) {
				output[idx] = p
				break
			}
		}
		require.NotEmpty(t, output[idx])
	}
	return output
}
