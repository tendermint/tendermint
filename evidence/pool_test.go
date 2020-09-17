package evidence_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/evidence"
	"github.com/tendermint/tendermint/evidence/mocks"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	sm "github.com/tendermint/tendermint/state"
	smmocks "github.com/tendermint/tendermint/state/mocks"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

func TestMain(m *testing.M) {

	code := m.Run()
	os.Exit(code)
}

const evidenceChainID = "test_chain"

var defaultEvidenceTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

func TestEvidencePoolBasic(t *testing.T) {
	var (
		height     = int64(1)
		stateStore = &smmocks.Store{}
		evidenceDB = dbm.NewMemDB()
		blockStore = &mocks.BlockStore{}
	)

	valSet, privVals := types.RandValidatorSet(3, 10)

	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
		&types.BlockMeta{Header: types.Header{Time: defaultEvidenceTime}},
	)
	stateStore.On("LoadValidators", mock.AnythingOfType("int64")).Return(valSet, nil)
	stateStore.On("Load").Return(createState(height+1, valSet), nil)

	pool, err := evidence.NewPool(evidenceDB, stateStore, blockStore)
	require.NoError(t, err)

	// evidence not seen yet:
	evs := pool.PendingEvidence(10)
	assert.Equal(t, 0, len(evs))

	ev := types.NewMockDuplicateVoteEvidenceWithValidator(height, defaultEvidenceTime, privVals[0], evidenceChainID)

	// good evidence
	evAdded := make(chan struct{})
	go func() {
		<-pool.EvidenceWaitChan()
		close(evAdded)
	}()

	// evidence seen but not yet committed:
	assert.NoError(t, pool.AddEvidence(ev))

	select {
	case <-evAdded:
	case <-time.After(5 * time.Second):
		t.Fatal("evidence was not added to list after 5s")
	}

	next := pool.EvidenceFront()
	assert.Equal(t, ev, next.Value.(types.Evidence))

	evs = pool.PendingEvidence(10)
	assert.Equal(t, 1, len(evs))

	// shouldn't be able to add evidence twice
	assert.Error(t, pool.AddEvidence(ev))
	assert.Equal(t, 1, len(pool.PendingEvidence(10)))

}

// Tests inbound evidence for the right time and height
func TestAddExpiredEvidence(t *testing.T) {
	var (
		val                 = types.NewMockPV()
		height              = int64(30)
		stateStore          = initializeValidatorState(val, height)
		evidenceDB          = dbm.NewMemDB()
		blockStore          = &mocks.BlockStore{}
		expiredEvidenceTime = time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
		expiredHeight       = int64(2)
	)

	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(func(h int64) *types.BlockMeta {
		if h == height || h == expiredHeight {
			return &types.BlockMeta{Header: types.Header{Time: defaultEvidenceTime.Add(time.Duration(height) * time.Minute)}}
		}
		return &types.BlockMeta{Header: types.Header{Time: expiredEvidenceTime}}
	})

	pool, err := evidence.NewPool(evidenceDB, stateStore, blockStore)
	require.NoError(t, err)

	testCases := []struct {
		evHeight      int64
		evTime        time.Time
		expErr        bool
		evDescription string
	}{
		{height, defaultEvidenceTime, false, "valid evidence"},
		{expiredHeight, defaultEvidenceTime, false, "valid evidence (despite old height)"},
		{height - 1, expiredEvidenceTime, false, "valid evidence (despite old time)"},
		{expiredHeight - 1, expiredEvidenceTime, true,
			"evidence from height 1 (created at: 2019-01-01 00:00:00 +0000 UTC) is too old"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.evDescription, func(t *testing.T) {
			ev := types.NewMockDuplicateVoteEvidenceWithValidator(tc.evHeight, tc.evTime, val, evidenceChainID)
			err := pool.AddEvidence(ev)
			if tc.expErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAddEvidenceFromConsensus(t *testing.T) {
	var height int64 = 10
	pool, val := defaultTestPool(height)
	ev := types.NewMockDuplicateVoteEvidenceWithValidator(height, defaultEvidenceTime, val, evidenceChainID)
	err := pool.AddEvidenceFromConsensus(ev, defaultEvidenceTime,
		types.NewValidatorSet([]*types.Validator{val.ExtractIntoValidator(2)}))
	assert.NoError(t, err)
	next := pool.EvidenceFront()
	assert.Equal(t, ev, next.Value.(types.Evidence))
	// shouldn't be able to submit the same evidence twice
	err = pool.AddEvidenceFromConsensus(ev, defaultEvidenceTime.Add(-1*time.Second),
		types.NewValidatorSet([]*types.Validator{val.ExtractIntoValidator(3)}))
	if assert.Error(t, err) {
		assert.Equal(t, "evidence already verified and added", err.Error())
	}
}

func TestEvidencePoolUpdate(t *testing.T) {
	height := int64(21)
	pool, val := defaultTestPool(height)
	state := pool.State()

	// create new block (no need to save it to blockStore)
	prunedEv := types.NewMockDuplicateVoteEvidenceWithValidator(1, defaultEvidenceTime,
		val, evidenceChainID)
	err := pool.AddEvidence(prunedEv)
	require.NoError(t, err)
	ev := types.NewMockDuplicateVoteEvidenceWithValidator(height, defaultEvidenceTime, val, evidenceChainID)
	lastCommit := makeCommit(height, val.PrivKey.PubKey().Address())
	block := types.MakeBlock(height+1, []types.Tx{}, lastCommit, []types.Evidence{ev})
	// update state (partially)
	state.LastBlockHeight = height + 1
	state.LastBlockTime = defaultEvidenceTime.Add(22 * time.Minute)
	err = pool.CheckEvidence(types.EvidenceList{ev})
	require.NoError(t, err)

	byzVals := pool.ABCIEvidence(block.Height, block.Evidence.Evidence)
	expectedByzVals := []abci.Evidence{
		{
			Type:             abci.EvidenceType_DUPLICATE_VOTE,
			Validator:        types.TM2PB.Validator(val.ExtractIntoValidator(10)),
			Height:           height,
			Time:             defaultEvidenceTime.Add(time.Duration(height) * time.Minute),
			TotalVotingPower: 10,
		},
	}
	assert.Equal(t, expectedByzVals, byzVals)
	assert.Equal(t, 1, len(pool.PendingEvidence(10)))

	pool.Update(state)

	// a) Update marks evidence as committed so pending evidence should be empty
	assert.Empty(t, pool.PendingEvidence(10))

	// b) If we try to check this evidence again it should fail because it has already been committed
	err = pool.CheckEvidence(types.EvidenceList{ev})
	if assert.Error(t, err) {
		assert.Equal(t, "evidence was already committed", err.(*types.ErrInvalidEvidence).Reason.Error())
	}

	assert.Empty(t, pool.ABCIEvidence(height, []types.Evidence{}))
}

func TestVerifyPendingEvidencePasses(t *testing.T) {
	var height int64 = 1
	pool, val := defaultTestPool(height)
	ev := types.NewMockDuplicateVoteEvidenceWithValidator(height, defaultEvidenceTime, val, evidenceChainID)
	err := pool.AddEvidence(ev)
	require.NoError(t, err)

	err = pool.CheckEvidence(types.EvidenceList{ev})
	assert.NoError(t, err)
}

func TestVerifyDuplicatedEvidenceFails(t *testing.T) {
	var height int64 = 1
	pool, val := defaultTestPool(height)
	ev := types.NewMockDuplicateVoteEvidenceWithValidator(height, defaultEvidenceTime, val, evidenceChainID)
	err := pool.CheckEvidence(types.EvidenceList{ev, ev})
	if assert.Error(t, err) {
		assert.Equal(t, "duplicate evidence", err.(*types.ErrInvalidEvidence).Reason.Error())
	}
}

// check that
func TestCheckEvidenceWithLightClientAttack(t *testing.T) {
	nValidators := 5
	conflictingVals, conflictingPrivVals := types.RandValidatorSet(nValidators, 10)
	trustedHeader := makeHeaderRandom(10)

	conflictingHeader := makeHeaderRandom(10)
	conflictingHeader.ValidatorsHash = conflictingVals.Hash()

	trustedHeader.ValidatorsHash = conflictingHeader.ValidatorsHash
	trustedHeader.NextValidatorsHash = conflictingHeader.NextValidatorsHash
	trustedHeader.ConsensusHash = conflictingHeader.ConsensusHash
	trustedHeader.AppHash = conflictingHeader.AppHash
	trustedHeader.LastResultsHash = conflictingHeader.LastResultsHash

	// for simplicity we are simulating a duplicate vote attack where all the validators in the
	// conflictingVals set voted twice
	blockID := makeBlockID(conflictingHeader.Hash(), 1000, []byte("partshash"))
	voteSet := types.NewVoteSet(evidenceChainID, 10, 1, tmproto.SignedMsgType(2), conflictingVals)
	commit, err := types.MakeCommit(blockID, 10, 1, voteSet, conflictingPrivVals, defaultEvidenceTime)
	require.NoError(t, err)
	ev := &types.LightClientAttackEvidence{
		ConflictingBlock: &types.LightBlock{
			SignedHeader: &types.SignedHeader{
				Header: conflictingHeader,
				Commit: commit,
			},
			ValidatorSet: conflictingVals,
		},
		CommonHeight: 10,
	}

	trustedBlockID := makeBlockID(trustedHeader.Hash(), 1000, []byte("partshash"))
	trustedVoteSet := types.NewVoteSet(evidenceChainID, 10, 1, tmproto.SignedMsgType(2), conflictingVals)
	trustedCommit, err := types.MakeCommit(trustedBlockID, 10, 1, trustedVoteSet, conflictingPrivVals, defaultEvidenceTime)
	require.NoError(t, err)

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

	pool, err := evidence.NewPool(dbm.NewMemDB(), stateStore, blockStore)
	require.NoError(t, err)
	pool.SetLogger(log.TestingLogger())

	err = pool.AddEvidence(ev)
	assert.NoError(t, err)

	err = pool.CheckEvidence(types.EvidenceList{ev})
	assert.NoError(t, err)

	// take away the last signature -> there are less validators then what we have detected,
	// hence we move to full verification where the evidence should still pass
	commit.Signatures = append(commit.Signatures[:nValidators-1], types.NewCommitSigAbsent())
	err = pool.CheckEvidence(types.EvidenceList{ev})
	assert.NoError(t, err)

	// take away the last two signatures -> should fail due to insufficient power
	commit.Signatures = append(commit.Signatures[:nValidators-2], types.NewCommitSigAbsent(), types.NewCommitSigAbsent())
	err = pool.CheckEvidence(types.EvidenceList{ev})
	assert.Error(t, err)
}

func TestRecoverPendingEvidence(t *testing.T) {
	height := int64(10)
	expiredEvidenceTime := time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	val := types.NewMockPV()
	valAddress := val.PrivKey.PubKey().Address()
	evidenceDB := dbm.NewMemDB()
	stateStore := initializeValidatorState(val, height)
	state, err := stateStore.Load()
	require.NoError(t, err)
	blockStore := initializeBlockStore(dbm.NewMemDB(), state, valAddress)
	pool, err := evidence.NewPool(evidenceDB, stateStore, blockStore)
	require.NoError(t, err)
	pool.SetLogger(log.TestingLogger())
	goodEvidence := types.NewMockDuplicateVoteEvidenceWithValidator(height,
		defaultEvidenceTime, val, evidenceChainID)
	expiredEvidence := types.NewMockDuplicateVoteEvidenceWithValidator(int64(1),
		expiredEvidenceTime, val, evidenceChainID)
	err = pool.AddEvidence(goodEvidence)
	require.NoError(t, err)
	err = pool.AddEvidence(expiredEvidence)
	require.NoError(t, err)
	newStateStore := &smmocks.Store{}
	newStateStore.On("Load").Return(sm.State{
		LastBlockTime:   defaultEvidenceTime.Add(49 * time.Hour),
		LastBlockHeight: height + 12,
		ConsensusParams: tmproto.ConsensusParams{
			Block: tmproto.BlockParams{
				MaxBytes: 22020096,
				MaxGas:   -1,
			},
			Evidence: tmproto.EvidenceParams{
				MaxAgeNumBlocks: 20,
				MaxAgeDuration:  1 * time.Hour,
				MaxNum:          50,
			},
		},
	}, nil)
	newPool, err := evidence.NewPool(evidenceDB, newStateStore, blockStore)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(newPool.PendingEvidence(10)))
	next := newPool.EvidenceFront()
	assert.Equal(t, goodEvidence, next.Value.(types.Evidence))

}

func initializeStateFromValidatorSet(valSet *types.ValidatorSet, height int64) sm.Store {
	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
	state := sm.State{
		ChainID:                     evidenceChainID,
		InitialHeight:               1,
		LastBlockHeight:             height,
		LastBlockTime:               defaultEvidenceTime,
		Validators:                  valSet,
		NextValidators:              valSet.CopyIncrementProposerPriority(1),
		LastValidators:              valSet,
		LastHeightValidatorsChanged: 1,
		ConsensusParams: tmproto.ConsensusParams{
			Block: tmproto.BlockParams{
				MaxBytes: 22020096,
				MaxGas:   -1,
			},
			Evidence: tmproto.EvidenceParams{
				MaxAgeNumBlocks: 20,
				MaxAgeDuration:  20 * time.Minute,
				MaxNum:          50,
			},
		},
	}

	// save all states up to height
	for i := int64(0); i <= height; i++ {
		state.LastBlockHeight = i
		if err := stateStore.Save(state); err != nil {
			panic(err)
		}
	}

	return stateStore
}

func initializeValidatorState(privVal types.PrivValidator, height int64) sm.Store {

	pubKey, _ := privVal.GetPubKey()
	validator := &types.Validator{Address: pubKey.Address(), VotingPower: 10, PubKey: pubKey}

	// create validator set and state
	valSet := &types.ValidatorSet{
		Validators: []*types.Validator{validator},
		Proposer:   validator,
	}

	return initializeStateFromValidatorSet(valSet, height)
}

// initializeBlockStore creates a block storage and populates it w/ a dummy
// block at +height+.
func initializeBlockStore(db dbm.DB, state sm.State, valAddr []byte) *store.BlockStore {
	blockStore := store.NewBlockStore(db)

	for i := int64(1); i <= state.LastBlockHeight; i++ {
		lastCommit := makeCommit(i-1, valAddr)
		block, _ := state.MakeBlock(i, []types.Tx{}, lastCommit, nil,
			state.Validators.GetProposer().Address)
		block.Header.Time = defaultEvidenceTime.Add(time.Duration(i) * time.Minute)
		block.Header.Version = tmversion.Consensus{Block: version.BlockProtocol, App: 1}
		const parts = 1
		partSet := block.MakePartSet(parts)

		seenCommit := makeCommit(i, valAddr)
		blockStore.SaveBlock(block, partSet, seenCommit)
	}

	return blockStore
}

func makeCommit(height int64, valAddr []byte) *types.Commit {
	commitSigs := []types.CommitSig{{
		BlockIDFlag:      types.BlockIDFlagCommit,
		ValidatorAddress: valAddr,
		Timestamp:        defaultEvidenceTime,
		Signature:        []byte("Signature"),
	}}
	return types.NewCommit(height, 0, types.BlockID{}, commitSigs)
}

func defaultTestPool(height int64) (*evidence.Pool, types.MockPV) {
	val := types.NewMockPV()
	valAddress := val.PrivKey.PubKey().Address()
	evidenceDB := dbm.NewMemDB()
	stateStore := initializeValidatorState(val, height)
	state, _ := stateStore.Load()
	blockStore := initializeBlockStore(dbm.NewMemDB(), state, valAddress)
	pool, err := evidence.NewPool(evidenceDB, stateStore, blockStore)
	if err != nil {
		panic("test evidence pool could not be created")
	}
	pool.SetLogger(log.TestingLogger())
	return pool, val
}

func createState(height int64, valSet *types.ValidatorSet) sm.State {
	return sm.State{
		ChainID:         evidenceChainID,
		LastBlockHeight: height,
		LastBlockTime:   defaultEvidenceTime,
		Validators:      valSet,
		ConsensusParams: *types.DefaultConsensusParams(),
	}
}
