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
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
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
		stateStore = &mocks.StateStore{}
		evidenceDB = dbm.NewMemDB()
		blockStore = &mocks.BlockStore{}
	)

	valSet, privVals := types.RandValidatorSet(3, 10)

	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
		&types.BlockMeta{Header: types.Header{Time: defaultEvidenceTime}},
	)
	stateStore.On("LoadValidators", mock.AnythingOfType("int64")).Return(valSet, nil)
	stateStore.On("LoadState").Return(createState(height+1, valSet))

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
	assert.Error(t, err)
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
	assert.Error(t, err)

	assert.Empty(t, pool.ABCIEvidence(height, []types.Evidence{}))
}

func TestVerifyEvidencePendingEvidencePasses(t *testing.T) {
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
	assert.Error(t, err)
}

func TestRecoverPendingEvidence(t *testing.T) {
	height := int64(10)
	expiredEvidenceTime := time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	val := types.NewMockPV()
	valAddress := val.PrivKey.PubKey().Address()
	evidenceDB := dbm.NewMemDB()
	stateStore := initializeValidatorState(val, height)
	blockStore := initializeBlockStore(dbm.NewMemDB(), stateStore.LoadState(), valAddress)
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
	newStateStore := &mocks.StateStore{}
	newStateStore.On("LoadState").Return(sm.State{
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
	})
	newPool, err := evidence.NewPool(evidenceDB, newStateStore, blockStore)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(newPool.PendingEvidence(10)))
	next := newPool.EvidenceFront()
	assert.Equal(t, goodEvidence, next.Value.(types.Evidence))

}

func initializeStateFromValidatorSet(valSet *types.ValidatorSet, height int64) evidence.StateStore {
	stateDB := dbm.NewMemDB()
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
		state.LastBlockTime = defaultEvidenceTime.Add(time.Duration(i) * time.Minute)
		sm.SaveState(stateDB, state)
	}

	return evidence.NewEvidenceStateStore(stateDB)
}

func initializeValidatorState(privVal types.PrivValidator, height int64) evidence.StateStore {

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
	blockStore := initializeBlockStore(dbm.NewMemDB(), stateStore.LoadState(), valAddress)
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

func TestEvidenceInfo(t *testing.T) {
	val := types.NewMockPV()
	evInfo := evidence.Info{
		Evidence:         types.NewMockDuplicateVoteEvidence(1, defaultEvidenceTime, evidenceChainID),
		Time:             defaultEvidenceTime,
		Validators:       []*types.Validator{val.ExtractIntoValidator(10)},
		TotalVotingPower: 10,
	}

	proto, err := evInfo.ToProto()
	assert.NoError(t, err)
	assert.NotNil(t, proto)

	evInfoConverted, err := evidence.InfoFromProto(proto)
	assert.NoError(t, err)
	assert.Equal(t, evInfo, evInfoConverted)

}
