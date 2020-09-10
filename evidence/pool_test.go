package evidence_test

import (
	"os"
	"testing"
	"time"

	// "github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/evidence"
	"github.com/tendermint/tendermint/evidence/mocks"
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
	stateStore.On("LoadState").Return(createState(height + 1, valSet))

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
}

// Tests inbound evidence for the right time and height
// func TestAddExpiredEvidence(t *testing.T) {
// 	var (
// 		val                 = types.NewMockPV()
// 		height              = int64(30)
// 		stateStore          = initializeValidatorState(val, height)
// 		evidenceDB          = dbm.NewMemDB()
// 		blockStore          = &mocks.BlockStore{}
// 		expiredEvidenceTime = time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
// 		expiredHeight       = int64(2)
// 	)

// 	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(func(h int64) *types.BlockMeta {
// 		if h == height || h == expiredHeight {
// 			return &types.BlockMeta{Header: types.Header{Time: defaultEvidenceTime}}
// 		}
// 		return &types.BlockMeta{Header: types.Header{Time: expiredEvidenceTime}}
// 	})

// 	pool, err := NewPool(evidenceDB, stateStore, blockStore)
// 	require.NoError(t, err)

// 	testCases := []struct {
// 		evHeight      int64
// 		evTime        time.Time
// 		expErr        bool
// 		evDescription string
// 	}{
// 		{height, defaultEvidenceTime, false, "valid evidence"},
// 		{expiredHeight, defaultEvidenceTime, false, "valid evidence (despite old height)"},
// 		{height - 1, expiredEvidenceTime, false, "valid evidence (despite old time)"},
// 		{expiredHeight - 1, expiredEvidenceTime, true,
// 			"evidence from height 1 (created at: 2019-01-01 00:00:00 +0000 UTC) is too old"},
// 	}

// 	for _, tc := range testCases {
// 		tc := tc
// 		t.Run(tc.evDescription, func(t *testing.T) {
// 			ev := types.NewMockDuplicateVoteEvidenceWithValidator(tc.evHeight, tc.evTime, val, evidenceChainID)
// 			err := pool.AddEvidence(ev)
// 			if tc.expErr {
// 				assert.Error(t, err)
// 			} else {
// 				assert.NoError(t, err)
// 			}
// 		})
// 	}
// }

// func TestEvidencePoolUpdate(t *testing.T) {
// 	height := int64(21)

// 	pool, val := defaultTestPool(height)

// 	state := pool.State()

// 	// create new block (no need to save it to blockStore)
// 	evidence := types.NewMockDuplicateVoteEvidence(height, time.Now(), evidenceChainID)
// 	lastCommit := makeCommit(height, val.PrivKey.PubKey().Address())
// 	block := types.MakeBlock(height+1, []types.Tx{}, lastCommit, []types.Evidence{evidence})
// 	// update state (partially)
// 	state.LastBlockHeight = height + 1

// 	pool.Update(block, state)

// 	// a) Update marks evidence as committed
// 	assert.True(t, pool.IsCommitted(evidence))
// }

// func TestVerifyEvidenceCommittedEvidenceFails(t *testing.T) {
// 	height := int64(1)
// 	pool, _ := defaultTestPool(height)
// 	committedEvidence := types.NewMockDuplicateVoteEvidence(height, time.Now(), evidenceChainID)
// 	pool.MarkEvidenceAsCommitted(height, []types.Evidence{committedEvidence})

// 	err := pool.Verify(committedEvidence)
// 	if assert.Error(t, err) {
// 		assert.Equal(t, "evidence was already committed", err.Error())
// 	}
// }

// func TestVeriyEvidencePendingEvidencePasses(t *testing.T) {
// 	var (
// 		val        = types.NewMockPV()
// 		height     = int64(1)
// 		stateStore = initializeValidatorState(val, height)
// 		blockStore = &mocks.BlockStore{}
// 	)

// 	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
// 		&types.BlockMeta{Header: types.Header{Time: defaultEvidenceTime}},
// 	)

// 	pool, err := NewPool(dbm.NewMemDB(), stateStore, blockStore)
// 	require.NoError(t, err)
// 	evidence := types.NewMockDuplicateVoteEvidenceWithValidator(height, defaultEvidenceTime, val, evidenceChainID)
// 	err = pool.AddEvidence(evidence)
// 	require.NoError(t, err)

// 	err = pool.Verify(evidence)
// 	assert.NoError(t, err)
// }

// func TestRecoverPendingEvidence(t *testing.T) {
// 	var (
// 		val                 = types.NewMockPV()
// 		valAddr             = val.PrivKey.PubKey().Address()
// 		height              = int64(30)
// 		stateStore          = initializeValidatorState(val, height)
// 		evidenceDB          = dbm.NewMemDB()
// 		blockStoreDB        = dbm.NewMemDB()
// 		state               = stateStore.LoadState()
// 		blockStore          = initializeBlockStore(blockStoreDB, state, valAddr)
// 		expiredEvidenceTime = time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
// 		goodEvidence        = types.NewMockDuplicateVoteEvidenceWithValidator(height,
// 			defaultEvidenceTime, val, evidenceChainID)
// 		expiredEvidence = types.NewMockDuplicateVoteEvidenceWithValidator(int64(1),
// 			expiredEvidenceTime, val, evidenceChainID)
// 	)

// 	// load good evidence
// 	goodKey := keyPending(goodEvidence)
// 	evi, err := types.EvidenceToProto(goodEvidence)
// 	require.NoError(t, err)
// 	goodEvidenceBytes, err := proto.Marshal(evi)
// 	require.NoError(t, err)
// 	_ = evidenceDB.Set(goodKey, goodEvidenceBytes)

// 	// load expired evidence
// 	expiredKey := keyPending(expiredEvidence)
// 	eevi, err := types.EvidenceToProto(expiredEvidence)
// 	require.NoError(t, err)

// 	expiredEvidenceBytes, err := proto.Marshal(eevi)
// 	require.NoError(t, err)

// 	_ = evidenceDB.Set(expiredKey, expiredEvidenceBytes)
// 	pool, err := NewPool(evidenceDB, stateStore, blockStore)
// 	require.NoError(t, err)
// 	assert.Equal(t, 1, pool.evidenceList.Len())
// 	assert.True(t, pool.IsPending(goodEvidence))
// 	assert.False(t, pool.Has(expiredEvidence))
// }

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
				MaxAgeDuration:  48 * time.Hour,
				MaxNum:          50,
			},
		},
	}

	// save all states up to height
	for i := int64(0); i <= height; i++ {
		state.LastBlockHeight = i
		sm.SaveState(stateDB, state)
	}

	return &stateStore{db: stateDB}
}

func initializeValidatorState(privVal types.PrivValidator, height int64) evidence.StateStore {

	pubKey, _ := privVal.GetPubKey()
	validator := &types.Validator{Address: pubKey.Address(), VotingPower: 0, PubKey: pubKey}

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
		Timestamp:        time.Now(),
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
	return pool, val
}

func createState(height int64, valSet *types.ValidatorSet) sm.State {
	return sm.State{
		ChainID: evidenceChainID,
		LastBlockHeight: height,
		LastBlockTime: defaultEvidenceTime,
		Validators: valSet,
		ConsensusParams: *types.DefaultConsensusParams(),
	}
}

type stateStore struct {
	db dbm.DB
}

var _ evidence.StateStore = &stateStore{}

// This is a temporary measure until stateDB becomes a store
// TODO: deprecate once state has a store
func NewEvidenceStateStore(db dbm.DB) evidence.StateStore {
	return &stateStore{db}
}

func (s *stateStore) LoadValidators(height int64) (*types.ValidatorSet, error) {
	return sm.LoadValidators(s.db, height)
}

func (s *stateStore) LoadState() sm.State {
	return sm.LoadState(s.db)
}

