package evidence

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	tmproto "github.com/tendermint/tendermint/proto/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

func TestMain(m *testing.M) {
	RegisterMockEvidences()

	code := m.Run()
	os.Exit(code)
}

func TestEvidencePool(t *testing.T) {
	var (
		valAddr      = []byte("val1")
		height       = int64(52)
		stateDB      = initializeValidatorState(valAddr, height)
		evidenceDB   = dbm.NewMemDB()
		blockStoreDB = dbm.NewMemDB()
		blockStore   = initializeBlockStore(blockStoreDB, sm.LoadState(stateDB), valAddr)
		evidenceTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

		goodEvidence = types.NewMockEvidence(height, time.Now(), valAddr)
		badEvidence  = types.NewMockEvidence(1, evidenceTime, valAddr)
	)

	pool, err := NewPool(stateDB, evidenceDB, blockStore)
	require.NoError(t, err)

	// bad evidence
	err = pool.AddEvidence(badEvidence)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "is too old; min height is 32 and evidence can not be older than")
	}
	assert.False(t, pool.IsPending(badEvidence))
	assert.True(t, pool.IsEvidenceExpired(badEvidence))

	// good evidence
	evAdded := make(chan struct{})
	go func() {
		<-pool.EvidenceWaitChan()
		close(evAdded)
	}()

	err = pool.AddEvidence(goodEvidence)
	require.NoError(t, err)

	select {
	case <-evAdded:
	case <-time.After(5 * time.Second):
		t.Fatal("evidence was not added to list after 5s")
	}

	assert.Equal(t, 1, pool.evidenceList.Len())

	// if we send it again, it shouldnt add and return an error
	err = pool.AddEvidence(goodEvidence)
	assert.NoError(t, err)
	assert.Equal(t, 1, pool.evidenceList.Len())
}

func TestProposingAndCommittingEvidence(t *testing.T) {
	var (
		valAddr       = []byte("validator_address")
		height        = int64(1)
		lastBlockTime = time.Now()
		stateDB       = initializeValidatorState(valAddr, height)
		evidenceDB    = dbm.NewMemDB()
		blockStoreDB  = dbm.NewMemDB()
		blockStore    = initializeBlockStore(blockStoreDB, sm.LoadState(stateDB), valAddr)
		evidenceTime  = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	)

	pool, err := NewPool(stateDB, evidenceDB, blockStore)
	require.NoError(t, err)

	// evidence not seen yet:
	evidence := types.NewMockEvidence(height, evidenceTime, valAddr)
	assert.False(t, pool.IsCommitted(evidence))

	// evidence seen but not yet committed:
	assert.NoError(t, pool.AddEvidence(evidence))
	assert.False(t, pool.IsCommitted(evidence))

	// test evidence is proposed
	proposedEvidence := pool.AllPendingEvidence()
	assert.Equal(t, proposedEvidence[0], evidence)

	// evidence seen and committed:
	pool.MarkEvidenceAsCommitted(height, lastBlockTime, proposedEvidence)
	assert.True(t, pool.IsCommitted(evidence))
	assert.False(t, pool.IsPending(evidence))
	assert.Equal(t, 0, pool.evidenceList.Len())

	// evidence should
}

func TestAddEvidence(t *testing.T) {
	var (
		valAddr      = []byte("val1")
		height       = int64(30)
		stateDB      = initializeValidatorState(valAddr, height)
		evidenceDB   = dbm.NewMemDB()
		blockStoreDB = dbm.NewMemDB()
		blockStore   = initializeBlockStore(blockStoreDB, sm.LoadState(stateDB), valAddr)
		evidenceTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	)

	pool, err := NewPool(stateDB, evidenceDB, blockStore)
	require.NoError(t, err)

	testCases := []struct {
		evHeight      int64
		evTime        time.Time
		expErr        bool
		evDescription string
	}{
		{height, time.Now(), false, "valid evidence"},
		{height, evidenceTime, false, "valid evidence (despite old time)"},
		{int64(1), time.Now(), false, "valid evidence (despite old height)"},
		{int64(1), evidenceTime, true,
			"evidence from height 1 (created at: 2019-01-01 00:00:00 +0000 UTC) is too old"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.evDescription, func(t *testing.T) {
			ev := types.NewMockEvidence(tc.evHeight, tc.evTime, valAddr)
			err := pool.AddEvidence(ev)
			if tc.expErr {
				assert.Error(t, err)
				t.Log(err)
			}
		})
	}
}

func TestEvidencePoolUpdate(t *testing.T) {
	var (
		valAddr      = []byte("validator_address")
		height       = int64(21)
		stateDB      = initializeValidatorState(valAddr, height)
		evidenceDB   = dbm.NewMemDB()
		blockStoreDB = dbm.NewMemDB()
		state        = sm.LoadState(stateDB)
		blockStore   = initializeBlockStore(blockStoreDB, state, valAddr)
		evidenceTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	)

	pool, err := NewPool(stateDB, evidenceDB, blockStore)
	require.NoError(t, err)
	expiredEvidence := types.NewMockEvidence(1, evidenceTime, valAddr)
	err = pool.AddEvidence(expiredEvidence)
	require.NoError(t, err)

	// create new block (no need to save it to blockStore)
	evidence := types.NewMockEvidence(height, time.Now(), valAddr)
	lastCommit := makeCommit(height, valAddr)
	block := types.MakeBlock(height+1, []types.Tx{}, lastCommit, []types.Evidence{evidence})
	// update state (partially)
	state.LastBlockHeight = height + 1

	pool.Update(block, state)

	// a) Update marks evidence as committed
	assert.True(t, pool.IsCommitted(evidence))
	// b) Update updates valToLastHeight map
	assert.Equal(t, height+1, pool.ValidatorLastHeight(valAddr))
	// c) Expired ecvidence should be removed
	assert.False(t, pool.IsPending(expiredEvidence))
}

func TestEvidencePoolNewPool(t *testing.T) {
	var (
		valAddr      = []byte("validator_address")
		height       = int64(1)
		stateDB      = initializeValidatorState(valAddr, height)
		evidenceDB   = dbm.NewMemDB()
		blockStoreDB = dbm.NewMemDB()
		state        = sm.LoadState(stateDB)
		blockStore   = initializeBlockStore(blockStoreDB, state, valAddr)
	)

	pool, err := NewPool(stateDB, evidenceDB, blockStore)
	require.NoError(t, err)

	assert.Equal(t, height, pool.ValidatorLastHeight(valAddr))
	assert.EqualValues(t, 0, pool.ValidatorLastHeight([]byte("non-existent-validator")))
}

func TestAddingAndPruningPOLC(t *testing.T) {
	var (
		valAddr      = []byte("validator_address")
		stateDB      = initializeValidatorState(valAddr, 1)
		evidenceDB   = dbm.NewMemDB()
		blockStoreDB = dbm.NewMemDB()
		state        = sm.LoadState(stateDB)
		blockStore   = initializeBlockStore(blockStoreDB, state, valAddr)
		height       = state.ConsensusParams.Evidence.MaxAgeNumBlocks * 2
		evidenceTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	)

	pubKey, _ := types.NewMockPV().GetPubKey()
	polc := types.NewMockPOLC(1, evidenceTime, pubKey)

	pool, err := NewPool(stateDB, evidenceDB, blockStore)
	require.NoError(t, err)

	err = pool.AddPOLC(polc)
	assert.NoError(t, err)

	// should be able to retrieve polc
	newPolc, err := pool.RetrievePOLC(1, 1)
	assert.NoError(t, err)
	assert.True(t, polc.Equal(newPolc))

	// should not be able to retrieve
	emptyPolc, err := pool.RetrievePOLC(2, 1)
	assert.Error(t, err)
	assert.Equal(t, types.ProofOfLockChange{}, emptyPolc)

	lastCommit := makeCommit(height-1, valAddr)
	block := types.MakeBlock(height, []types.Tx{}, lastCommit, []types.Evidence{})
	// update state (partially)
	state.LastBlockHeight = height
	pool.state.LastBlockHeight = height

	// update should prune the polc
	pool.Update(block, state)

	emptyPolc, err = pool.RetrievePOLC(1, 1)
	if assert.Error(t, err) {
		assert.Equal(t, "unable to find polc at height 1 and round 1", err.Error())
	}
	assert.Equal(t, types.ProofOfLockChange{}, emptyPolc)

}

func TestRecoverPendingEvidence(t *testing.T) {
	var (
		valAddr         = []byte("val1")
		height          = int64(30)
		stateDB         = initializeValidatorState(valAddr, height)
		evidenceDB      = dbm.NewMemDB()
		blockStoreDB    = dbm.NewMemDB()
		state           = sm.LoadState(stateDB)
		blockStore      = initializeBlockStore(blockStoreDB, state, valAddr)
		evidenceTime    = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
		goodEvidence    = types.NewMockEvidence(height, time.Now(), valAddr)
		expiredEvidence = types.NewMockEvidence(int64(1), evidenceTime, valAddr)
	)

	// load good evidence
	goodKey := keyPending(goodEvidence)
	goodEvidenceBytes := cdc.MustMarshalBinaryBare(goodEvidence)
	_ = evidenceDB.Set(goodKey, goodEvidenceBytes)

	// load expired evidence
	expiredKey := keyPending(expiredEvidence)
	expiredEvidenceBytes := cdc.MustMarshalBinaryBare(expiredEvidence)
	_ = evidenceDB.Set(expiredKey, expiredEvidenceBytes)
	pool, err := NewPool(stateDB, evidenceDB, blockStore)
	require.NoError(t, err)
	assert.Equal(t, 1, pool.evidenceList.Len())
	assert.True(t, pool.IsPending(goodEvidence))
}

func initializeValidatorState(valAddr []byte, height int64) dbm.DB {
	stateDB := dbm.NewMemDB()

	// create validator set and state
	valSet := &types.ValidatorSet{
		Validators: []*types.Validator{
			{Address: valAddr, VotingPower: 0},
		},
	}
	state := sm.State{
		LastBlockHeight:             height,
		LastBlockTime:               tmtime.Now(),
		LastValidators:              valSet,
		Validators:                  valSet,
		NextValidators:              valSet.CopyIncrementProposerPriority(1),
		LastHeightValidatorsChanged: 1,
		ConsensusParams: tmproto.ConsensusParams{
			Block: tmproto.BlockParams{
				MaxBytes: 22020096,
				MaxGas:   -1,
			},
			Evidence: tmproto.EvidenceParams{
				MaxAgeNumBlocks: 20,
				MaxAgeDuration:  48 * time.Hour,
			},
		},
	}

	// save all states up to height
	for i := int64(0); i <= height; i++ {
		state.LastBlockHeight = i
		sm.SaveState(stateDB, state)
	}

	return stateDB
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
