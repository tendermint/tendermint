package evidence

import (
	"os"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/evidence/mocks"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
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
		val        = types.NewMockPV()
		height     = int64(1)
		stateStore = initializeValidatorState(val, height)
		evidenceDB = dbm.NewMemDB()
		blockStore = &mocks.BlockStore{}
	)

	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
		&types.BlockMeta{Header: types.Header{Time: defaultEvidenceTime}},
	)

	pool, err := NewPool(evidenceDB, stateStore, blockStore)
	require.NoError(t, err)

	// evidence not seen yet:
	evidence := types.NewMockDuplicateVoteEvidenceWithValidator(height, defaultEvidenceTime, val, evidenceChainID)
	assert.False(t, pool.IsCommitted(evidence))

	// good evidence
	evAdded := make(chan struct{})
	go func() {
		<-pool.EvidenceWaitChan()
		close(evAdded)
	}()

	// evidence seen but not yet committed:
	assert.NoError(t, pool.AddEvidence(evidence))

	select {
	case <-evAdded:
	case <-time.After(5 * time.Second):
		t.Fatal("evidence was not added to list after 5s")
	}

	assert.Equal(t, 1, pool.evidenceList.Len())

	assert.False(t, pool.IsCommitted(evidence))
	assert.True(t, pool.IsPending(evidence))

	// test evidence is proposed
	proposedEvidence := pool.AllPendingEvidence()
	assert.Equal(t, proposedEvidence[0], evidence)

	proposedEvidence = pool.PendingEvidence(1)
	assert.Equal(t, proposedEvidence[0], evidence)

	// evidence seen and committed:
	pool.MarkEvidenceAsCommitted(height, proposedEvidence)
	assert.True(t, pool.IsCommitted(evidence))
	assert.False(t, pool.IsPending(evidence))
	assert.Equal(t, 0, pool.evidenceList.Len())

	// no evidence should be pending
	proposedEvidence = pool.PendingEvidence(1)
	assert.Empty(t, proposedEvidence)
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
			return &types.BlockMeta{Header: types.Header{Time: defaultEvidenceTime}}
		}
		return &types.BlockMeta{Header: types.Header{Time: expiredEvidenceTime}}
	})

	pool, err := NewPool(evidenceDB, stateStore, blockStore)
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

func TestEvidencePoolUpdate(t *testing.T) {
	height := int64(21)

	pool, val := defaultTestPool(height)

	state := pool.State()

	// create new block (no need to save it to blockStore)
	evidence := types.NewMockDuplicateVoteEvidence(height, time.Now(), evidenceChainID)
	lastCommit := makeCommit(height, val.PrivKey.PubKey().Address())
	block := types.MakeBlock(height+1, []types.Tx{}, lastCommit, []types.Evidence{evidence})
	// update state (partially)
	state.LastBlockHeight = height + 1

	pool.Update(block, state)

	// a) Update marks evidence as committed
	assert.True(t, pool.IsCommitted(evidence))
}

func TestAddingAndPruningPOLC(t *testing.T) {
	var (
		val           = types.NewMockPV()
		expiredHeight = int64(1)
		firstBlockID  = types.BlockID{
			Hash: tmrand.Bytes(tmhash.Size),
			PartSetHeader: types.PartSetHeader{
				Total: 1,
				Hash:  tmrand.Bytes(tmhash.Size),
			},
		}
		stateStore          = initializeValidatorState(val, expiredHeight)
		blockStore          = &mocks.BlockStore{}
		expiredEvidenceTime = time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	)

	pool, err := NewPool(dbm.NewMemDB(), stateStore, blockStore)
	require.NoError(t, err)
	pool.SetLogger(log.TestingLogger())
	state := pool.State()
	height := state.ConsensusParams.Evidence.MaxAgeNumBlocks * 2

	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
		&types.BlockMeta{Header: types.Header{Time: expiredEvidenceTime}},
	)

	voteA := makeVote(1, 1, 0, val.PrivKey.PubKey().Address(), firstBlockID, expiredEvidenceTime)
	vA := voteA.ToProto()
	err = val.SignVote(evidenceChainID, vA)
	require.NoError(t, err)
	voteA.Signature = vA.Signature

	pubKey, _ := types.NewMockPV().GetPubKey()
	polc := &types.ProofOfLockChange{
		Votes:  []*types.Vote{voteA},
		PubKey: pubKey,
	}

	err = pool.AddPOLC(polc)
	assert.NoError(t, err)

	// should be able to retrieve polc
	newPolc, err := pool.RetrievePOLC(1, 1)
	assert.NoError(t, err)
	assert.True(t, polc.Equal(newPolc))

	// should not be able to retrieve because it doesn't exist
	emptyPolc, err := pool.RetrievePOLC(2, 1)
	assert.NoError(t, err)
	assert.Nil(t, emptyPolc)

	lastCommit := makeCommit(height-1, val.PrivKey.PubKey().Address())
	block := types.MakeBlock(height, []types.Tx{}, lastCommit, []types.Evidence{})
	// update state (partially)
	state.LastBlockHeight = height
	pool.state.LastBlockHeight = height

	// update should prune the polc
	pool.Update(block, state)

	emptyPolc, err = pool.RetrievePOLC(1, 1)
	assert.NoError(t, err)
	assert.Nil(t, emptyPolc)

}

func TestVerifyEvidenceCommittedEvidenceFails(t *testing.T) {
	height := int64(1)
	pool, _ := defaultTestPool(height)
	committedEvidence := types.NewMockDuplicateVoteEvidence(height, time.Now(), evidenceChainID)
	pool.MarkEvidenceAsCommitted(height, []types.Evidence{committedEvidence})

	err := pool.Verify(committedEvidence)
	if assert.Error(t, err) {
		assert.Equal(t, "evidence was already committed", err.Error())
	}
}

func TestVeriyEvidencePendingEvidencePasses(t *testing.T) {
	var (
		val        = types.NewMockPV()
		height     = int64(1)
		stateStore = initializeValidatorState(val, height)
		blockStore = &mocks.BlockStore{}
	)

	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
		&types.BlockMeta{Header: types.Header{Time: defaultEvidenceTime}},
	)

	pool, err := NewPool(dbm.NewMemDB(), stateStore, blockStore)
	require.NoError(t, err)
	evidence := types.NewMockDuplicateVoteEvidenceWithValidator(height, defaultEvidenceTime, val, evidenceChainID)
	err = pool.AddEvidence(evidence)
	require.NoError(t, err)

	err = pool.Verify(evidence)
	assert.NoError(t, err)
}

func TestRecoverPendingEvidence(t *testing.T) {
	var (
		val                 = types.NewMockPV()
		valAddr             = val.PrivKey.PubKey().Address()
		height              = int64(30)
		stateStore          = initializeValidatorState(val, height)
		evidenceDB          = dbm.NewMemDB()
		blockStoreDB        = dbm.NewMemDB()
		state               = stateStore.LoadState()
		blockStore          = initializeBlockStore(blockStoreDB, state, valAddr)
		expiredEvidenceTime = time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
		goodEvidence        = types.NewMockDuplicateVoteEvidenceWithValidator(height,
			defaultEvidenceTime, val, evidenceChainID)
		expiredEvidence = types.NewMockDuplicateVoteEvidenceWithValidator(int64(1),
			expiredEvidenceTime, val, evidenceChainID)
	)

	// load good evidence
	goodKey := keyPending(goodEvidence)
	evi, err := types.EvidenceToProto(goodEvidence)
	require.NoError(t, err)
	goodEvidenceBytes, err := proto.Marshal(evi)
	require.NoError(t, err)
	_ = evidenceDB.Set(goodKey, goodEvidenceBytes)

	// load expired evidence
	expiredKey := keyPending(expiredEvidence)
	eevi, err := types.EvidenceToProto(expiredEvidence)
	require.NoError(t, err)

	expiredEvidenceBytes, err := proto.Marshal(eevi)
	require.NoError(t, err)

	_ = evidenceDB.Set(expiredKey, expiredEvidenceBytes)
	pool, err := NewPool(evidenceDB, stateStore, blockStore)
	require.NoError(t, err)
	assert.Equal(t, 1, pool.evidenceList.Len())
	assert.True(t, pool.IsPending(goodEvidence))
	assert.False(t, pool.Has(expiredEvidence))
}

// Comprehensive set of test cases relating to the adding, upgrading and overall
// processing of PotentialAmnesiaEvidence and AmnesiaEvidence
func TestAmnesiaEvidence(t *testing.T) {
	var (
		val     = types.NewMockPV()
		val2    = types.NewMockPV()
		pubKey  = val.PrivKey.PubKey()
		pubKey2 = val2.PrivKey.PubKey()
		valSet  = &types.ValidatorSet{
			Validators: []*types.Validator{
				val.ExtractIntoValidator(1),
				val2.ExtractIntoValidator(3),
			},
			Proposer: val.ExtractIntoValidator(1),
		}
		height     = int64(30)
		stateStore = initializeStateFromValidatorSet(valSet, height)
		evidenceDB = dbm.NewMemDB()
		state      = stateStore.LoadState()
		blockStore = &mocks.BlockStore{}
		//evidenceTime    = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
		firstBlockID = types.BlockID{
			Hash: tmrand.Bytes(tmhash.Size),
			PartSetHeader: types.PartSetHeader{
				Total: 1,
				Hash:  tmrand.Bytes(tmhash.Size),
			},
		}
		secondBlockID = types.BlockID{
			Hash: tmrand.Bytes(tmhash.Size),
			PartSetHeader: types.PartSetHeader{
				Total: 1,
				Hash:  tmrand.Bytes(tmhash.Size),
			},
		}
		evidenceTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	)

	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
		&types.BlockMeta{Header: types.Header{Time: evidenceTime}},
	)

	// TEST SETUP
	pool, err := NewPool(evidenceDB, stateStore, blockStore)
	require.NoError(t, err)

	pool.SetLogger(log.TestingLogger())

	voteA := makeVote(height, 0, 0, pubKey.Address(), firstBlockID, evidenceTime)
	vA := voteA.ToProto()
	err = val.SignVote(evidenceChainID, vA)
	voteA.Signature = vA.Signature
	require.NoError(t, err)
	voteB := makeVote(height, 1, 0, pubKey.Address(), secondBlockID, evidenceTime.Add(3*time.Second))
	vB := voteB.ToProto()
	err = val.SignVote(evidenceChainID, vB)
	voteB.Signature = vB.Signature
	require.NoError(t, err)
	voteC := makeVote(height, 2, 0, pubKey.Address(), firstBlockID, evidenceTime.Add(2*time.Second))
	vC := voteC.ToProto()
	err = val.SignVote(evidenceChainID, vC)
	voteC.Signature = vC.Signature
	require.NoError(t, err)
	ev := &types.PotentialAmnesiaEvidence{
		VoteA:     voteA,
		VoteB:     voteB,
		Timestamp: evidenceTime,
	}

	polc := &types.ProofOfLockChange{
		Votes:  []*types.Vote{voteB},
		PubKey: pubKey2,
	}
	err = pool.AddPOLC(polc)
	require.NoError(t, err)

	polc, err = pool.RetrievePOLC(height, 1)
	require.NoError(t, err)
	require.NotEmpty(t, polc)

	secondValVote := makeVote(height, 1, 0, pubKey2.Address(), secondBlockID, evidenceTime.Add(1*time.Second))
	vv2 := secondValVote.ToProto()
	err = val2.SignVote(evidenceChainID, vv2)
	require.NoError(t, err)
	secondValVote.Signature = vv2.Signature

	validPolc := &types.ProofOfLockChange{
		Votes:  []*types.Vote{secondValVote},
		PubKey: pubKey,
	}

	// CASE A
	pool.logger.Info("CASE A")
	// we expect the evidence pool to find the polc but log an error as the polc is not valid -> vote was
	// not from a validator in this set. However, an error isn't thrown because the evidence pool
	// should still be able to save the regular potential amnesia evidence.
	err = pool.AddEvidence(ev)
	assert.NoError(t, err)

	// evidence requires trial period until it is available -> we expect no evidence to be returned
	assert.Equal(t, 0, len(pool.PendingEvidence(1)))
	assert.True(t, pool.IsOnTrial(ev))

	nextHeight := pool.nextEvidenceTrialEndedHeight
	assert.Greater(t, nextHeight, int64(0))

	// CASE B
	pool.logger.Info("CASE B")
	// evidence is not ready to be upgraded so we return the height we expect the evidence to be.
	nextHeight = pool.upgradePotentialAmnesiaEvidence()
	assert.Equal(t, height+pool.state.ConsensusParams.Evidence.ProofTrialPeriod, nextHeight)

	// CASE C
	pool.logger.Info("CASE C")
	// now evidence is ready to be upgraded to amnesia evidence -> we expect -1 to be the next height as their is
	// no more pending potential amnesia evidence left
	lastCommit := makeCommit(height+1, pubKey.Address())
	block := types.MakeBlock(height+2, []types.Tx{}, lastCommit, []types.Evidence{})
	state.LastBlockHeight = height + 2

	pool.Update(block, state)
	assert.Equal(t, int64(-1), pool.nextEvidenceTrialEndedHeight)

	assert.Equal(t, 1, len(pool.PendingEvidence(1)))

	// CASE D
	pool.logger.Info("CASE D")
	// evidence of voting back in the past which is instantly punishable -> amnesia evidence is made directly
	ev2 := &types.PotentialAmnesiaEvidence{
		VoteA:     voteC,
		VoteB:     voteB,
		Timestamp: evidenceTime,
	}
	err = pool.AddEvidence(ev2)
	assert.NoError(t, err)
	expectedAe := &types.AmnesiaEvidence{
		PotentialAmnesiaEvidence: ev2,
		Polc:                     types.NewEmptyPOLC(),
	}

	assert.True(t, pool.IsPending(expectedAe))
	assert.Equal(t, 2, len(pool.AllPendingEvidence()))

	// CASE E
	pool.logger.Info("CASE E")
	// test for receiving amnesia evidence
	ae := types.NewAmnesiaEvidence(ev, types.NewEmptyPOLC())
	// we need to run the trial period ourselves so amnesia evidence should not be added, instead
	// we should extract out the potential amnesia evidence and trying to add that before realising
	// that we already have it -> no error
	err = pool.AddEvidence(ae)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(pool.AllPendingEvidence()))

	voteD := makeVote(height, 2, 0, pubKey.Address(), firstBlockID, evidenceTime.Add(4*time.Second))
	vD := voteD.ToProto()
	err = val.SignVote(evidenceChainID, vD)
	require.NoError(t, err)
	voteD.Signature = vD.Signature

	// CASE F
	pool.logger.Info("CASE F")
	// a new amnesia evidence is seen. It has an empty polc so we should extract the potential amnesia evidence
	// and start our own trial
	newPe := &types.PotentialAmnesiaEvidence{
		VoteA:     voteB,
		VoteB:     voteD,
		Timestamp: evidenceTime,
	}
	newAe := &types.AmnesiaEvidence{
		PotentialAmnesiaEvidence: newPe,
		Polc:                     types.NewEmptyPOLC(),
	}
	err = pool.AddEvidence(newAe)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(pool.AllPendingEvidence()))
	assert.True(t, pool.IsOnTrial(newPe))

	// CASE G
	pool.logger.Info("CASE G")
	// Finally, we receive an amnesia evidence containing a valid polc for an earlier potential amnesia evidence
	// that we have already upgraded to. We should ad this new amnesia evidence in replace of the prior
	// amnesia evidence with an empty polc that we have
	aeWithPolc := &types.AmnesiaEvidence{
		PotentialAmnesiaEvidence: ev,
		Polc:                     validPolc,
	}
	err = pool.AddEvidence(aeWithPolc)
	assert.NoError(t, err)
	assert.True(t, pool.IsPending(aeWithPolc))
	assert.Equal(t, 2, len(pool.AllPendingEvidence()))
	t.Log(pool.AllPendingEvidence())

}

func initializeStateFromValidatorSet(valSet *types.ValidatorSet, height int64) StateStore {
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
				MaxAgeNumBlocks:  20,
				MaxAgeDuration:   48 * time.Hour,
				MaxNum:           50,
				ProofTrialPeriod: 1,
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

func initializeValidatorState(privVal types.PrivValidator, height int64) StateStore {

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

func makeVote(height int64, round, index int32, addr bytes.HexBytes,
	blockID types.BlockID, time time.Time) *types.Vote {
	return &types.Vote{
		Type:             tmproto.SignedMsgType(2),
		Height:           height,
		Round:            round,
		BlockID:          blockID,
		Timestamp:        time,
		ValidatorAddress: addr,
		ValidatorIndex:   index,
	}
}

func defaultTestPool(height int64) (*Pool, types.MockPV) {
	val := types.NewMockPV()
	valAddress := val.PrivKey.PubKey().Address()
	evidenceDB := dbm.NewMemDB()
	stateStore := initializeValidatorState(val, height)
	blockStore := initializeBlockStore(dbm.NewMemDB(), stateStore.LoadState(), valAddress)
	pool, err := NewPool(evidenceDB, stateStore, blockStore)
	if err != nil {
		panic("test evidence pool could not be created")
	}
	return pool, val
}
