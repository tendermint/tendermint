package evidence

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/libs/bytes"
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

const evidenceChainID = "test_chain"

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
		valAddr      = []byte("validator_address")
		height       = int64(1)
		stateDB      = initializeValidatorState(valAddr, height)
		evidenceDB   = dbm.NewMemDB()
		blockStoreDB = dbm.NewMemDB()
		blockStore   = initializeBlockStore(blockStoreDB, sm.LoadState(stateDB), valAddr)
		evidenceTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
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
	pool.MarkEvidenceAsCommitted(height, proposedEvidence)
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
	)

	pool, err := NewPool(stateDB, evidenceDB, blockStore)
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
	newPolc, exists := pool.RetrievePOLC(1, 1)
	assert.True(t, exists)
	assert.True(t, polc.Equal(newPolc))

	// should not be able to retrieve
	emptyPolc, exists := pool.RetrievePOLC(2, 1)
	assert.False(t, exists)
	assert.Equal(t, types.ProofOfLockChange{}, emptyPolc)

	lastCommit := makeCommit(height-1, valAddr)
	block := types.MakeBlock(height, []types.Tx{}, lastCommit, []types.Evidence{})
	// update state (partially)
	state.LastBlockHeight = height
	pool.state.LastBlockHeight = height

	// update should prune the polc
	pool.Update(block, state)

	emptyPolc, exists = pool.RetrievePOLC(1, 1)
	assert.False(t, exists)
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

func TestPotentialAmnesiaEvidence(t *testing.T) {
	var (
		val    = types.NewMockPV()
		pubKey = val.PrivKey.PubKey()
		valSet = &types.ValidatorSet{
			Validators: []*types.Validator{
				val.ExtractIntoValidator(0),
			},
		}
		height       = int64(30)
		stateDB      = initializeStateFromValidatorSet(valSet, height)
		evidenceDB   = dbm.NewMemDB()
		blockStoreDB = dbm.NewMemDB()
		state        = sm.LoadState(stateDB)
		blockStore   = initializeBlockStore(blockStoreDB, state, pubKey.Address())
		//evidenceTime    = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
		firstBlockID = types.BlockID{
			Hash: []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			PartsHeader: types.PartSetHeader{
				Total: 1,
				Hash:  []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			},
		}
		secondBlockID = types.BlockID{
			Hash: []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
			PartsHeader: types.PartSetHeader{
				Total: 1,
				Hash:  []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
			},
		}
		evidenceTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	)

	pool, err := NewPool(stateDB, evidenceDB, blockStore)
	require.NoError(t, err)

	polc := types.NewMockPOLC(25, evidenceTime, pubKey)
	err = pool.AddPOLC(polc)
	require.NoError(t, err)

	_, exists := pool.RetrievePOLC(25, 1)
	require.True(t, exists)

	voteA := makeVote(25, 0, 0, pubKey.Address(), firstBlockID)
	err = val.SignVote(evidenceChainID, voteA)
	require.NoError(t, err)
	voteB := makeVote(25, 1, 0, pubKey.Address(), secondBlockID)
	err = val.SignVote(evidenceChainID, voteB)
	require.NoError(t, err)
	ev := types.PotentialAmnesiaEvidence{
		VoteA: voteA,
		VoteB: voteB,
	}
	err = pool.AddEvidence(ev)
	assert.NoError(t, err)

	// evidence requires trial period until it is available -> we expect no evidence to be returned
	assert.Equal(t, 0, len(pool.PendingEvidence(1)))

	nextHeight := pool.nextEvidenceTrialEndedHeight
	assert.Greater(t, nextHeight, int64(0))

	// evidence is not ready to be upgraded so we return the height we expect the evidence to be.
	nextHeight = pool.upgradePotentialAmnesiaEvidence()
	assert.Equal(t, height+pool.state.ConsensusParams.Evidence.ProofTrialPeriod, nextHeight)

	// now evidence is ready to be upgraded to amnesia evidence -> we expect -1 to be the next height as their is
	// no more pending potential amnesia evidence left
	pool.state.LastBlockHeight = nextHeight
	nextHeight = pool.upgradePotentialAmnesiaEvidence()
	assert.Equal(t, int64(-1), nextHeight)

	assert.Equal(t, 1, len(pool.PendingEvidence(1)))

	// evidence of voting back in the past which is instantly punishable -> amnesia evidence is made directly
	ev2 := types.PotentialAmnesiaEvidence{
		VoteA: voteB,
		VoteB: voteA,
	}
	err = pool.AddEvidence(ev2)
	assert.NoError(t, err)

	assert.Equal(t, 2, len(pool.AllPendingEvidence()))

}

func initializeStateFromValidatorSet(valSet *types.ValidatorSet, height int64) dbm.DB {
	stateDB := dbm.NewMemDB()
	state := sm.State{
		ChainID:                     evidenceChainID,
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
				MaxAgeNumBlocks:  20,
				MaxAgeDuration:   48 * time.Hour,
				ProofTrialPeriod: 1,
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

func initializeValidatorState(valAddr []byte, height int64) dbm.DB {

	// create validator set and state
	valSet := &types.ValidatorSet{
		Validators: []*types.Validator{
			{Address: valAddr, VotingPower: 0},
		},
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

func makeVote(height int64, round, index int32, addr bytes.HexBytes, blockID types.BlockID) *types.Vote {
	return &types.Vote{
		Type:             tmproto.SignedMsgType(2),
		Height:           height,
		Round:            round,
		BlockID:          blockID,
		Timestamp:        time.Now(),
		ValidatorAddress: addr,
		ValidatorIndex:   index,
	}
}
